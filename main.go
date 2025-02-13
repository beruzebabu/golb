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

var blogConfig BlogConfiguration = BlogConfiguration{Title: TITLE, Port: 8080}

func parseFlags() BlogConfiguration {
	title := flag.String("title", TITLE, "specifies the blog title")
	password := flag.String("password", "", "specifies the management password")
	port := flag.Int("port", 8080, "specifies the port to use, default is 8080")
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

	return BlogConfiguration{Title: *title, Hash: hashed, Salt: [4]byte(randbytes), Port: *port}
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
	go refreshPosts(30)
	go expireSessions(60)
	hostname := fmt.Sprintf(":%v", blogConfig.Port)
	fmt.Println("Server running on ", hostname)
	log.Fatal(http.ListenAndServe(hostname, nil))
}

func updatePostsList() (map[string]bool, error) {
	var availablePosts map[string]bool = map[string]bool{}
	postlist, err := filepath.Glob("posts/*.md")
	if err != nil {
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
		availablePosts, err := updatePostsList()
		if err != nil {
			log.Println(err)
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
		log.Println(err, " ", r.RemoteAddr)
	}

	if !ok {
		renderPage(w, "login.html", nil)
		return
	}

	form := CreatePostData{}
	if r.Method == "GET" {
		renderPage(w, "create.html", form)
		return
	} else if r.Method == "POST" {
		err := r.ParseForm()
		if err != nil {
			log.Println(err)
			form.HTMLMessage = "Failed to parse data!"
			renderPage(w, "create.html", form)
			return
		}
		publish := r.PostFormValue("publish") != ""
		form := CreatePostData{Title: r.PostFormValue("title"), Text: r.PostFormValue("data"), Publish: publish}
		if publish {
			filename, err := writePost(form)
			if err != nil {
				log.Println(err)
				form.HTMLMessage = "Failed publish post!"
				renderPage(w, "create.html", form)
				return
			}
			form.HTMLMessage = template.HTML("Published to file " + filename)
			availablePosts, err := updatePostsList()
			if err != nil {
				log.Println(err)
				form.HTMLMessage = "Failed to update list of published posts!"
				renderPage(w, "create.html", form)
				return
			}
			postsMutex.Lock()
			posts = availablePosts
			postsMutex.Unlock()
		} else {
			post, err := buildPost(form)
			if err != nil {
				log.Println(err)
				form.HTMLMessage = "Failed to generate preview!"
				renderPage(w, "create.html", form)
				return
			}
			postdata, err := parsePost(post, "")
			if err != nil {
				log.Println(err)
				form.HTMLMessage = "Failed to generate preview!"
				renderPage(w, "create.html", form)
				return
			}
			form.HTMLMessage = postdata.Text
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
			log.Println("login failed for ", r.RemoteAddr, " due to invalid form data")
			renderPage(w, "login.html", "Login failed!")
			return
		}
		password := r.PostFormValue("password")
		res := calcHash(password, blogConfig.Salt[:])
		if res != blogConfig.Hash {
			log.Println("login failed for ", r.RemoteAddr, " due to invalid password")
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
		log.Println("login succeeded for ", r.RemoteAddr)
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
