package main

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
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

func readPostHeader(filename string, postdir string) (PostHeader, error) {
	filebytes, err := os.ReadFile(filepath.Join(postdir, filename))
	if err != nil {
		return PostHeader{}, err
	}

	postheader, err := parsePostHeader(filebytes, filename)
	if err != nil {
		return PostHeader{}, err
	}

	return postheader, nil
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
	return PostData{PostHeader: header, Text: markdown.String()}, nil
}

func readPost(filename string, postdir string) (PostData, error) {
	filebytes, err := os.ReadFile(filepath.Join(postdir, filename))
	if err != nil {
		return PostData{}, err
	}

	post, err := parsePost(filebytes, filename)
	if err != nil {
		return PostData{}, err
	}

	return post, nil
}

func buildPost(data CreatePostData) ([]byte, error) {
	empty := CreatePostData{}
	if data == empty || data.Title == "" {
		return nil, errors.New("Post can't be empty")
	}

	var stringbuilder strings.Builder
	stringbuilder.WriteString("### " + data.Title + "\n")
	stringbuilder.WriteString("###### " + time.Now().Format(time.RFC1123) + "\n")
	stringbuilder.WriteString("---\n")
	stringbuilder.WriteString(data.Text)

	return []byte(stringbuilder.String()), nil
}

func writePost(data CreatePostData, postdir string) (string, error) {
	post, err := buildPost(data)
	if err != nil {
		return "", err
	}

	filename := filepath.Join(postdir, generatePostFilename(data.Title))
	deletePost(generatePostFilename(data.Title), postdir)

	err = os.WriteFile(filename, post, 0700)
	if err != nil {
		return "", err
	}

	return filename, nil
}

func deletePost(postname string, postdir string) error {
	filename := filepath.Join(postdir, postname)

	// no error handling here since we don't know if an error occured due to the file not being present, or an os level error
	os.Remove(filename + ".old")
	os.Rename(filename, filename+".old")

	return nil
}

func generatePostFilename(title string) string {
	return url.PathEscape(strings.ToLower(title)) + ".md"
}
