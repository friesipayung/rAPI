package engine

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/friesipayung/rapi/internal/model"
)

func TestInterpolate(t *testing.T) {
	vars := Vars{"baseUrl": "https://api.test", "token": "abc123"}
	cases := []struct{ in, want string }{
		{"{{baseUrl}}/users", "https://api.test/users"},
		{"Bearer {{ token }}", "Bearer abc123"},
		{"{{unknown}}", "{{unknown}}"},
		{"{{baseUrl}}?t={{token}}", "https://api.test?t=abc123"},
		{"", ""},
	}
	for _, c := range cases {
		if got := Interpolate(c.in, vars); got != c.want {
			t.Errorf("Interpolate(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRunBasicJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"hello": r.Header.Get("X-Test")})
	}))
	defer srv.Close()

	exec := &Executor{}
	res := exec.Run(&model.HttpRequest{
		Method:  http.MethodPost,
		URL:     srv.URL,
		Headers: []model.KV{{Name: "X-Test", Value: "v1"}},
		Body:    &model.Body{Type: model.BodyJSON, Data: `{"a":1}`},
	}, nil, "")
	if res.Error != "" {
		t.Fatalf("unexpected error: %s", res.Error)
	}
	if res.Status != 200 {
		t.Fatalf("status = %d", res.Status)
	}
	if res.Body == nil || res.Body.Type != "json" {
		t.Fatalf("body type = %+v", res.Body)
	}
	if !strings.Contains(res.Body.Data, `"hello": "v1"`) {
		t.Fatalf("body = %s", res.Body.Data)
	}
	if res.TimeMs < 0 || res.SizeBytes == 0 {
		t.Fatalf("timing/size not recorded: %dms %dB", res.TimeMs, res.SizeBytes)
	}
}

func TestRunBearerAndApiKeyQuery(t *testing.T) {
	var gotAuth, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotQuery = r.URL.Query().Get("api_key")
	}))
	defer srv.Close()

	exec := &Executor{}
	res := exec.Run(&model.HttpRequest{
		Method: http.MethodGet,
		URL:    srv.URL,
		Auth:   &model.Auth{Type: model.AuthBearer, Bearer: &model.BearerAuth{Token: "tok"}},
	}, nil, "")
	if res.Error != "" || gotAuth != "Bearer tok" {
		t.Fatalf("bearer: %v auth=%q", res.Error, gotAuth)
	}

	res = exec.Run(&model.HttpRequest{
		Method: http.MethodGet,
		URL:    srv.URL,
		Auth:   &model.Auth{Type: model.AuthApiKey, ApiKey: &model.ApiKeyAuth{Key: "api_key", Value: "secret", In: "query"}},
	}, nil, "")
	if res.Error != "" || gotQuery != "secret" {
		t.Fatalf("apikey query: %v query=%q", res.Error, gotQuery)
	}
}

func TestRunFormBody(t *testing.T) {
	var gotCT, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
	}))
	defer srv.Close()

	exec := &Executor{}
	res := exec.Run(&model.HttpRequest{
		Method: http.MethodPost,
		URL:    srv.URL,
		Body:   &model.Body{Type: model.BodyForm, FormData: []model.KV{{Name: "user", Value: "ali"}, {Name: "x", Value: "1"}}},
	}, nil, "")
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	if gotCT != "application/x-www-form-urlencoded" {
		t.Fatalf("content type = %q", gotCT)
	}
	if !strings.Contains(gotBody, "user=ali") {
		t.Fatalf("body = %q", gotBody)
	}
}

func TestRunNoRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/final", http.StatusFound)
		}
	}))
	defer srv.Close()

	follow := false
	exec := &Executor{}
	res := exec.Run(&model.HttpRequest{Method: "GET", URL: srv.URL + "/start"},
		&model.Settings{FollowRedirects: &follow}, "")
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	if res.Status != http.StatusFound {
		t.Fatalf("status = %d, want 302 (no redirect follow)", res.Status)
	}
}

