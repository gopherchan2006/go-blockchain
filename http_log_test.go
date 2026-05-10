package main

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenAccessLogCreatesFile(t *testing.T) {
	dir := t.TempDir()
	al, err := openAccessLog(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer al.Close()
	if _, err := os.Stat(filepath.Join(dir, "http-access.log")); err != nil {
		t.Fatal(err)
	}
}

func TestWithHTTPAccessLogRecordsStatus(t *testing.T) {
	var buf bytes.Buffer
	lg := log.New(&buf, "", 0)
	h := withHTTPAccessLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}), lg)
	req := httptest.NewRequest(http.MethodGet, "/x?y=1", nil)
	req.RemoteAddr = "1.2.3.4:5"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTeapot {
		t.Fatal(rec.Code)
	}
	s := buf.String()
	if !strings.Contains(s, `GET "/x?y=1"`) || !strings.Contains(s, "418") {
		t.Fatal(s)
	}
}

func TestWithHTTPAccessLogNilPassthrough(t *testing.T) {
	called := false
	h := withHTTPAccessLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if !called {
		t.Fatal("handler")
	}
}

func TestTruncateRun(t *testing.T) {
	if truncateRun("abc", 10) != "abc" {
		t.Fatal()
	}
	s := truncateRun("abcdefghijklmnop", 10)
	if len(s) != 10 || s[9] != '.' {
		t.Fatal(s)
	}
}
