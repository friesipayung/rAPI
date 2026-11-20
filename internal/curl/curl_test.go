package curl

import (
	"testing"

	"github.com/friesipayung/rapi/internal/model"
)

func TestParseSimpleGET(t *testing.T) {
	r, err := Parse(`curl https://api.example.com/users`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Method != "GET" || r.URL != "https://api.example.com/users" {
		t.Fatalf("got %s %s", r.Method, r.URL)
	}
}

func TestParseHeadersAndMethod(t *testing.T) {
	r, err := Parse(`curl -X PUT 'https://x.test/api' \
  -H 'Authorization: Bearer abc123' \
  -H 'Accept: application/json'`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Method != "PUT" {
		t.Fatalf("method = %s", r.Method)
	}
	if len(r.Headers) != 2 || r.Headers[0].Value != "Bearer abc123" {
		t.Fatalf("headers = %+v", r.Headers)
	}
}

func TestParseJSONBody(t *testing.T) {
	r, err := Parse(`curl -X POST https://x.test/users -H 'Content-Type: application/json' -d '{"name":"budi","age":30}'`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Body == nil || r.Body.Type != model.BodyJSON || r.Body.Data != `{"name":"budi","age":30}` {
		t.Fatalf("body = %+v", r.Body)
	}
	if !hasHeader(r.Headers, "Content-Type") {
		t.Fatal("explicit content-type header must be preserved")
	}
}

func TestParseFormBody(t *testing.T) {
	r, err := Parse(`curl -d 'username=budi' -d 'password=rahasia' https://x.test/login`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Method != "POST" {
		t.Fatalf("method = %s", r.Method)
	}
	if r.Body == nil || r.Body.Type != model.BodyForm {
		t.Fatalf("body = %+v", r.Body)
	}
	if len(r.Body.FormData) != 2 || r.Body.FormData[0].Name != "username" {
		t.Fatalf("form = %+v", r.Body.FormData)
	}
}

func TestParseBasicAuth(t *testing.T) {
	r, err := Parse(`curl -u admin:s3cret https://x.test/secure`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Auth == nil || r.Auth.Type != model.AuthBasic || r.Auth.Basic.Username != "admin" || r.Auth.Basic.Password != "s3cret" {
		t.Fatalf("auth = %+v", r.Auth)
	}
}

func TestParseBearerLongFlag(t *testing.T) {
	r, err := Parse(`curl --oauth2-bearer tok123 https://x.test`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Auth == nil || r.Auth.Type != model.AuthBearer || r.Auth.Bearer.Token != "tok123" {
		t.Fatalf("auth = %+v", r.Auth)
	}
}

func TestParseGetWithDataToQuery(t *testing.T) {
	r, err := Parse(`curl -G -d 'page=2' -d 'size=10' https://x.test/items`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Method != "GET" || len(r.Params) != 2 || r.Params[0].Name != "page" {
		t.Fatalf("method=%s params=%+v", r.Method, r.Params)
	}
}

func TestParseHeadAndTimeout(t *testing.T) {
	r, err := Parse(`curl -I -m 15 https://x.test/health`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Method != "HEAD" {
		t.Fatalf("method = %s", r.Method)
	}
}

func TestParseAttachedShortValue(t *testing.T) {
	r, err := Parse(`curl -XDELETE https://x.test/items/7`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Method != "DELETE" {
		t.Fatalf("method = %s", r.Method)
	}
}

func TestParseUserAgentAndLongEquals(t *testing.T) {
	r, err := Parse(`curl --url=https://x.test --user-agent "rapi-test/1.0" https://x.test/fallback`)
	if err != nil {
		t.Fatal(err)
	}
	if r.URL != "https://x.test" {
		t.Fatalf("url = %s", r.URL)
	}
	if !hasHeader(r.Headers, "User-Agent") {
		t.Fatalf("headers = %+v", r.Headers)
	}
}

func TestParseMultipart(t *testing.T) {
	r, err := Parse(`curl -F "name=report" -F "file=@data.csv" https://x.test/upload`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Method != "POST" || r.Body == nil || r.Body.Type != model.BodyMultipart || len(r.Body.FormData) != 2 {
		t.Fatalf("body = %+v", r.Body)
	}
}

func TestParseWget(t *testing.T) {
	r, err := Parse(`wget --method=POST --header='Content-Type: application/json' --body-data='{"a":1}' https://x.test/api`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Method != "POST" || r.URL != "https://x.test/api" {
		t.Fatalf("got %s %s", r.Method, r.URL)
	}
	if r.Body == nil || r.Body.Type != model.BodyJSON {
		t.Fatalf("body = %+v", r.Body)
	}
}

func TestParseWgetSimple(t *testing.T) {
	r, err := Parse(`wget -q https://x.test/file.tar.gz`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Method != "GET" || r.URL != "https://x.test/file.tar.gz" {
		t.Fatalf("got %s %s", r.Method, r.URL)
	}
}

func TestParseRejectsOther(t *testing.T) {
	if _, err := Parse(`http POST https://x.test`); err == nil {
		t.Fatal("expected error for non-curl/wget")
	}
	if _, err := Parse(`curl -d 'x=1'`); err == nil {
		t.Fatal("expected error for missing URL")
	}
}

func TestTokenizeQuotes(t *testing.T) {
	words, err := Tokenize(`curl 'https://x.test/?a=1&b=2' -H "X-Note: he said \"hi\""`)
	if err != nil {
		t.Fatal(err)
	}
	if len(words) != 4 {
		t.Fatalf("words = %q", words)
	}
	if words[1] != "https://x.test/?a=1&b=2" {
		t.Fatalf("word1 = %q", words[1])
	}
	if words[3] != `X-Note: he said "hi"` {
		t.Fatalf("word3 = %q", words[3])
	}
}
