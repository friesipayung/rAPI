package codegen

import (
	"strings"
	"testing"

	"github.com/friesipayung/rapi/internal/model"
)

func canonical() *model.HttpRequest {
	return &model.HttpRequest{
		Method: "POST",
		URL:    "https://{{host}}/api/users",
		Params: []model.KV{{Name: "page", Value: "2", Type: "query"}},
		Headers: []model.KV{
			{Name: "Accept", Value: "application/json"},
		},
		Auth: &model.Auth{Type: model.AuthBearer, Bearer: &model.BearerAuth{Token: "tok123"}},
		Body: &model.Body{Type: model.BodyJSON, Data: `{"name":"budi"}`},
	}
}

func allTargets(t *testing.T) []Target {
	t.Helper()
	ts := Targets()
	if len(ts) < 14 {
		t.Fatalf("expected >=14 targets, got %d", len(ts))
	}
	return ts
}

func TestAllTargetsGenerateCanonical(t *testing.T) {
	req := canonical()
	for _, tgt := range allTargets(t) {
		out, err := Generate(tgt.Key, req, nil)
		if err != nil {
			t.Fatalf("%s: %v", tgt.Key, err)
		}
		if strings.TrimSpace(out) == "" {
			t.Fatalf("%s: empty output", tgt.Key)
		}
		// URL (with var placeholder) must appear in every snippet
		if !strings.Contains(out, "{{host}}") {
			t.Errorf("%s: missing {{host}} placeholder\n%s", tgt.Key, out)
		}
		// query param page=2 must appear
		if !strings.Contains(out, "page=2") {
			t.Errorf("%s: missing query param page=2\n%s", tgt.Key, out)
		}
		// bearer token must appear
		if !strings.Contains(out, "tok123") {
			t.Errorf("%s: missing bearer token\n%s", tgt.Key, out)
		}
		// JSON body must appear
		if !strings.Contains(out, "budi") {
			t.Errorf("%s: missing body content\n%s", tgt.Key, out)
		}
		// vars comment must list the placeholder
		if !strings.Contains(out, "{{host}}") {
			t.Errorf("%s: vars comment missing", tgt.Key)
		}
	}
}

func TestFormTargetSamples(t *testing.T) {
	req := &model.HttpRequest{
		Method: "POST",
		URL:    "https://x.test/login",
		Auth:   &model.Auth{Type: model.AuthBasic, Basic: &model.BasicAuth{Username: "u", Password: "p"}},
		Body:   &model.Body{Type: model.BodyForm, FormData: []model.KV{{Name: "user", Value: "budi"}, {Name: "pass", Value: "rahasia"}}},
	}
	for _, key := range []string{"python", "go", "php", "ruby", "rust", "csharp", "java", "node-axios", "js-fetch", "swift", "node-native", "curl", "httpie"} {
		out, err := Generate(key, req, nil)
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		if !strings.Contains(out, "budi") {
			t.Errorf("%s: form field missing\n%s", key, out)
		}
	}
}

func TestMultipartHasFilesNote(t *testing.T) {
	req := &model.HttpRequest{
		Method: "POST",
		URL:    "https://x.test/upload",
		Body:   &model.Body{Type: model.BodyMultipart, FormData: []model.KV{{Name: "file", Value: "@data.csv"}}},
	}
	out, err := Generate("python", req, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "open('data.csv'") {
		t.Fatalf("python multipart file missing:\n%s", out)
	}
	out, _ = Generate("curl", req, nil)
	if !strings.Contains(out, "-F 'file=@data.csv'") {
		t.Fatalf("curl -F missing:\n%s", out)
	}
}

func TestAuthComments(t *testing.T) {
	req := &model.HttpRequest{
		Method: "GET",
		URL:    "https://x.test",
		Auth:   &model.Auth{Type: model.AuthOAuth2, OAuth2: &model.OAuth2Auth{GrantType: "client_credentials", TokenURL: "https://auth.test/token"}},
	}
	out, err := Generate("python", req, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "https://auth.test/token") || !strings.Contains(out, "OAuth 2") {
		t.Fatalf("oauth note missing:\n%s", out)
	}
}

func TestUnknownTarget(t *testing.T) {
	if _, err := Generate("nope", canonical(), nil); err == nil {
		t.Fatal("expected error for unknown target")
	}
}

func TestBuildErrors(t *testing.T) {
	if _, err := Build(nil, nil); err == nil {
		t.Fatal("nil request should error")
	}
	if _, err := Build(&model.HttpRequest{Method: "GET"}, nil); err == nil {
		t.Fatal("missing URL should error")
	}
}

func TestTimeoutEmitted(t *testing.T) {
	req := canonical()
	out, err := Generate("curl", req, &model.Settings{Timeout: 12})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "-m 12") {
		t.Fatalf("curl timeout missing:\n%s", out)
	}
	out, _ = Generate("go", req, &model.Settings{Timeout: 12})
	if !strings.Contains(out, "12 * time.Second") {
		t.Fatalf("go timeout missing:\n%s", out)
	}
}

func TestDisabledParamsExcluded(t *testing.T) {
	req := canonical()
	req.Params = append(req.Params, model.KV{Name: "draft", Value: "1", Type: "query", Disabled: true})
	out, err := Generate("curl", req, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "draft=1") {
		t.Fatalf("disabled param leaked:\n%s", out)
	}
}
