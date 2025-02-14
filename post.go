package main

import (
	"errors"
	"html/template"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/yuin/goldmark"
)

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
	return PostHeader{Title: strings.TrimPrefix(splitstrings[0], "### "), Timestamp: timestamp, URL: strings.TrimSuffix(postId, ".md"), ContentIndex: index + 1}, nil
}

func parsePost(filebytes []byte, postId string) (PostData, error) {
	header, err := parsePostHeader(filebytes, postId)
	if err != nil {
		return PostData{}, err
	}
	var markdown strings.Builder
	err = goldmark.Convert(filebytes, &markdown)
	if err != nil {
		return PostData{}, err
	}
	return PostData{PostHeader: header, Text: template.HTML(markdown.String())}, nil
}

func buildPost(data CreatePostData) ([]byte, error) {
	empty := CreatePostData{}
	if data == empty {
		return nil, errors.New("Post can't be empty")
	}

	var stringbuilder strings.Builder
	stringbuilder.WriteString("### " + data.Title + "\n")
	stringbuilder.WriteString("###### " + time.Now().Format(time.RFC1123) + "\n")
	stringbuilder.WriteString("---\n")
	stringbuilder.WriteString(data.Text)

	return []byte(stringbuilder.String()), nil
}

func writePost(data CreatePostData) (string, error) {
	post, err := buildPost(data)
	if err != nil {
		return "", err
	}

	filename := url.QueryEscape(strings.ToLower(data.Title)) + ".md"

	err = os.WriteFile("posts/"+filename, post, 0700)
	if err != nil {
		return "", err
	}

	return filename, nil
}
