package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/yuin/goldmark"
)

type BlogConfiguration struct {
	Title string
}

type TemplateData struct {
	Title string
	Page  template.HTML
}

type CreatePostData struct {
	Title   string
	Text    string
	Publish bool
}

type PostHeader struct {
	Title     string
	Timestamp string
	URL       string
}

type PostData struct {
	PostHeader
	Text template.HTML
}

const TITLE string = "Microblog"

var templates *template.Template = template.Must(template.ParseGlob("templates/*.html"))
var posts map[string]bool = map[string]bool{}
var mutex sync.Mutex

var blogConfig BlogConfiguration = BlogConfiguration{Title: TITLE}

func parseFlags() BlogConfiguration {
	title := flag.String("title", TITLE, "specifies the blog title")
	flag.Parse()

	return BlogConfiguration{Title: *title}
}

func main() {
	blogConfig = parseFlags()

	availablePosts, err := updatePostsList()
	if err != nil {
		log.Fatal(err)
	}

	mutex.Lock()
	posts = availablePosts
	mutex.Unlock()

	http.Handle("/files/", http.FileServer(http.Dir("")))
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/posts", homeHandler)
	http.HandleFunc("/posts/{postId}", postsHandler)
	http.HandleFunc("/create", createPostHandler)
	fmt.Println("Server running on http://localhost:8080")
	go refreshPosts(30)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func updatePostsList() (map[string]bool, error) {
	var availablePosts map[string]bool = map[string]bool{}
	postlist, err := filepath.Glob("posts/*.md")
	if err != nil {
		log.Println(err)
		return map[string]bool{}, err
	}

	for _, post := range postlist {
		availablePosts[filepath.Base(post)] = true
	}
	return availablePosts, nil
}

func refreshPosts(sleepseconds int) {
	for {
		time.Sleep(time.Duration(sleepseconds) * time.Second)
		var availablePosts map[string]bool = map[string]bool{}
		postlist, err := filepath.Glob("posts/*.md")
		if err != nil {
			log.Println(err)
		}

		for _, post := range postlist {
			availablePosts[filepath.Base(post)] = true
		}
		mutex.Lock()
		posts = availablePosts
		mutex.Unlock()
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	dfs := os.DirFS("posts")
	postlist, err := fs.ReadDir(dfs, ".")
	if err != nil {
		log.Println(err)
		renderPage(w, "error.html", "Something went wrong, please check back later!")
		return
	}

	var postsheaders []PostHeader
	for _, post := range postlist {
		file, err := dfs.Open(post.Name())
		defer file.Close()
		if err != nil {
			log.Println(err, post.Name())
			renderPage(w, "error.html", "Something went wrong, please check back later!")
			return
		}

		filebytes, err := io.ReadAll(file)
		if err != nil {
			log.Println(err, post.Name())
			renderPage(w, "error.html", "Something went wrong, please check back later!")
			return
		}

		postheader, err := parsePostHeader(filebytes, post.Name())
		if err != nil {
			log.Println(err, post.Name())
			renderPage(w, "error.html", "Something went wrong, please check back later!")
			return
		}
		postsheaders = append(postsheaders, postheader)
	}

	slices.SortStableFunc(postsheaders, func(a PostHeader, b PostHeader) int {
		atime, err := time.Parse(time.RFC1123, a.Timestamp)
		if err != nil {
			return -1
		}
		btime, err := time.Parse(time.RFC1123, b.Timestamp)
		if err != nil {
			return 1
		}

		return btime.Compare(atime)
	})

	renderPage(w, "index.html", postsheaders)
}

func postsHandler(w http.ResponseWriter, r *http.Request) {
	postId := r.PathValue("postId")
	postId = fmt.Sprintf("%v.md", postId)

	if !posts[postId] {
		renderPage(w, "error.html", "Post not found!")
		return
	}

	md, err := os.ReadFile("posts/" + postId)
	if err != nil {
		log.Println(err, postId)
		renderPage(w, "error.html", "Post not found!")
		return
	}

	postdata, err := parsePost(md, postId)
	if err != nil {
		log.Println(err, postId)
		renderPage(w, "error.html", "Something went wrong, please check back later!")
		return
	}

	renderPage(w, "post.html", postdata)
}

func createPostHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		form := CreatePostData{}
		renderPage(w, "create.html", form)
		return
	} else if r.Method == "POST" {
		err := r.ParseForm()
		if err != nil {
			log.Println(err)
			renderPage(w, "create.html", "Failed to parse data!")
			return
		}
		publish := r.PostFormValue("publish") != ""
		form := CreatePostData{Title: r.PostFormValue("title"), Text: r.PostFormValue("data"), Publish: publish}
		if publish {
			err = writePost(form)
			if err != nil {
				log.Println(err)
				renderPage(w, "create.html", "Failed publish post!")
				return
			}
			availablePosts, err := updatePostsList()
			if err != nil {
				log.Println(err)
				renderPage(w, "create.html", "Failed to update list of published posts!")
				return
			}
			mutex.Lock()
			posts = availablePosts
			mutex.Unlock()
		}
		renderPage(w, "create.html", form)
		return
	}
	renderPage(w, "error.html", "Page not found!")
	return
}

