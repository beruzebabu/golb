package main

import (
	"bytes"
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"
)

const TITLE string = "Golb"

var templates *template.Template = template.Must(template.ParseGlob("templates/*.html"))
var sessions map[string]time.Time = map[string]time.Time{}
var postHeadersCache SyncCache[map[string]PostHeader] = SyncCache[map[string]PostHeader]{}
var sortedPostIndexCache SyncCache[[]PostHeader] = SyncCache[[]PostHeader]{}
var sessionsMutex sync.Mutex

var blogConfig BlogConfiguration = BlogConfiguration{Title: TITLE, Port: 8080}

func parseFlags() BlogConfiguration {
	titleEnv := os.Getenv("GOLB_TITLE")
	passwordEnv := os.Getenv("GOLB_PASSWORD")
	portEnv := os.Getenv("GOLB_PORT")
	postEnv := os.Getenv("GOLB_POSTDIR")

	if titleEnv == "" {
		titleEnv = TITLE
	}

	if portEnv == "" {
		portEnv = "8080"
	}

	if postEnv == "" {
		postEnv = "posts"
	}

	defPort, err := strconv.Atoi(portEnv)
	if err != nil {
		defPort = 8080
	}

	title := flag.String("title", titleEnv, "specifies the blog title")
	password := flag.String("password", passwordEnv, "specifies the management password")
	port := flag.Int("port", defPort, "specifies the port to use, default is 8080")
	postDir := flag.String("postdir", postEnv, "specifies the directory to use for posts")
	flag.Parse()

	if *password == "" {
		log.Fatal("management password is required")
	}

	*postDir = filepath.Clean(*postDir)

	randbytes := make([]byte, 4)
	_, err = rand.Read(randbytes)
	if err != nil {
		log.Fatal("couldn't read cryptographically secure rand")
	}

	hashed, err := calcHash(*password, randbytes)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("parsed flags, title = %v, port = %v, postdir = %v", *title, *port, *postDir)

	return BlogConfiguration{Title: *title, Hash: hashed, Salt: [4]byte(randbytes), Port: *port, PostDir: *postDir}
}

func main() {
	blogConfig = parseFlags()

	refreshPosts(0)

	http.Handle("/files/", http.FileServer(http.Dir("")))
	http.Handle("/favicon.ico", http.RedirectHandler("/files/favicon.ico", 301))
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/posts", homeHandler)
	http.HandleFunc("/posts/{postId}", postsHandler)
	http.HandleFunc("/create", createPostHandler)
	http.HandleFunc("/create/{postId}", editPostHandler)
	http.HandleFunc("/delete/{postId}", deletePostHandler)
	http.HandleFunc("/login", loginHandler)
	go refreshPosts(30)
	go expireSessions(60)
	hostname := fmt.Sprintf(":%v", blogConfig.Port)
	fmt.Println("Server running on ", hostname)
	log.Fatal(http.ListenAndServe(hostname, nil))
}

func generatePostFilenamesList() ([]string, error) {
	postpaths, err := filepath.Glob(filepath.Join(blogConfig.PostDir, "*.md"))
	if err != nil {
		return []string{}, err
	}

	var postlist []string = []string{}
	for _, path := range postpaths {
		postlist = append(postlist, filepath.Base(path))
	}

	return postlist, nil
}

func generatePostHeaderCaches() (map[string]PostHeader, []PostHeader, error) {
	var postsCache map[string]PostHeader = map[string]PostHeader{}
	var postHeaders []PostHeader = []PostHeader{}
	postsList, err := generatePostFilenamesList()
	if err != nil {
		log.Println(err)
		return map[string]PostHeader{}, []PostHeader{}, err
	}
	for _, name := range postsList {
		postheader, err := readPostHeader(name, blogConfig.PostDir)
		if err != nil {
			log.Println(err)
			return map[string]PostHeader{}, []PostHeader{}, err
		}

		postsCache[name] = postheader
		postHeaders = append(postHeaders, postheader)
	}
	return postsCache, postHeaders, nil
}

