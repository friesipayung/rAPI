package main

// CLI mode `rapi run` (P0 tasks 3.1–3.3): runs a collection through the same
// Runner + RunService the GUI uses — no Wails/application code on this path.
//
// Usage:
//
//	rapi run <collectionID> [filter...] [--env name] [--stop-on-failure]
//	         [--report json|junit|pretty] [--out file] [--vault /path]
//	         [--cookies=false]
//
// Exit codes: 0 = all steps passed (or the plan is empty), 1 = at least one
// failed step, 2 = configuration error (bad flags, no vault, collection not
// found, report write error).

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/friesipayung/rapi/internal/model"
	"github.com/friesipayung/rapi/internal/runner"
	"github.com/friesipayung/rapi/internal/store"
	"github.com/friesipayung/rapi/internal/vault"
)

const (
	runExitOK     = 0
	runExitFailed = 1
	runExitConfig = 2
)

const runUsage = `usage: rapi run <collectionID> [filter...] [--env name] [--stop-on-failure]
                 [--report json|junit|pretty] [--out file] [--vault /path]
                 [--cookies=false]

Runs every HTTP request of a collection (or only the subtrees matching the
positional filters) and prints one line per step plus a summary.

  collectionID        collection directory name inside the vault (required)
  filter...           optional node ids ("users", "users/list") limiting the
                      run; multiple filters are OR-ed together
  --env name          environment to use (default: the collection's active env)
  --stop-on-failure   skip all remaining steps after the first failed one
  --report format     pretty (default), json or junit
  --out file          write the report to file instead of stdout (pretty
                      progress still goes to stdout)
  --vault path        vault directory (default: the current directory when it
                      contains .rapi/config.yml)
  --cookies           reserved (the CLI never uses a cookie jar in this
                      release; the flag is accepted but not wired)
`

// runCLI is the entry point wired from main() for `rapi run <args>`.
func runCLI(args []string) int {
	return runCLIOut(os.Stdout, args)
}

// runCLIOut is runCLI with an injectable stdout (tests capture output there).
// Errors and usage go to stderr.
func runCLIOut(stdout io.Writer, args []string) int {
	flagArgs, positional := splitRunArgs(args)

	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, runUsage) }
	envName := fs.String("env", "", "environment name (default: the collection's active env)")
	stopOnFailure := fs.Bool("stop-on-failure", false, "skip all remaining steps after the first failed one")
	format := fs.String("report", "pretty", "report format: pretty, json or junit")
	out := fs.String("out", "", "write the report to this file instead of stdout (pretty progress still goes to stdout)")
	vaultFlag := fs.String("vault", "", "vault directory (default: the current directory when it contains .rapi/config.yml)")
	// Cookie jar ruling (Unit F): the CLI never uses the jar; the flag stays
	// accepted-but-unwired so scripts can pass --cookies=false harmlessly.
	fs.Bool("cookies", true, "reserved (accepted but not wired; the CLI does not use a cookie jar)")
	if err := fs.Parse(flagArgs); err != nil {
		return runExitConfig
	}

	if len(positional) == 0 {
		fmt.Fprintln(os.Stderr, "rapi run: missing collection id")
		fs.Usage()
		return runExitConfig
	}
	collectionID := positional[0]
	filters := positional[1:]

	switch *format {
	case "pretty", "json", "junit":
	default:
		fmt.Fprintf(os.Stderr, "rapi run: unknown --report format %q (want pretty, json or junit)\n", *format)
		return runExitConfig
	}

	// Vault resolution: --vault wins; otherwise the current directory is
	// used when it contains the vault marker .rapi/config.yml (the same
	// file store.LoadVaultConfig and vault.Open require).
	vaultDir := *vaultFlag
	if vaultDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "rapi run: cannot determine working directory: %v\n", err)
			return runExitConfig
		}
		vaultDir = cwd
	}
	if _, err := os.Stat(filepath.Join(vaultDir, ".rapi", "config.yml")); err != nil {
		fmt.Fprintf(os.Stderr, "rapi run: not a rAPI vault (no .rapi/config.yml in %s); pass --vault /path\n", vaultDir)
		return runExitConfig
	}
	mgr := vault.New()
	if err := mgr.SetDir(vaultDir); err != nil {
		fmt.Fprintf(os.Stderr, "rapi run: cannot open vault: %v\n", err)
		return runExitConfig
	}

	tree, err := store.GetTree(vaultDir, collectionID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rapi run: %v\n", err)
		return runExitConfig
	}

	// Filters are OR-ed: PlanFromTree runs once per filter and the step
	// lists are concatenated, deduplicated by request id and preserving
	// first occurrence (a filter inside another filter's subtree would
	// otherwise produce the request twice). No filter = whole collection.
	var steps []runner.Step
	if len(filters) == 0 {
		steps = runner.PlanFromTree([]*model.TreeNode{tree}, "")
	} else {
		seen := map[string]bool{}
		for _, f := range filters {
			for _, st := range runner.PlanFromTree([]*model.TreeNode{tree}, f) {
				if !seen[st.RequestID] {
					seen[st.RequestID] = true
					steps = append(steps, st)
				}
			}
		}
	}

	// Output layout: pretty progress (step lines + summary) goes to stdout
	// in every mode except json/junit writing their report to stdout —
	// there the report replaces the pretty text so stdout stays
	// machine-readable. --out sends the final report to a file; pretty
	// output still goes to stdout (and with --report pretty the file
	// receives the same pretty text).
	livePretty := *out != "" || *format == "pretty"

	var pretty bytes.Buffer
	onStep := func(sr runner.StepResult) {
		line := formatStepLine(sr)
		pretty.WriteString(line)
		pretty.WriteByte('\n')
		if livePretty {
			fmt.Fprintln(stdout, line)
		}
	}

	scope := runner.NewScope()
	scope.StopOnFailure = *stopOnFailure
	// FINAL RULING (Unit F): the CLI never uses the cookie jar — no store
	// read, no jar attached, nothing persisted. Same RunService/Runner as
	// the GUI, with the app-level jar switch hard-off.
	runSvc := NewRunService(mgr)
	runSvc.SetCookiesEnabled(false)
	rep := (&runner.Runner{Exec: runSvc}).Run(
		context.Background(), steps, *envName, scope, onStep)

	skipped := 0
	for _, sr := range rep.Steps {
		if sr.Skipped {
			skipped++
		}
	}
	passed := len(rep.Steps) - rep.FailCount - skipped
	if passed < 0 {
		passed = 0
	}
	summary := fmt.Sprintf("%d passed, %d failed, %d skipped in %.1fs",
		passed, rep.FailCount, skipped, float64(rep.TotalMs)/1000)
	pretty.WriteString(summary)
	pretty.WriteByte('\n')
	if livePretty {
		fmt.Fprintln(stdout, summary)
	}

	// Deliver the final report.
	switch *format {
	case "pretty":
		if *out != "" {
			if err := os.WriteFile(*out, pretty.Bytes(), 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "rapi run: cannot write report: %v\n", err)
				return runExitConfig
			}
		}
	case "json":
		b, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "rapi run: cannot encode report: %v\n", err)
			return runExitConfig
		}
		b = append(b, '\n')
		if err := deliverReport(*out, stdout, b); err != nil {
			fmt.Fprintf(os.Stderr, "rapi run: %v\n", err)
			return runExitConfig
		}
	case "junit":
		if err := deliverReport(*out, stdout, rep.MarshalJunit()); err != nil {
			fmt.Fprintf(os.Stderr, "rapi run: %v\n", err)
			return runExitConfig
		}
	}

	if rep.FailCount > 0 {
		return runExitFailed
	}
	return runExitOK
}

