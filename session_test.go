package main

import (
	"net/http"
	"testing"
	"time"
)

func TestCalcHash(t *testing.T) {
	input := "test"
	salt := []byte{144, 134, 195, 91}
	hash := calcHash(input, salt)
	if hash != "b0e292b2e7822a4cde578f5b10456dab1420820eb74f62e230e30b03f9fd6db1" {
		t.Fatalf("Hash calculation does not match expected hash %v != %v", hash, "b0e292b2e7822a4cde578f5b10456dab1420820eb74f62e230e30b03f9fd6db1")
	}
}

func TestCalcHashEmpty(t *testing.T) {
	input := ""
	salt := []byte{144, 134, 195, 91}
	hash := calcHash(input, salt)
	if hash == "" {
		t.Fatal("Hash calculation does not work correctly when supplying empty input")
	}
}

func TestSessionExpiry(t *testing.T) {
	expiredTime := time.Now().Add(-time.Duration(61) * time.Minute)
	sessions = map[string]time.Time{"b0e292b2e7822a4cde578f5b10456dab1420820eb74f62e230e30b03f9fd6db1": expiredTime}
	go expireSessions(1)
	time.Sleep(time.Duration(2) * time.Second)
	if len(sessions) > 0 {
		t.Fatal("Sessions are not being cleared correctly on expiry")
	}
}

func TestSessionCheck(t *testing.T) {
	sessions = map[string]time.Time{"b0e292b2e7822a4cde578f5b10456dab1420820eb74f62e230e30b03f9fd6db1": time.Now()}
	r := &http.Request{
		Header: map[string][]string{
			"Cookie": {"microblog_h=b0e292b2e7822a4cde578f5b10456dab1420820eb74f62e230e30b03f9fd6db1"},
		},
	}

	ok, err := checkSession(r)
	if err != nil || ok == false {
		t.Fatal("Valid session throws an error")
	}

	r = &http.Request{
		Header: map[string][]string{
			"Cookie": {"microblog_h="},
		},
	}

	ok, err = checkSession(r)
	if err == nil || ok == true {
		t.Fatal("Invalid session doesn't throw an error")
	}

	r = &http.Request{
		Header: map[string][]string{
			"Cookie": {},
		},
	}

	ok, err = checkSession(r)
	if err == nil || ok == true {
		t.Fatal("Invalid session doesn't throw an error")
	}

	r = &http.Request{
		Header: map[string][]string{
			"Cookie": {"microblog_h=test"},
		},
	}

	ok, err = checkSession(r)
	if err == nil || ok == true {
		t.Fatal("Invalid hash doesn't throw an error")
	}
}
