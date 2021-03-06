package main

import (
	"bytes"
	"compress/gzip"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestServingNonExistingFile(t *testing.T) {
	dir := "/path/to/non/existing/directory"
	httpRequest, _ := http.NewRequest("GET", "/file", nil)
	request := &gitRequest{
		Request:         httpRequest,
		relativeURIPath: "/static/file",
	}

	w := httptest.NewRecorder()
	handleServeFile(&dir, CacheDisabled, nil)(w, request)
	assertResponseCode(t, w, 404)
}

func TestServingDirectory(t *testing.T) {
	dir, err := ioutil.TempDir("", "deploy")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	httpRequest, _ := http.NewRequest("GET", "/file", nil)
	request := &gitRequest{
		Request:         httpRequest,
		relativeURIPath: "/",
	}

	w := httptest.NewRecorder()
	handleServeFile(&dir, CacheDisabled, nil)(w, request)
	assertResponseCode(t, w, 404)
}

func TestServingMalformedUri(t *testing.T) {
	dir := "/path/to/non/existing/directory"
	httpRequest, _ := http.NewRequest("GET", "/file", nil)
	request := &gitRequest{
		Request:         httpRequest,
		relativeURIPath: "/../../../static/file",
	}

	w := httptest.NewRecorder()
	handleServeFile(&dir, CacheDisabled, nil)(w, request)
	assertResponseCode(t, w, 500)
}

func TestExecutingHandlerWhenNoFileFound(t *testing.T) {
	dir := "/path/to/non/existing/directory"
	httpRequest, _ := http.NewRequest("GET", "/file", nil)
	request := &gitRequest{
		Request:         httpRequest,
		relativeURIPath: "/static/file",
	}

	executed := false
	handleServeFile(&dir, CacheDisabled, func(w http.ResponseWriter, r *gitRequest) {
		executed = (r == request)
	})(nil, request)
	if !executed {
		t.Error("The handler should get executed")
	}
}

func TestServingTheActualFile(t *testing.T) {
	dir, err := ioutil.TempDir("", "deploy")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	httpRequest, _ := http.NewRequest("GET", "/file", nil)
	request := &gitRequest{
		Request:         httpRequest,
		relativeURIPath: "/file",
	}

	fileContent := "STATIC"
	ioutil.WriteFile(filepath.Join(dir, "file"), []byte(fileContent), 0600)

	w := httptest.NewRecorder()
	handleServeFile(&dir, CacheDisabled, nil)(w, request)
	assertResponseCode(t, w, 200)
	if w.Body.String() != fileContent {
		t.Error("We should serve the file: ", w.Body.String())
	}
}

func testServingThePregzippedFile(t *testing.T, enableGzip bool) {
	dir, err := ioutil.TempDir("", "deploy")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	httpRequest, _ := http.NewRequest("GET", "/file", nil)
	request := &gitRequest{
		Request:         httpRequest,
		relativeURIPath: "/file",
	}

	if enableGzip {
		httpRequest.Header.Set("Accept-Encoding", "gzip, deflate")
	}

	fileContent := "STATIC"

	var fileGzipContent bytes.Buffer
	fileGzip := gzip.NewWriter(&fileGzipContent)
	fileGzip.Write([]byte(fileContent))
	fileGzip.Close()

	ioutil.WriteFile(filepath.Join(dir, "file.gz"), fileGzipContent.Bytes(), 0600)
	ioutil.WriteFile(filepath.Join(dir, "file"), []byte(fileContent), 0600)

	w := httptest.NewRecorder()
	handleServeFile(&dir, CacheDisabled, nil)(w, request)
	assertResponseCode(t, w, 200)
	if enableGzip {
		assertResponseHeader(t, w, "Content-Encoding", "gzip")
		if bytes.Compare(w.Body.Bytes(), fileGzipContent.Bytes()) != 0 {
			t.Error("We should serve the pregzipped file")
		}
	} else {
		assertResponseCode(t, w, 200)
		assertResponseHeader(t, w, "Content-Encoding", "")
		if w.Body.String() != fileContent {
			t.Error("We should serve the file: ", w.Body.String())
		}
	}
}

func TestServingThePregzippedFile(t *testing.T) {
	testServingThePregzippedFile(t, true)
}

func TestServingThePregzippedFileWithoutEncoding(t *testing.T) {
	testServingThePregzippedFile(t, false)
}