// deliverReport writes the serialized report to the --out file, or to stdout
// when no file was given.
func deliverReport(outPath string, stdout io.Writer, data []byte) error {
	if outPath == "" {
		_, err := stdout.Write(data)
		return err
	}
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return fmt.Errorf("cannot write report: %w", err)
	}
	return nil
}

// splitRunArgs separates flag arguments from positional arguments so flags
// may appear after positionals (`rapi run col --env local`), as the usage
// line advertises. stdlib flag stops parsing at the first non-flag argument,
// so we pre-partition: value-taking flags consume the next argument,
// `--name=value` forms stand alone, boolean flags stand alone, and
// everything after `--` (or any non-flag token) is positional. Unknown flags
// still reach fs.Parse and fail with exit code 2.
func splitRunArgs(args []string) (flags, positional []string) {
	takesValue := map[string]bool{
		"env":    true,
		"report": true,
		"out":    true,
		"vault":  true,
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if len(a) < 2 || a[0] != '-' {
			positional = append(positional, a)
			continue
		}
		name := strings.TrimPrefix(strings.TrimPrefix(a, "-"), "-")
		if strings.Contains(name, "=") {
			flags = append(flags, a)
			continue
		}
		if takesValue[name] && i+1 < len(args) {
			flags = append(flags, a, args[i+1])
			i++
			continue
		}
		flags = append(flags, a)
	}
	return flags, positional
}

// formatStepLine renders one pretty progress line:
//
//	✓ GET users/list → 200 (123ms)
//	✗ GET users/list → 500 (45ms)
//	- users/list → skipped
//
// Unicode ✓/✗ matches the style already used in the repo's tooling scripts.
func formatStepLine(sr runner.StepResult) string {
	label := sr.Name
	if label == "" {
		label = sr.RequestID
	}
	if sr.Skipped {
		return fmt.Sprintf("- %s → skipped", label)
	}
	method := ""
	if sr.Result != nil && sr.Result.Request != nil && sr.Result.Request.Method != "" {
		method = sr.Result.Request.Method + " "
	}
	if sr.Result == nil {
		return fmt.Sprintf("✗ %s%s → no result (%dms)", method, label, sr.DurationMs)
	}
	detail := strconv.Itoa(sr.Result.Status)
	switch {
	case sr.Result.Error != "":
		detail = sr.Result.Error
	case sr.Result.Status >= 400:
		// keep the status number as the detail
	case sr.Result.ScriptError != "":
		detail = "script: " + sr.Result.ScriptError
	default:
		if a := firstFailedAssertion(sr.Result); a != nil {
			detail = "assertion failed: " + a.Expression + " " + a.Operator
			if a.Expected != "" {
				detail += " " + a.Expected
			}
		}
	}
	mark := "✓"
	if cliStepFailed(sr) {
		mark = "✗"
	}
	return fmt.Sprintf("%s %s%s → %s (%dms)", mark, method, label, detail, sr.DurationMs)
}

// cliStepFailed mirrors internal/runner.stepFailed (unexported there) so the
// ✓/✗ marks agree with the runner's failure definition and FailCount.
func cliStepFailed(sr runner.StepResult) bool {
	if sr.Skipped {
		return false
	}
	if sr.Result == nil {
		return true
	}
	if sr.Result.Error != "" || sr.Result.ScriptError != "" || sr.Result.Status >= 400 {
		return true
	}
	return firstFailedAssertion(sr.Result) != nil
}

func firstFailedAssertion(res *model.RunResult) *model.AssertionResult {
	for i := range res.Assertions {
		if !res.Assertions[i].Pass {
			return &res.Assertions[i]
		}
	}
	return nil
}
