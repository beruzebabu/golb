package main

import (
	"testing"
)

func TestParsePostHeader(t *testing.T) {
	filebytes := []byte{}
	_, err := parsePostHeader(filebytes, "test")
	if err == nil {
		t.Fatal("Parsing empty post should fail")
	}

	filebytes = []byte(`### hello
###### Wed, 05 Feb 2025 17:54:14 CET
---
Hello, world!`)
	postheader, err := parsePostHeader(filebytes, "test")

	if err != nil || postheader.Title != "hello" || postheader.Timestamp != "Wed, 05 Feb 2025 17:54:14 CET" || postheader.URL != "test" || postheader.ContentIndex != 3 {
		t.Fatal("Parsing valid post should succeed")
	}

	filebytes = []byte(`### hello
---
Hello, world!`)
	postheader, err = parsePostHeader(filebytes, "test")

	if err != nil || postheader.Title != "hello" || postheader.Timestamp != "" || postheader.URL != "test" || postheader.ContentIndex != 2 {
		t.Fatal("Parsing valid post should succeed")
	}
}

func TestParsePost(t *testing.T) {
	filebytes := []byte{}
	_, err := parsePost(filebytes, "test")
	if err == nil {
		t.Fatal("Parsing empty post should fail")
	}

	filebytes = []byte(`### hello

Hello, world!`)

	post, err := parsePost(filebytes, "test")

	if err == nil {
		t.Fatal("Parsing invalid post should fail")
	}

	filebytes = []byte(`### hello
###### Wed, 05 Feb 2025 17:54:14 CET
---
Hello, world!`)

	filehtml := `<h3>hello</h3>
<h6>Wed, 05 Feb 2025 17:54:14 CET</h6>
<hr>
<p>Hello, world!</p>
` // newline is required here

	post, err = parsePost(filebytes, "test")

	if err != nil || post.Title != "hello" || post.Timestamp != "Wed, 05 Feb 2025 17:54:14 CET" || post.URL != "test" || post.Text != filehtml {
		t.Fatal("Parsing valid post should succeed")
	}
}

func TestBuildPost(t *testing.T) {
	cpostdata := CreatePostData{}
	_, err := buildPost(cpostdata)
	if err == nil {
		t.Fatal("Building post from empty post data should fail")
	}

	cpostdata = CreatePostData{Title: "Test", Text: "Also a test", Publish: false}
	_, err = buildPost(cpostdata)
	if err != nil {
		t.Fatal("Building post from valid post data should succeed")
	}
}
