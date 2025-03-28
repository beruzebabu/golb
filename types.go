package main

type BlogConfiguration struct {
	Title       string
	Hash        string
	Salt        [4]byte
	Port        int
	PostDir     string
	TemplateDir string
	FileDir     string
	ViewOnly    bool
}

func (bc BlogConfiguration) isPasswordless() bool {
	if bc.ViewOnly || bc.Hash == "" {
		return true
	}
	return false
}

type TemplateData struct {
	Title string
	Page  string
}

type CreatePostData struct {
	Title       string
	Text        string
	Publish     bool
	HTMLMessage string
}

type PostHeader struct {
	Title        string
	Timestamp    string
	URL          string
	ContentIndex int
}

type PostData struct {
	PostHeader
	Text string
}

type PageParameters[T any] struct {
	PageData     T
	HasSession   bool
	CurrentPage  int
	NextPage     int
	PreviousPage int
}
