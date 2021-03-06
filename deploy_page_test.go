package main

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestIfNoDeployPageExist(t *testing.T) {
	dir, err := ioutil.TempDir("", "deploy")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	w := httptest.NewRecorder()

	executed := false
	handleDeployPage(&dir, func(w http.ResponseWriter, r *gitRequest) {
		executed = true
	})(w, nil)
	if !executed {
		t.Error("The handler should get executed")
	}
}

func TestIfDeployPageExist(t *testing.T) {
	dir, err := ioutil.TempDir("", "deploy")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	deployPage := "DEPLOY"
	ioutil.WriteFile(filepath.Join(dir, "index.html"), []byte(deployPage), 0600)

	w := httptest.NewRecorder()

	executed := false
	handleDeployPage(&dir, func(w http.ResponseWriter, r *gitRequest) {
		executed = true
	})(w, nil)
	if executed {
		t.Error("The handler should not get executed")
	}
	w.Flush()

	assertResponseCode(t, w, 200)
	assertResponseBody(t, w, deployPage)
}
