package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/friesipayung/rapi/internal/model"
	"github.com/friesipayung/rapi/internal/store"
)

func vaultFor(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".rapi"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestImportPostman(t *testing.T) {
	v := vaultFor(t)
	src := filepath.Join(t.TempDir(), "postman.json")
	write(t, src, `{
		"info": {"name": "Postman Demo", "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},
		"item": [
			{
				"name": "Users",
				"item": [
					{
						"name": "Get User",
						"request": {
							"method": "GET",
							"url": {"raw": "https://api.test/users/1?full=true", "query": [{"key": "full", "value": "true"}]},
							"header": [{"key": "Accept", "value": "application/json"}],
							"auth": {"type": "bearer", "bearer": [{"key": "token", "value": "{{token}}"}]},
							"description": "# Get a user"
						},
						"response": [{"name": "200 OK", "status": "OK", "code": 200, "body": "{\"id\":1}", "header": [{"key": "Content-Type", "value": "application/json"}]}]
					},
					{
						"name": "Create User",
						"request": {
							"method": "POST",
							"url": "https://api.test/users",
							"body": {"mode": "raw", "raw": "{\"name\":\"x\"}", "options": {"raw": {"language": "json"}}}
						}
					}
				]
			},
			{
				"name": "Login",
				"request": {
					"method": "POST",
					"url": "https://api.test/login",
					"body": {"mode": "urlencoded", "urlencoded": [{"key": "user", "value": "a"}, {"key": "pass", "value": "b"}]},
					"auth": {"type": "basic", "basic": [{"key": "username", "value": "u"}, {"key": "password", "value": "p"}]}
				}
			}
		]
	}`)

	if got := Detect(src); got != "postman" {
		t.Fatalf("detect = %q", got)
	}
	res, err := ImportFromPath(v, src, "auto", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Requests != 3 || res.Folders != 1 {
		t.Fatalf("result = %+v", res)
	}

	doc, err := store.LoadRequestDoc(v, res.CollectionID, "users/get-user")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Http.URL != "https://api.test/users/1?full=true" {
		t.Fatalf("url = %q", doc.Http.URL)
	}
	if len(doc.Http.Params) != 1 || doc.Http.Params[0].Name != "full" {
		t.Fatalf("params = %+v", doc.Http.Params)
	}
	if doc.Http.Auth == nil || doc.Http.Auth.Bearer.Token != "{{token}}" {
		t.Fatalf("auth = %+v", doc.Http.Auth)
	}
	if doc.Docs != "# Get a user" {
		t.Fatalf("docs = %q", doc.Docs)
	}
	exs := doc.ExamplesAsList()
	if len(exs) != 1 || exs[0].Response.Status != 200 || exs[0].Response.Body.Type != "json" {
		t.Fatalf("examples = %+v", exs)
	}

	doc2, _ := store.LoadRequestDoc(v, res.CollectionID, "login")
	if doc2.Http.Body == nil || doc2.Http.Body.Type != model.BodyForm || len(doc2.Http.Body.FormData) != 2 {
		t.Fatalf("login body = %+v", doc2.Http.Body)
	}
	if doc2.Http.Auth.Basic.Username != "u" {
		t.Fatalf("login auth = %+v", doc2.Http.Auth)
	}
}

func TestImportBruno(t *testing.T) {
	v := vaultFor(t)
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "auth"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(src, "get anything.bru"), strings.Join([]string{
		"meta {",
		"  name: get anything",
		"  type: http",
		"  seq: 1",
		"}",
		"",
		"get {",
		"  url: {{baseUrl}}/anything",
		"}",
		"",
		"headers {",
		"  x-test: 1",
		"}",
		"",
		"query {",
		"  full: true",
		"}",
		"",
		"auth {",
		"  bearer {",
		"    token: {{token}}",
		"  }",
		"}",
		"",
		"body:json {",
		"  {\"k\": \"v\"}",
		"}",
		"",
		"docs {",
		"  # Bruno request",
		"}",
	}, "\n"))
	write(t, filepath.Join(src, "auth", "login.bru"), strings.Join([]string{
		"meta {",
		"  name: login",
		"  type: http",
		"}",
		"",
		"post {",
		"  url: {{baseUrl}}/login",
		"}",
		"",
		"body:form {",
		"  urlencoded {",
		"    user: ali",
		"  }",
		"}",
	}, "\n"))
	write(t, filepath.Join(src, "folder.bru"), strings.Join([]string{
		"vars {",
		"  host: https://example.test",
		"}",
	}, "\n"))

	if got := Detect(src); got != "bruno" {
		t.Fatalf("detect = %q", got)
	}
	res, err := ImportFromPath(v, src, "bruno", "My Bruno")
	if err != nil {
		t.Fatal(err)
	}
	if res.CollectionID != "my-bruno" || res.Requests != 2 || res.Folders != 1 {
		t.Fatalf("result = %+v", res)
	}
	doc, err := store.LoadRequestDoc(v, res.CollectionID, "auth/login")
	if err != nil {
		t.Fatalf("load auth/login: %v", err)
	}
	if doc.Http.Method != "POST" || doc.Http.Body.Type != model.BodyForm {
		t.Fatalf("login = %+v body %+v", doc.Http, doc.Http.Body)
	}
}

func TestImportInsomnia(t *testing.T) {
	v := vaultFor(t)
	src := filepath.Join(t.TempDir(), "insomnia.json")
	write(t, src, `{
		"resources": [
			{"_id": "wrk_1", "_type": "workspace", "name": "Insomnia Demo"},
			{"_id": "env_1", "_type": "environment", "parentId": "wrk_1", "name": "Base", "data": {"baseUrl": "https://api.test"}},
			{"_id": "fld_1", "_type": "folder", "parentId": "wrk_1", "name": "Admin"},
			{"_id": "req_1", "_type": "request", "parentId": "fld_1", "name": "Delete User", "method": "DELETE", "url": "https://api.test/users/1", "authentication": {"type": "basic", "username": "u", "password": "p"}},
			{"_id": "req_2", "_type": "request", "parentId": "wrk_1", "name": "Patch User", "method": "PATCH", "url": "https://api.test/users/1?full=1", "body": {"mimeType": "application/json", "text": "{\"ok\":true}"}, "headers": [{"name": "X-A", "value": "b", "disabled": true}]}
		]
	}`)

	if got := Detect(src); got != "insomnia" {
		t.Fatalf("detect = %q", got)
	}
	res, err := ImportFromPath(v, src, "auto", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Requests != 2 || res.Folders != 1 || res.Environments != 1 {
		t.Fatalf("result = %+v", res)
	}
	doc, _ := store.LoadRequestDoc(v, res.CollectionID, "admin/delete-user")
	if doc.Http.Method != "DELETE" || doc.Http.Auth.Basic.Password != "p" {
		t.Fatalf("delete-user = %+v", doc.Http)
	}
	doc2, _ := store.LoadRequestDoc(v, res.CollectionID, "patch-user")
	if len(doc2.Http.Params) != 1 || doc2.Http.Params[0].Name != "full" {
		t.Fatalf("patch params = %+v", doc2.Http.Params)
	}
	if doc2.Http.Body.Type != model.BodyJSON {
		t.Fatalf("patch body = %+v", doc2.Http.Body)
	}
	if len(doc2.Http.Headers) != 1 || !doc2.Http.Headers[0].Disabled {
		t.Fatalf("patch headers = %+v", doc2.Http.Headers)
	}
}

func TestValidateCollection(t *testing.T) {
	v := vaultFor(t)
	src := filepath.Join(t.TempDir(), "p.json")
	write(t, src, `{"info": {"name": "Val", "schema": "v2.1.0"}, "item": [
		{"name": "A", "request": {"method": "GET", "url": "https://x.test"}}
	]}`)
	res, err := ImportFromPath(v, src, "postman", "")
	if err != nil {
		t.Fatal(err)
	}
	rep := ValidateCollection(v, res.CollectionID)
	if !rep.OK || rep.Requests != 1 {
		t.Fatalf("report = %+v", rep)
	}

	doc, _ := store.LoadRequestDoc(v, res.CollectionID, "a")
	doc.Http.URL = ""
	if err := store.SaveRequestDoc(v, res.CollectionID, "a", doc); err != nil {
		t.Fatal(err)
	}
	rep2 := ValidateCollection(v, res.CollectionID)
	if rep2.OK || len(rep2.Problems) == 0 {
		t.Fatalf("report2 = %+v", rep2)
	}
	if !strings.Contains(rep2.Problems[0], "http.url") {
		t.Fatalf("problems = %v", rep2.Problems)
	}
}

func TestImportUnknownFormat(t *testing.T) {
	v := vaultFor(t)
	src := filepath.Join(t.TempDir(), "x.txt")
	write(t, src, "not a collection")
	if _, err := ImportFromPath(v, src, "auto", ""); err == nil {
		t.Fatal("expected error for unknown format")
	}
}
