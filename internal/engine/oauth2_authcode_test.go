package engine

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/friesipayung/rapi/internal/model"
)

func timeoutAfter(sec int) <-chan time.Time {
	return time.After(time.Duration(sec) * time.Second)
}

func listenLoopback() (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:0")
}

func redirectURIFor(ln net.Listener) string {
	return fmt.Sprintf("http://127.0.0.1:%d/callback", ln.Addr().(*net.TCPAddr).Port)
}

func TestPKCEPair(t *testing.T) {
	v, c := pkcePair()
	if len(v) < 43 || len(v) > 128 {
		t.Fatalf("verifier length out of RFC range: %d", len(v))
	}
	if strings.ContainsAny(v, "+/=") {
		t.Fatalf("verifier not base64url: %q", v)
	}
	if c == "" || strings.ContainsAny(c, "+/=") {
		t.Fatalf("bad challenge %q", c)
	}
	v2, _ := pkcePair()
	if v == v2 {
		t.Fatal("verifier reuse across calls")
	}
}

func TestBuildAuthURL(t *testing.T) {
	u, err := buildAuthURL("https://sso.test/authorize?foo=1", "cid", "http://127.0.0.1:9/callback", "read write", "st4te", "chal")
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(u)
	q := parsed.Query()
	if q.Get("response_type") != "code" || q.Get("client_id") != "cid" || q.Get("state") != "st4te" ||
		q.Get("code_challenge") != "chal" || q.Get("code_challenge_method") != "S256" ||
		q.Get("scope") != "read write" || q.Get("foo") != "1" {
		t.Fatalf("bad auth URL: %s", u)
	}
	if _, err := buildAuthURL("not a url", "cid", "http://x", "", "", ""); err == nil {
		t.Fatal("expected error for invalid authURL")
	}
}

// TestAuthorizationCodeFlow walks the interactive flow end-to-end: the
// "authorization server" is an httptest server, the "browser" is an HTTP
// client following the 302 into the loopback handler.
func TestAuthorizationCodeFlow(t *testing.T) {
	var gotChallenge, gotVerifier string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/authorize"):
			q := r.URL.Query()
			if q.Get("code_challenge_method") == "S256" {
				gotChallenge = q.Get("code_challenge")
			}
			http.Redirect(w, r, q.Get("redirect_uri")+"?code=XYZ&state="+q.Get("state"), http.StatusFound)
		case strings.HasPrefix(r.URL.Path, "/token"):
			_ = r.ParseForm()
			gotVerifier = r.PostFormValue("code_verifier")
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"access_token":"AT","refresh_token":"RT","expires_in":3600}`)
		}
	}))
	defer srv.Close()

	o := &model.OAuth2Auth{
		GrantType: "authorization_code",
		AuthURL:   srv.URL + "/authorize",
		TokenURL:  srv.URL + "/token",
		ClientID:  "cid",
		UsePKCE:   true,
	}

	ln, err := listenLoopback()
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	redirectURI := redirectURIFor(ln)
	verifier, challenge := pkcePair()
	const state = "teststate"
	authURL, err := buildAuthURL(o.AuthURL, o.ClientID, redirectURI, "api", state, challenge)
	if err != nil {
		t.Fatal(err)
	}

	codeCh := make(chan authCodeResult, 1)
	go http.Serve(ln, loopbackHandler(state, codeCh))

	// "browser": GET the authorization URL and follow the 302 into the
	// loopback handler (default client follows redirects); the final 200 is
	// the loopback handler's success page
	resp, err := http.Get(authURL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	var res authCodeResult
	select {
	case res = <-codeCh:
	case <-timeoutAfter(2):
		t.Fatal("no code captured")
	}
	if res.err != nil {
		t.Fatal(res.err)
	}
	if res.code != "XYZ" {
		t.Fatalf("code = %q", res.code)
	}

	form := url.Values{"grant_type": {"authorization_code"}, "scope": {"api"}}
	access, refresh, err := exchangeAuthorizationCode(context.Background(), o, form, res.code, verifier, redirectURI)
	if err != nil {
		t.Fatal(err)
	}
	if access != "AT" || refresh != "RT" {
		t.Fatalf("tokens = %q / %q", access, refresh)
	}
	if gotChallenge == "" {
		t.Fatal("PKCE challenge not sent")
	}
	if gotVerifier != verifier {
		t.Fatalf("code_verifier mismatch: sent %q got %q", verifier, gotVerifier)
	}
}

func TestStateMismatchRejected(t *testing.T) {
	ch := make(chan authCodeResult, 1)
	h := loopbackHandler("expected", ch)
	req := httptest.NewRequest("GET", "/callback?code=X&state=wrong", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)
	select {
	case res := <-ch:
		if res.err == nil {
			t.Fatal("expected state mismatch error")
		}
	case <-timeoutAfter(1):
		t.Fatal("no result")
	}
}

func TestAuthErrorRejected(t *testing.T) {
	ch := make(chan authCodeResult, 1)
	h := loopbackHandler("expected", ch)
	req := httptest.NewRequest("GET", "/callback?error=access_denied&error_description=nope", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)
	select {
	case res := <-ch:
		if res.err == nil || !strings.Contains(res.err.Error(), "access_denied") {
			t.Fatalf("expected access_denied error, got %v", res.err)
		}
	case <-timeoutAfter(1):
		t.Fatal("no result")
	}
}

func TestParseTokenResponse(t *testing.T) {
	tok, err := parseTokenResponse([]byte(`{"access_token":"a","refresh_token":"r","expires_in":10}`))
	if err != nil || tok.accessToken != "a" || tok.refreshToken != "r" || tok.expiresIn != 10 {
		t.Fatalf("tok=%+v err=%v", tok, err)
	}
	if _, err := parseTokenResponse([]byte(`{"nope":1}`)); err == nil {
		t.Fatal("expected error for missing access_token")
	}
}
