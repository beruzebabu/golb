package main

import (
	"crypto"
	"encoding/hex"
	"errors"
	"net/http"
	"time"
)

func calcHash(text string, seed []byte) (string, error) {
	if len(text) > 4096 {
		return "", errors.New("input text too long")
	}

	h := crypto.SHA256.New()
	h.Write(seed)
	h.Write([]byte(text + TITLE))
	return hex.EncodeToString(h.Sum(nil)), nil
}

func checkSession(r *http.Request) (bool, error) {
	hcookie, err := r.Cookie("microblog_h")
	if err != nil {
		return false, errors.New("couldn't find session cookie")
	}

	cookieid := hcookie.Value

	sessionsMutex.Lock()
	_, ok := sessions[cookieid]
	sessionsMutex.Unlock()

	if !ok {
		return false, errors.New("invalid session")
	}

	return true, nil
}

func expireSessions(sleepseconds int) {
	for {
		time.Sleep(time.Duration(sleepseconds) * time.Second)

		var sessionsToDelete []string

		for sess, t := range sessions {
			if t.Add(time.Duration(3600)*time.Second).Compare(time.Now()) < 0 {
				sessionsToDelete = append(sessionsToDelete, sess)
			}
		}

		sessionsMutex.Lock()
		for _, sess := range sessionsToDelete {
			delete(sessions, sess)
		}
		sessionsMutex.Unlock()
	}
}