func refreshPosts(sleepseconds int) {
	for {
		availablePosts, postHeaders, err := generatePostHeaderCaches()
		if err != nil {
			log.Println(err)
			return
		}

		slices.SortStableFunc(postHeaders, func(a PostHeader, b PostHeader) int {
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

		sortedPostIndexCache.Set(postHeaders)
		postHeadersCache.Set(availablePosts)
		if sleepseconds < 1 {
			break
		}
		time.Sleep(time.Duration(sleepseconds) * time.Second)
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	postHeaders := sortedPostIndexCache.Get()
	sess, _ := checkSession(r)
	parameters := PageParameters[[]PostHeader]{PageData: postHeaders, HasSession: sess}

	renderPage(w, "index.html", parameters)
}

func postsHandler(w http.ResponseWriter, r *http.Request) {
	postId := r.PathValue("postId")
	postId = fmt.Sprintf("%v.md", postId)

	posts := postHeadersCache.Get()
	_, ok := posts[postId]

	if !ok {
		renderPage(w, "error.html", "Post not found!")
		return
	}

	postdata, err := readPost(postId, blogConfig.PostDir)
	if err != nil {
		log.Println(err, postId)
		renderPage(w, "error.html", "Something went wrong, please check back later!")
		return
	}

	sess, _ := checkSession(r)
	parameters := PageParameters[PostData]{PageData: postdata, HasSession: sess}

	renderPage(w, "post.html", parameters)
}

func createPostHandler(w http.ResponseWriter, r *http.Request) {
	ok, err := checkSession(r)
	if err != nil {
		log.Println(err, " ", r.RemoteAddr)
	}

	if !ok {
		http.Redirect(w, r, "login", 307)
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
			filename, err := writePost(form, blogConfig.PostDir)
			if err != nil {
				log.Println(err)
				form.HTMLMessage = "Failed publish post!"
				renderPage(w, "create.html", form)
				return
			}
			form.HTMLMessage = "Published to file " + filename
			refreshPosts(0)
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

func editPostHandler(w http.ResponseWriter, r *http.Request) {
	ok, err := checkSession(r)
	if err != nil {
		log.Println(err, " ", r.RemoteAddr)
	}

	if !ok {
		http.Redirect(w, r, "/login", 307)
		return
	}

	if r.Method == "GET" {
		postId := r.PathValue("postId")
		postId = fmt.Sprintf("%v.md", postId)

		md, err := os.ReadFile(filepath.Join(blogConfig.PostDir, postId))
		if err != nil {
			log.Println(err, postId)
			renderPage(w, "error.html", "Post not found!")
			return
		}

		post, err := parsePost(md, strings.TrimSuffix(postId, ".md"))
		if err != nil {
			log.Println(err, postId)
			renderPage(w, "error.html", "Post not found!")
			return
		}
		rstring := strings.ReplaceAll(string(md), "\r", "")
		splitstrings := strings.Split(rstring, "\n")
		form := CreatePostData{Title: post.Title, Text: strings.Join(splitstrings[post.ContentIndex:], "\n"), Publish: true, HTMLMessage: post.Text}
		renderPage(w, "create.html", form)
		return
	}
	renderPage(w, "error.html", "Page not found!")
	return
}

func deletePostHandler(w http.ResponseWriter, r *http.Request) {
	ok, err := checkSession(r)
	if err != nil {
		log.Println(err, " ", r.RemoteAddr)
	}

	if !ok {
		renderPage(w, "error.html", "Page not found!")
		return
	}
	if r.Method == "DELETE" || r.Method == "GET" {
		postId := r.PathValue("postId")
		postId = fmt.Sprintf("%v.md", postId)

		err := os.Remove(filepath.Join(blogConfig.PostDir, postId))
		if err != nil {
			w.WriteHeader(404)
			renderPage(w, "delete.html", "Deleting post failed: "+err.Error())
			return
		}
		w.WriteHeader(200)
		renderPage(w, "delete.html", "Post "+postId+" deleted!")
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
		res, err := calcHash(password, blogConfig.Salt[:])
		if err != nil {
			log.Println(err)
			renderPage(w, "login.html", "Login failed!")
			return
		}
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
		session, err := calcHash(blogConfig.Hash, randbytes)
		if err != nil {
			log.Println(err)
			renderPage(w, "login.html", "Login failed!")
			return
		}
		sessionsMutex.Lock()
		sessions[session] = time.Now()
		sessionsMutex.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "microblog_h", Value: session, Path: "/", Secure: true, MaxAge: 3600})
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
	s := string(buf.Bytes())
	templatedata := TemplateData{Title: blogConfig.Title, Page: s}

	templates.ExecuteTemplate(w, "_base.html", templatedata)
}