func TestEffectiveAuthChain(t *testing.T) {
	col := &model.CollectionDoc{}
	col.Config.Request = &model.RequestDefaults{Auth: &model.Auth{Type: model.AuthBasic, Basic: &model.BasicAuth{Username: "col"}}}
	folder := &model.FolderDoc{}
	folder.Request = &model.RequestDefaults{Auth: &model.Auth{Type: model.AuthInherit}}

	got := EffectiveAuth(col, folder, nil)
	if got == nil || got.Basic.Username != "col" {
		t.Fatalf("inherit should fall through to collection, got %+v", got)
	}

	reqAuth := &model.Auth{Type: model.AuthBearer, Bearer: &model.BearerAuth{Token: "req"}}
	if got := EffectiveAuth(col, folder, reqAuth); got.Bearer.Token != "req" {
		t.Fatalf("request auth should win, got %+v", got)
	}
}

func TestEffectiveInterpolation(t *testing.T) {
	col := &model.CollectionDoc{}
	col.Config.Request = &model.RequestDefaults{Variables: []model.Variable{{Name: "baseUrl", Value: "https://col.example"}}}
	env := &model.EnvironmentDoc{Name: "local", Variables: []model.Variable{{Name: "baseUrl", Value: "https://local.example"}, {Name: "token", Value: "t1"}}}

	doc := &model.RequestDoc{}
	doc.Http.Method = "GET"
	doc.Http.URL = "{{baseUrl}}/x"
	doc.Http.Auth = &model.Auth{Type: model.AuthBearer, Bearer: &model.BearerAuth{Token: "{{token}}"}}

	eff, _, _ := Effective(col, nil, doc, env, func(string) *model.EnvironmentDoc { return nil })
	if eff.URL != "https://local.example/x" {
		t.Fatalf("env should override collection var, url = %q", eff.URL)
	}
	if eff.Auth.Bearer.Token != "t1" {
		t.Fatalf("auth token = %q", eff.Auth.Bearer.Token)
	}
}

func TestDigestAuthFlow(t *testing.T) {
	realm, nonce := "rapi@test", "deadbeef"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Digest realm="%s", nonce="%s", qop=auth, algorithm=MD5`, realm, nonce))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// verify the response hash
		auth := r.Header.Get("Authorization")
		var resp, cnonce, nc, uri string
		for _, part := range strings.Split(auth, ", ") {
			kv := strings.SplitN(part, "=", 2)
			switch kv[0] {
			case "response":
				resp = strings.Trim(kv[1], `"`)
			case "cnonce":
				cnonce = strings.Trim(kv[1], `"`)
			case "nc":
				nc = kv[1]
			case "uri":
				uri = strings.Trim(kv[1], `"`)
			}
		}
		ha1 := md5.Sum([]byte("user:" + realm + ":pass"))
		ha2 := md5.Sum([]byte("GET:" + uri))
		want := md5.Sum([]byte(hex.EncodeToString(ha1[:]) + ":" + nonce + ":" + nc + ":" + cnonce + ":auth:" + hex.EncodeToString(ha2[:])))
		if resp != hex.EncodeToString(want[:]) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	exec := &Executor{}
	res := exec.Run(&model.HttpRequest{
		Method: http.MethodGet,
		URL:    srv.URL,
		Auth:   &model.Auth{Type: model.AuthDigest, Digest: &model.DigestAuth{Username: "user", Password: "pass"}},
	}, nil, "")
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	if res.Status != 200 {
		t.Fatalf("digest auth failed, status = %d", res.Status)
	}
}

func TestClassifyBody(t *testing.T) {
	rb := classifyBody("application/json; charset=utf-8", []byte(`{"a":1}`))
	if rb.Type != "json" || !strings.Contains(rb.Data, "\n") {
		t.Fatalf("json pretty failed: %+v", rb)
	}
	rb = classifyBody("image/png", []byte{0x89, 0x50})
	if rb.Type != "binary" {
		t.Fatalf("binary misclassified: %s", rb.Type)
	}
}
