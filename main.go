package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

type TemplateData struct {
    Title string
    Page template.HTML
}

type CreatePostData struct {
    Title string
    Text string
    Publish bool
}

const TITLE string = "Microblog"
var templates *template.Template = template.Must(template.ParseGlob("templates/*.html"))
var posts map[string]bool = map[string]bool{}

func main() {
    postlist, err := filepath.Glob("posts/*.md")
    if err != nil {
        log.Fatal(err)
    }

    for _, post := range postlist {
        posts[filepath.Base(post)] = true
    }

    http.Handle("/files/", http.FileServer(http.Dir("")))
    http.HandleFunc("/", homeHandler)
    http.HandleFunc("/posts", postsHandler)
    http.HandleFunc("/posts/{postId}", postsHandler)
	http.HandleFunc("/create", createPostHandler)
    fmt.Println("Server running on http://localhost:8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
    renderPage(w, "index.html", "Welcome to Microblog!")
    // fmt.Fprintf(w, "Welcome to Microblog!")
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
        renderPage(w, "error.html", "Post not found!")
        return
    }

    renderPage(w, "index.html", template.HTML(md))
}

func createPostHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method == "GET" {
        form := CreatePostData{}
        renderPage(w, "create.html", form)
        return
    } else if r.Method == "POST" {
        err := r.ParseForm()
        if err != nil {
            renderPage(w, "create.html", "Failed to parse data!")
            return
        }
        publish := r.PostFormValue("publish") != ""
        form := CreatePostData{Title: r.PostFormValue("title"), Text: r.PostFormValue("data"), Publish: publish}
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
    templatedata := TemplateData{Title: TITLE, Page: s}

    templates.ExecuteTemplate(w, "_base.html", templatedata)
}