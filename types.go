package main

import "html/template"

type BlogConfiguration struct {
	Title string
	Hash  string
	Salt  [4]byte
}

type TemplateData struct {
	Title string
	Page  template.HTML
}

type CreatePostData struct {
	Title       string
	Text        string
	Publish     bool
	HTMLMessage template.HTML
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