func renderPage(w http.ResponseWriter, tmpl string, data any) {
	var buf *bytes.Buffer = bytes.NewBuffer([]byte{})
	templates.ExecuteTemplate(buf, tmpl, data)
	s := template.HTML(buf.Bytes())
	templatedata := TemplateData{Title: blogConfig.Title, Page: s}

	templates.ExecuteTemplate(w, "_base.html", templatedata)
}

func parsePostHeader(filebytes []byte, postId string) (PostHeader, error) {
	rstring := strings.ReplaceAll(string(filebytes), "\r", "")
	splitstrings := strings.Split(rstring, "\n")
	index := slices.Index(splitstrings, "---")
	if index == -1 || len(splitstrings) < 2 || index >= len(splitstrings) {
		return PostHeader{}, errors.New("Invalid post format, cannot parse post")
	}
	var timestamp string
	if index >= 2 {
		timestamp = strings.TrimPrefix(splitstrings[1], "###### ")
	}
	return PostHeader{Title: strings.TrimPrefix(splitstrings[0], "### "), Timestamp: timestamp, URL: strings.TrimSuffix(postId, ".md")}, nil
}

func parsePost(filebytes []byte, postId string) (PostData, error) {
	rstring := strings.ReplaceAll(string(filebytes), "\r", "")
	splitstrings := strings.Split(rstring, "\n")
	index := slices.Index(splitstrings, "---")
	if index == -1 || len(splitstrings) < 2 || index >= len(splitstrings) {
		return PostData{}, errors.New("Invalid post format, cannot parse post")
	}
	var timestamp string
	if index >= 2 {
		timestamp = strings.TrimPrefix(splitstrings[1], "###### ")
	}
	var markdown strings.Builder
	err := goldmark.Convert([]byte(strings.Join(splitstrings, "\n")), &markdown)
	if err != nil {
		return PostData{}, err
	}
	return PostData{PostHeader: PostHeader{Title: strings.TrimPrefix(splitstrings[0], "### "), Timestamp: timestamp, URL: strings.TrimSuffix(postId, ".md")}, Text: template.HTML(markdown.String())}, nil
}

func writePost(data CreatePostData) error {
	var stringbuilder strings.Builder
	stringbuilder.WriteString("### " + data.Title + "\n")
	stringbuilder.WriteString("###### " + time.Now().Format(time.RFC1123) + "\n")
	stringbuilder.WriteString("---\n")
	stringbuilder.WriteString(data.Text)

	filename := url.QueryEscape(strings.ToLower(data.Title)) + ".md"

	err := os.WriteFile("posts/"+filename, []byte(stringbuilder.String()), 0700)
	if err != nil {
		return err
	}

	return nil
}
