package main

import (
	"bytes"
	"crypto/rand"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

const TITLE string = "Golb"

var templates *template.Template = template.Must(template.ParseGlob("templates/*.html"))
var posts map[string]bool = map[string]bool{}
var sessions map[string]time.Time = map[string]time.Time{}
var postsMutex sync.Mutex
var sessionsMutex sync.Mutex

var blogConfig BlogConfiguration = BlogConfiguration{Title: TITLE}

func parseFlags() BlogConfiguration {
	title := flag.String("title", TITLE, "specifies the blog title")
	password := flag.String("password", "", "specifies the management password")
	flag.Parse()

	if *password == "" {
		log.Fatal("management password is required")
	}

	randbytes := make([]byte, 4)
	_, err := rand.Read(randbytes)
	if err != nil {
		log.Fatal("couldn't read cryptographically secure rand")
	}
	hashed := calcHash(*password, randbytes)

	return BlogConfiguration{Title: *title, Hash: hashed, Salt: [4]byte(randbytes)}
}

func main() {
	blogConfig = parseFlags()

	availablePosts, err := updatePostsList()
	if err != nil {
		log.Fatal(err)
	}

	postsMutex.Lock()
	posts = availablePosts
	postsMutex.Unlock()

	http.Handle("/files/", http.FileServer(http.Dir("")))
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/posts", homeHandler)
	http.HandleFunc("/posts/{postId}", postsHandler)
	http.HandleFunc("/create", createPostHandler)
	http.HandleFunc("/login", loginHandler)
	fmt.Println("Server running on http://localhost:8080")
	go refreshPosts(30)
	go expireSessions(60)
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
		postsMutex.Lock()
		posts = availablePosts
		postsMutex.Unlock()
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
	ok, err := checkSession(r)
	if err != nil {
		log.Println(err)
	}

	if !ok {
		renderPage(w, "login.html", nil)
		return
	}

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
			postsMutex.Lock()
			posts = availablePosts
			postsMutex.Unlock()
		}
		renderPage(w, "create.html", form)
		return
	}
	renderPage(w, "error.html", "Page not found!")
	return
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		err := r.ParseForm()
		if err != nil {
			renderPage(w, "login.html", "Login failed!")
			return
		}
		password := r.PostFormValue("password")
		res := calcHash(password, blogConfig.Salt[:])
		if res != blogConfig.Hash {
			renderPage(w, "login.html", "Login failed!")
			return
		}
		randbytes := make([]byte, 4)
		_, err = rand.Read(randbytes)
		if err != nil {
			log.Println("couldn't read cryptographically secure rand, session aborted")
			renderPage(w, "login.html", "Login failed!")
			return
		}
		session := calcHash(blogConfig.Hash, randbytes)
		sessionsMutex.Lock()
		sessions[session] = time.Now()
		sessionsMutex.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "microblog_h", Value: session, Path: "/", Secure: true})
		renderPage(w, "login.html", "Login succeeded!")
		return
	}

	renderPage(w, "login.html", nil)
	return
}

func renderPage(w http.ResponseWriter, tmpl string, data any) {
	var buf *bytes.Buffer = bytes.NewBuffer([]byte{})
	templates.ExecuteTemplate(buf, tmpl, data)
	s := template.HTML(buf.Bytes())
	templatedata := TemplateData{Title: blogConfig.Title, Page: s}

	templates.ExecuteTemplate(w, "_base.html", templatedata)
}
