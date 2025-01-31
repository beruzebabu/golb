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
    Page string
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
        renderPage(w, "index.html", "Post not found!")
        return
    }

    md, err := os.ReadFile("posts/" + postId)
    if err != nil {
        renderPage(w, "index.html", "Post not found!")
        return
    }

    renderPage(w, "index.html", string(md))
}

func createPostHandler(w http.ResponseWriter, r *http.Request) {
    // Implement logic to create new posts
}

func renderPage(w http.ResponseWriter, tmpl string, data any) {
    var buf *bytes.Buffer = bytes.NewBuffer([]byte{})
    templates.ExecuteTemplate(buf, tmpl, data)
    s := string(buf.Bytes())
    templatedata := TemplateData{Title: TITLE, Page: s}

    templates.ExecuteTemplate(w, "_base.html", templatedata)
}