package main

import (
	"bytes"
	"crypto/rand"
	"flag"
	"fmt"
	"html/template"
	"log"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

const TITLE string = "Golb"

var templates *template.Template = template.Must(template.ParseGlob("templates/*.html"))
var sessions map[string]time.Time = map[string]time.Time{}
var postHeadersCache SyncCache[map[string]PostHeader] = SyncCache[map[string]PostHeader]{}
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

	refreshPosts(0)

	http.Handle("/files/", http.FileServer(http.Dir("")))
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

func generatePostsSet() (map[string]bool, error) {
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

func generatePostHeaderCache() (map[string]PostHeader, error) {
	var postsCache map[string]PostHeader = map[string]PostHeader{}
	postsSet, err := generatePostsSet()
	if err != nil {
		log.Println(err)
		return map[string]PostHeader{}, err
	}
	for name := range postsSet {
		filebytes, err := os.ReadFile("posts/" + name)
		if err != nil {
			log.Println(err, name)
			return map[string]PostHeader{}, err
		}

		postheader, err := parsePostHeader(filebytes, name)
		if err != nil {
			log.Println(err, name)
			return map[string]PostHeader{}, err
		}

		postsCache[name] = postheader
	}
	return postsCache, nil
}

func refreshPosts(sleepseconds int) {
	for {
		availablePosts, err := generatePostHeaderCache()
		if err != nil {
			log.Println(err)
			return
		}
		postHeadersCache.Set(availablePosts)
		if sleepseconds < 1 {
			break
		}
		time.Sleep(time.Duration(sleepseconds) * time.Second)
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	postsheaders := slices.Collect(maps.Values[map[string]PostHeader](postHeadersCache.Get()))
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

	sess, _ := checkSession(r)
	parameters := PageParameters[[]PostHeader]{PageData: postsheaders, HasSession: sess}

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
			filename, err := writePost(form)
			if err != nil {
				log.Println(err)
				form.HTMLMessage = "Failed publish post!"
				renderPage(w, "create.html", form)
				return
			}
			form.HTMLMessage = template.HTML("Published to file " + filename)
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

		md, err := os.ReadFile("posts/" + postId)
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

		err := os.Remove("posts/" + postId)
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
	s := template.HTML(buf.Bytes())
	templatedata := TemplateData{Title: blogConfig.Title, Page: s}

	templates.ExecuteTemplate(w, "_base.html", templatedata)
}
