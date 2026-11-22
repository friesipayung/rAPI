package main

// CLI tests (P0 task 3.1): runCLI against a minimal temp vault built the
// same way as the Unit A chaining fixture. The vault-dependent path runs
// end-to-end here (vault → tree → plan → RunService → runner → output),
// which doubles as the smoke evidence for the `rapi run` mode.

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adrg/xdg"
	"github.com/friesipayung/rapi/internal/runner"
)

// newRunVault writes a minimal one-collection vault into a temp dir and
// returns the vault path. The collection "cli" has two GET requests hitting
// the given base URL; paths served by the test's httptest server decide
// pass/fail per scenario.
func newRunVault(t *testing.T, baseURL string) string {
	t.Helper()

	// keep vault.New()/SetDir away from the real ~/.config/rapi/app.json
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "xdg-config"))
	xdg.Reload()
	t.Cleanup(xdg.Reload)

	root := t.TempDir()
	files := map[string]string{
		".rapi/config.yml": "formatVersion: \"1\"\nname: Test Vault\n",
		"cli/opencollection.yml": `opencollection: "1.0.0"
info:
  name: CLI
`,
		"cli/environments/local.yml": fmt.Sprintf(`name: local
variables:
  - name: baseUrl
    value: %s
`, baseURL),
		"cli/one.yml": `http:
  method: GET
  url: "{{baseUrl}}/one"
`,
		"cli/two.yml": `http:
  method: GET
  url: "{{baseUrl}}/two"
`,
	}
	for rel, content := range files {
		target := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// newRunServer serves /one and /two with 200; anything under /boom with 500.
func newRunServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/one", "/two":
			fmt.Fprint(w, `{"ok":true}`)
		default:
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"ok":false}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestRunCLIHappyPath(t *testing.T) {
	vaultDir := newRunVault(t, newRunServer(t).URL)

	var out bytes.Buffer
	code := runCLIOut(&out, []string{"cli", "--vault", vaultDir, "--env", "local"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; output:\n%s", code, out.String())
	}
	got := out.String()
	if !strings.Contains(got, "✓ GET one → 200") {
		t.Errorf("output must contain the pass line for request one, got:\n%s", got)
	}
	if !strings.Contains(got, "✓ GET two → 200") {
		t.Errorf("output must contain the pass line for request two, got:\n%s", got)
	}
	if !strings.Contains(got, "2 passed, 0 failed, 0 skipped in") {
		t.Errorf("output must contain the summary line, got:\n%s", got)
	}
}

func TestRunCLIFailingRequestExitsOne(t *testing.T) {
	vaultDir := newRunVault(t, newRunServer(t).URL)
	// point request two at the failing path
	two := filepath.Join(vaultDir, "cli", "two.yml")
	if err := os.WriteFile(two, []byte("http:\n  method: GET\n  url: \"{{baseUrl}}/boom\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	code := runCLIOut(&out, []string{"cli", "--vault", vaultDir, "--env", "local"})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; output:\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "✗ GET two → 500") {
		t.Errorf("output must contain the failure line, got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "1 passed, 1 failed, 0 skipped in") {
		t.Errorf("output must contain the summary line, got:\n%s", out.String())
	}
}

func TestRunCLIStopOnFailureSkipsRest(t *testing.T) {
	vaultDir := newRunVault(t, newRunServer(t).URL)
	two := filepath.Join(vaultDir, "cli", "two.yml")
	if err := os.WriteFile(two, []byte("http:\n  method: GET\n  url: \"{{baseUrl}}/boom\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	// reverse the run order with two filters so the failing request runs first
	code := runCLIOut(&out, []string{"cli", "cli/two", "cli/one", "--vault", vaultDir, "--env", "local", "--stop-on-failure"})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; output:\n%s", code, out.String())
	}
	got := out.String()
	if !strings.Contains(got, "- one → skipped") {
		t.Errorf("remaining step must be reported as skipped, got:\n%s", got)
	}
	if !strings.Contains(got, "0 passed, 1 failed, 1 skipped in") {
		t.Errorf("summary must count the skipped step, got:\n%s", got)
	}
}

func TestRunCLIJsonReport(t *testing.T) {
	vaultDir := newRunVault(t, newRunServer(t).URL)
	two := filepath.Join(vaultDir, "cli", "two.yml")
	if err := os.WriteFile(two, []byte("http:\n  method: GET\n  url: \"{{baseUrl}}/boom\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	code := runCLIOut(&out, []string{"cli", "--vault", vaultDir, "--env", "local", "--report", "json"})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; output:\n%s", code, out.String())
	}
	var rep runner.Report
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("stdout must be valid JSON: %v\n%s", err, out.String())
	}
	if rep.FailCount != 1 {
		t.Errorf("json failCount = %d, want 1", rep.FailCount)
	}
	if len(rep.Steps) != 2 {
		t.Errorf("json steps = %d, want 2", len(rep.Steps))
	}
}

func TestRunCLIJunitReportToFile(t *testing.T) {
	vaultDir := newRunVault(t, newRunServer(t).URL)
	reportPath := filepath.Join(t.TempDir(), "report.xml")

	var out bytes.Buffer
	// --out redirects the junit report to the file; pretty progress still
	// goes to stdout
	code := runCLIOut(&out, []string{"cli", "--vault", vaultDir, "--env", "local", "--report", "junit", "--out", reportPath})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; output:\n%s", code, out.String())
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	var suite struct {
		XMLName xml.Name `xml:"testsuite"`
		Tests   int      `xml:"tests,attr"`
		Failure int      `xml:"failures,attr"`
	}
	if err := xml.Unmarshal(data, &suite); err != nil {
		t.Fatalf("report file must be valid XML: %v\n%s", err, data)
	}
	if suite.Tests != 2 || suite.Failure != 0 {
		t.Errorf("junit tests=%d failures=%d, want 2/0", suite.Tests, suite.Failure)
	}
	if !strings.Contains(out.String(), "2 passed, 0 failed, 0 skipped in") {
		t.Errorf("pretty progress must still go to stdout, got:\n%s", out.String())
	}
}

func TestRunCLIMissingCollectionExitsTwo(t *testing.T) {
	vaultDir := newRunVault(t, newRunServer(t).URL)

	var out bytes.Buffer
	code := runCLIOut(&out, []string{"no-such-collection", "--vault", vaultDir})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; output:\n%s", code, out.String())
	}

	// a directory without the .rapi/config.yml marker is not a vault either
	code = runCLIOut(&out, []string{"cli", "--vault", t.TempDir()})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 for a non-vault directory", code)
	}
}

func TestRunCLIBadFlagsExitTwo(t *testing.T) {
	vaultDir := newRunVault(t, newRunServer(t).URL)

	var out bytes.Buffer
	for _, args := range [][]string{
		{},                                    // missing collection id
		{"cli", "--report", "yaml", "--vault", vaultDir}, // bad format
		{"cli", "--no-such-flag", "--vault", vaultDir},   // unknown flag
	} {
		code := runCLIOut(&out, args)
		if code != 2 {
			t.Errorf("runCLI(%v) exit code = %d, want 2", args, code)
		}
	}
}
