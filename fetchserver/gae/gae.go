// Copyright 2012 Phus Lu. All rights reserved.

package gae

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"appengine"
	"appengine/urlfetch"
)

const (
	Version  = "1.0"
	Password = ""

	FetchMaxSize = 1024 * 1024 * 4
	Deadline     = 30 * time.Second
)

func favicon(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte("User-agent: *\nDisallow: /\n"))
}

func handlerError(w http.ResponseWriter, html string, code int) {
	fmt.Fprintf(w, "HTTP/1.1 %d\r\n", code)
	fmt.Fprintf(w, "Content-Type: text/html; charset=utf-8\r\n")
	fmt.Fprintf(w, "Content-Length: %d\r\n", len(html))
	w.Write([]byte("\r\n"))
	w.Write([]byte(html))
}

func copyResponse(w io.Writer, resp *http.Response) {
	fmt.Fprintf(w, "%s %s\r\n", resp.Proto, resp.Status)
	for key, values := range resp.Header {
		for _, value := range values {
			fmt.Fprintf(w, "%s: %s\r\n", key, value)
		}
	}
	w.Write([]byte("\r\n"))
	io.Copy(w, resp.Body)
}

func handler(w http.ResponseWriter, r *http.Request) {
	var err error
	context := appengine.NewContext(r)
	if "gzip" == r.Header.Get("Content-Encoding") {
		r.Body, err = gzip.NewReader(r.Body)
		if err != nil {
			context.Criticalf("gzip.NewReader(%#v) return %#v", r.Body, err)
		}
	}

	req, err := http.ReadRequest(bufio.NewReader(r.Body))
	if err != nil {
		context.Criticalf("http.ReadRequest(%#v) return %#v", r.Body, err)
	}

	params := make(map[string]string, 2)
	paramPrefix := "X-Fetch-"
	for key, values := range r.Header {
		if strings.HasPrefix(key, paramPrefix) {
			params[strings.ToLower(key[len(paramPrefix):])] = values[0]
		}
	}
	for _, key := range params {
		req.Header.Del(key)
	}
	if Password != "" {
		if password, ok := params["password"]; !ok || password != Password {
			handlerError(w, "Wrong Password.", 403)
		}
	}

	deadline := Deadline

	var errors []string
	for i := 0; i < 2; i++ {
		t := &urlfetch.Transport{Context: context, Deadline: deadline, AllowInvalidServerCertificate: true}
		resp, err := t.RoundTrip(req)
		if err != nil {
			message := err.Error()
			errors = append(errors, message)
			if strings.Contains(message, "FETCH_ERROR") {
				context.Errorf("URLFetchServiceError_FETCH_ERROR(type=%T, deadline=%v, url=%v)", err, deadline, req.URL)
				time.Sleep(time.Second)
				deadline *= 2
			} else if strings.Contains(message, "DEADLINE_EXCEEDED") {
				context.Errorf("URLFetchServiceError_DEADLINE_EXCEEDED(type=%T, deadline=%v, url=%v)", err, deadline, req.URL)
				time.Sleep(time.Second)
				deadline *= 2
			} else if strings.Contains(message, "INVALID_URL") {
				handlerError(w, fmt.Sprintf("Invalid URL: %v", err), 501)
				return
			} else if strings.Contains(message, "RESPONSE_TOO_LARGE") {
				context.Errorf("URLFetchServiceError_RESPONSE_TOO_LARGE(type=%T, deadline=%v, url=%v)", err, deadline, req.URL)
				req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", FetchMaxSize))
				deadline *= 2
			} else {
				context.Errorf("URLFetchServiceError UNKOWN(type=%T, deadline=%v, url=%v, error=%v)", err, deadline, req.URL, err)
				time.Sleep(4 * time.Second)
			}
			continue
		}
		w.Header().Set("Content-Type", "image/gif")
		resp.Header.Set("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
		if resp.TransferEncoding == nil && resp.ContentLength <= 1024*1024 {
			w.Header().Set("X-Content-Encoding", "gzip")
			gw := gzip.NewWriter(w)
			defer gw.Close()
			copyResponse(gw, resp)
		} else {
			copyResponse(w, resp)
		}
		return
	}
	handlerError(w, fmt.Sprintf("Go Server Fetch Failed: %v", errors), 502)
}

func root(w http.ResponseWriter, r *http.Request) {
	context := appengine.NewContext(r)
	version, _ := strconv.ParseInt(strings.Split(appengine.VersionID(context), ".")[1], 10, 64)
	ctime := time.Unix(version/(1<<28)+8*3600, 0).Format(time.RFC3339)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "GoAgent go server %s works, deployed at %s\n", Version, ctime)
}

func init() {
	http.HandleFunc("/favicon.ico", favicon)
	http.HandleFunc("/robots.txt", robots)
	http.HandleFunc("/_gh/", handler)
	http.HandleFunc("/", root)
}
