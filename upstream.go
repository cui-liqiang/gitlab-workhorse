/*
The upstream type implements http.Handler.

In this file we handle request routing and interaction with the authBackend.
*/

package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

type serviceHandleFunc func(w http.ResponseWriter, r *gitRequest)

type upstream struct {
	httpClient      *http.Client
	httpProxy       *httputil.ReverseProxy
	authBackend     string
	relativeURLRoot string
}

type authorizationResponse struct {
	// GL_ID is an environment variable used by gitlab-shell hooks during 'git
	// push' and 'git pull'
	GL_ID string
	// RepoPath is the full path on disk to the Git repository the request is
	// about
	RepoPath string
	// ArchivePath is the full path where we should find/create a cached copy
	// of a requested archive
	ArchivePath string
	// ArchivePrefix is used to put extracted archive contents in a
	// subdirectory
	ArchivePrefix string
	// CommitId is used do prevent race conditions between the 'time of check'
	// in the GitLab Rails app and the 'time of use' in gitlab-workhorse.
	CommitId string
	// StoreLFSPath is provided by the GitLab Rails application
	// to mark where the tmp file should be placed
	StoreLFSPath string
	// LFS object id
	LfsOid string
	// LFS object size
	LfsSize int64
	// TmpPath is the path where we should store temporary files
	// This is set by authorization middleware
	TempPath string
}

// A gitRequest is an *http.Request decorated with attributes returned by the
// GitLab Rails application.
type gitRequest struct {
	*http.Request
	authorizationResponse
	u *upstream

	// This field contains the URL.Path stripped from RelativeUrlRoot
	relativeURIPath string
}

func newUpstream(authBackend string, authTransport http.RoundTripper) *upstream {
	gitlabURL, err := url.Parse(authBackend)
	if err != nil {
		log.Fatalln(err)
	}
	relativeURLRoot := gitlabURL.Path
	if !strings.HasSuffix(relativeURLRoot, "/") {
		relativeURLRoot += "/"
	}

	// If the relative URL is '/foobar' and we tell httputil.ReverseProxy to proxy
	// to 'http://example.com/foobar' then we get a redirect loop, so we clear the
	// Path field here.
	gitlabURL.Path = ""

	up := &upstream{
		authBackend:     authBackend,
		httpClient:      &http.Client{Transport: authTransport},
		httpProxy:       httputil.NewSingleHostReverseProxy(gitlabURL),
		relativeURLRoot: relativeURLRoot,
	}
	up.httpProxy.Transport = authTransport
	return up
}

func (u *upstream) ServeHTTP(ow http.ResponseWriter, r *http.Request) {
	var g httpRoute

	w := newLoggingResponseWriter(ow)
	defer w.Log(r)

	// Drop WebSocket connection and CONNECT method
	if r.RequestURI == "*" {
		httpError(&w, r, "Connection upgrade not allowed", http.StatusBadRequest)
		return
	}

	// Disallow connect
	if r.Method == "CONNECT" {
		httpError(&w, r, "CONNECT not allowed", http.StatusBadRequest)
		return
	}

	// Check URL Root
	URIPath := cleanURIPath(r.URL.Path)
	if !strings.HasPrefix(URIPath, u.relativeURLRoot) && URIPath+"/" != u.relativeURLRoot {
		httpError(&w, r, fmt.Sprintf("Not found %q", URIPath), http.StatusNotFound)
		return
	}

	// Strip prefix and add "/"
	// To match against non-relative URL
	// Making it simpler for our matcher
	relativeURIPath := cleanURIPath(strings.TrimPrefix(URIPath, u.relativeURLRoot))

	// Look for a matching Git service
	foundService := false
	for _, g = range httpRoutes {
		if g.method != "" && r.Method != g.method {
			continue
		}

		if g.regex == nil || g.regex.MatchString(relativeURIPath) {
			foundService = true
			break
		}
	}
	if !foundService {
		// The protocol spec in git/Documentation/technical/http-protocol.txt
		// says we must return 403 if no matching service is found.
		httpError(&w, r, "Forbidden", http.StatusForbidden)
		return
	}

	request := gitRequest{
		Request:         r,
		relativeURIPath: relativeURIPath,
		u:               u,
	}

	g.handleFunc(&w, &request)
}
