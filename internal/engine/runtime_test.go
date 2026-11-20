package engine

// Phase 4 tests: scripts (goja), assertions, oauth2 config validation.

import (
	"testing"

	"github.com/friesipayung/rapi/internal/model"
)

func TestRunPreScriptSetsVars(t *testing.T) {
	script := model.Script{Type: model.ScriptPre, Content: `rapi.vars.set("token", "abc" + 123); console.log("hello", 1);`}
	out := RunScript(script, &model.HttpRequest{Method: "GET", URL: "https://x.test"}, nil, Vars{"existing": "1"})
	if out.Error != "" {
		t.Fatalf("script error: %s", out.Error)
	}
	if out.Vars["token"] != "abc123" {
		t.Fatalf("token = %q, want abc123", out.Vars["token"])
	}
	if len(out.Console) != 1 || out.Console[0] != "hello 1" {
		t.Fatalf("console = %v", out.Console)
	}
}

func TestRunPreScriptReadsRequestAndVars(t *testing.T) {
	script := model.Script{Type: model.ScriptPre, Content: `rapi.vars.set("u", rapi.request.url + "?" + rapi.env.get("q"));`}
	req := &model.HttpRequest{Method: "GET", URL: "https://x.test/a"}
	out := RunScript(script, req, nil, Vars{"q": "1"})
	if out.Error != "" {
		t.Fatalf("script error: %s", out.Error)
	}
	if out.Vars["u"] != "https://x.test/a?1" {
		t.Fatalf("u = %q", out.Vars["u"])
	}
}

func TestRunPostScriptReadsResponse(t *testing.T) {
	script := model.Script{Type: model.ScriptPost, Content: `if (rapi.response.status !== 200) throw "bad status"; rapi.vars.set("id", rapi.response.body.id);`}
	res := &model.RunResult{Status: 200, Body: &model.ResponseBody{Type: "json", Data: `{"id":42}`}}
	out := RunScript(script, &model.HttpRequest{Method: "GET", URL: "https://x.test"}, res, Vars{})
	if out.Error != "" {
		t.Fatalf("script error: %s", out.Error)
	}
	if out.Vars["id"] != "42" {
		t.Fatalf("id = %q", out.Vars["id"])
	}
}

func TestRunScriptSyntaxError(t *testing.T) {
	script := model.Script{Type: model.ScriptPre, Content: `this is not js !!`}
	out := RunScript(script, &model.HttpRequest{}, nil, Vars{})
	if out.Error == "" {
		t.Fatal("expected a syntax error")
	}
}

func TestAssertionsPassAndFail(t *testing.T) {
	res := &model.RunResult{
		Status: 200,
		Body:   &model.ResponseBody{Type: "json", Data: `{"orders":[{"id":1},{"id":2}]}`},
		Headers: []model.KV{
			{Name: "Content-Type", Value: "application/json"},
		},
	}
	assertions := []model.Assertion{
		{Expression: "res.status", Operator: "equal", Value: "200"},
		{Expression: "res.body.orders.length", Operator: "equal", Value: "2"},
		{Expression: "res.headers['content-type']", Operator: "contains", Value: "json"},
		{Expression: "res.status", Operator: "lessThan", Value: "300"},
		{Expression: "res.status", Operator: "equal", Value: "404"}, // fails
		{Expression: "res.body.missing", Operator: "exists"},        // fails
		{Expression: "res.status", Operator: "equal", Value: "0", Disabled: true},
	}
	got := RunAssertions(assertions, res)
	if len(got) != 6 {
		t.Fatalf("got %d results, want 6 (disabled skipped)", len(got))
	}
	for i, want := range []bool{true, true, true, true, false, false} {
		if got[i].Pass != want {
			t.Fatalf("assertion %d (%s) pass=%v, want %v (actual=%q err=%q)", i, got[i].Expression, got[i].Pass, want, got[i].Actual, got[i].Error)
		}
	}
}

func TestAssertionInvalidExpression(t *testing.T) {
	res := &model.RunResult{Status: 200}
	got := RunAssertions([]model.Assertion{{Expression: "res.status >=", Operator: "equal"}}, res)
	if got[0].Pass || got[0].Error == "" {
		t.Fatalf("expected error result, got %+v", got[0])
	}
}

func TestOAuth2ConfigValidation(t *testing.T) {
	if _, err := FetchOAuth2Token(nil, &model.OAuth2Auth{TokenURL: ""}); err == nil {
		t.Fatal("expected error for empty token URL")
	}
	if _, err := FetchOAuth2Token(nil, &model.OAuth2Auth{TokenURL: "https://x.test/token", GrantType: "implicit"}); err == nil {
		t.Fatal("expected error for unsupported grant type")
	}
}
