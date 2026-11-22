package engine

// Jar tests (P0 task 6.2): with RunOpts{Jar} the http.Client stores cookies
// set by the server and replays them on the next request with the same jar;
// without a jar nothing is stored or sent.

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"

	"github.com/friesipayung/rapi/internal/model"
)

func TestRunWithJarStoresAndReplaysCookies(t *testing.T) {
	// Every response sets sid=sesame; the body reports whether the request
	// carried it, so the assertions below read straight off the result body.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Headers must be set before the first body write.
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: "sesame", Path: "/"})
		if _, err := r.Cookie("sid"); err == nil {
			fmt.Fprint(w, "with-cookie")
		} else {
			fmt.Fprint(w, "no-cookie")
		}
	}))
	defer srv.Close()

	req := func() *model.HttpRequest {
		return &model.HttpRequest{Method: "GET", URL: srv.URL}
	}
	e := &Executor{}

	// Without a jar: never stored, never replayed.
	r1 := e.Run(req(), nil, "")
	if r1.Error != "" || r1.Body.Data != "no-cookie" {
		t.Fatalf("request 1 (no jar) = %q / %q", r1.Error, r1.Body.Data)
	}
	r2 := e.Run(req(), nil, "")
	if r2.Body.Data != "no-cookie" {
		t.Fatalf("request 2 without jar must stay cookieless, got %q", r2.Body.Data)
	}

	// With a shared jar: request 1 receives none, request 2 replays it.
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	r3 := e.Run(req(), nil, "", RunOpts{Jar: jar})
	if r3.Error != "" || r3.Body.Data != "no-cookie" {
		t.Fatalf("request 1 (jar) = %q / %q", r3.Error, r3.Body.Data)
	}
	r4 := e.Run(req(), nil, "", RunOpts{Jar: jar})
	if r4.Body.Data != "with-cookie" {
		t.Fatalf("request 2 with shared jar must carry the cookie, got %q", r4.Body.Data)
	}
}

func TestRunOptsLastWins(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie("sid"); err == nil {
			fmt.Fprint(w, "with-cookie")
		} else {
			fmt.Fprint(w, "no-cookie")
		}
	}))
	defer srv.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	var empty http.CookieJar // nil

	e := &Executor{}
	// A nil jar in the last opt must override an earlier jar (last wins).
	res := e.Run(&model.HttpRequest{Method: "GET", URL: srv.URL}, nil, "", RunOpts{Jar: jar}, RunOpts{Jar: empty})
	if res.Error != "" || res.Body.Data != "no-cookie" {
		t.Fatalf("nil jar in last opt must disable cookies: %q / %q", res.Error, res.Body.Data)
	}
}
