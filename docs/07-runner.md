# 07 — Collection runner

The runner executes every HTTP request of a collection (or a folder subtree) in
tree order and reports a per-step result. The GUI drawer and the `rapi run` CLI
share the same machinery: the plan is built from the same tree the sidebar
serves (`PlanFromTree`), each step is executed by the same request engine the
editor uses, and the same report aggregation feeds both.

## 1. Running from the UI

Select a collection or folder in the sidebar and click the **run** button in
the toolbar (send icon — disabled for a single request, which has nothing to
chain). The runner opens as a drawer on the right:

- Header shows `Run: <name>`; the environment picker defaults to the
  collection's active environment (the one the editor's env picker persists).
- **Stop on failure** skips all remaining steps after the first failed one.
- The step list is seeded from the tree before the run starts, so the whole
  plan (with per-step spinners) is visible before the first event arrives.
- Each completed row shows ✓ passed / ✕ failed / → skipped, the method, the
  status code (red on ≥ 400 or a transport error) and the duration. Click a
  completed step to expand its full response (same response panel as the
  editor).
- The footer counts `passed · failed · skipped` and the total run time.

Runs execute in the background; progress arrives as `runner-step` events and
the aggregated `runner-done` report replaces the live rows at the end.

A step counts as failed when the request errors at the transport level, the
HTTP status is ≥ 400, a script errors, or any assertion fails. Skipped steps
never count as failed.

## 2. Variables & precedence during a run

Inside a run the merge order (highest wins) is:

```
run scope (chaining) > environment > folder > collection
```

This matches single-request execution (environment > folder > collection,
see [02-data-format.md](02-data-format.md)) with one addition: the **run
scope** sits on top. Scripts — pre or post — can still override any of these
per request; whatever a script sets is also written back into the run scope,
which is what makes chaining possible.

## 3. Request chaining

A post-script can set variables with `rapi.vars.set(k, v)` (or
`rapi.env.set`, same effect). Internally both write `__out.vars`, and the
runner folds those into the run scope so every later step of the run sees
them via `{{k}}`. A token obtained in step 1 is available in step 2 — no
files, no external state:

```yaml
# 01-login.yml — POST /auth/login
runtime:
  scripts:
    - type: post
      content: |
        const token = rapi.response.body.data.access_token
        rapi.vars.set("token", token)
```

```yaml
# 02-get-orders.yml — GET /orders
http:
  headers:
    - name: Authorization
      value: "Bearer {{token}}"
```

The chain lives only for the duration of the run (a single-request send gets
a fresh, empty scope). Cross-run state should live in an environment instead.

## 4. CLI reference

The GUI is not required — the same runner ships in the `rapi` binary:

```
rapi run <collectionID> [filter...] [--env name] [--stop-on-failure]
         [--report pretty|json|junit] [--out file] [--vault /path]
         [--cookies=false]
```

| Argument / flag | Meaning |
|---|---|
| `collectionID` | collection directory name inside the vault (required) |
| `filter...` | optional node ids (`users`, `users/list`) limiting the run; multiple filters are OR-ed together and deduplicated, keeping first-occurrence order |
| `--env name` | environment to use; default is the collection's active env (matched case-insensitively) |
| `--stop-on-failure` | skip all remaining steps after the first failed one |
| `--report fmt` | `pretty` (default), `json` or `junit` |
| `--out file` | write the report to a file instead of stdout; pretty progress still streams to stdout |
| `--vault path` | vault directory; default is the current directory when it contains `.rapi/config.yml` |
| `--cookies` | reserved no-op — accepted for script compatibility, never wired (see §6) |

Exit codes:

| Code | Meaning |
|---|---|
| `0` | all steps passed (or the plan was empty) |
| `1` | at least one step failed |
| `2` | configuration error — bad flags, no/invalid vault, unknown collection, or the report could not be written |

Output layout: with `--report json` or `--report junit` the report replaces
the pretty text on stdout so stdout stays machine-readable; with `--out` the
report goes to the file and the pretty step lines keep streaming to stdout.
The `json` report is the run report (`startedAt`, `envUsed`, `totalMs`,
`failCount`, `steps[]`); `junit` emits a `<testsuite>` with one `<testcase>`
per step (failures and skips included), readable by any CI JUnit publisher.

## 5. The vault-in-git + CI combo

Because a vault is an ordinary git repository, your API tests version and
ship with your code. A GitHub Actions workflow can check the vault out and
run it — no daemon, no cloud service, just the `rapi` binary and the repo:

```yaml
name: api-tests
on: [push]

jobs:
  run-collection:
    runs-on: ubuntu-latest
    steps:
      # The vault is its own git repo — check it out like any code.
      - uses: actions/checkout@v4
        with:
          repository: acme/api-vault        # your vault repo (private is fine)
          token: ${{ secrets.VAULT_PAT }}   # PAT with read access to the vault repo
          path: vault

      - name: Install rAPI from GitHub releases
        run: |
          url=$(curl -fsSL https://api.github.com/repos/friesipayung/rAPI/releases/latest \
                | grep -o 'https://[^"]*_amd64\.deb' | head -1)
          curl -fsSL -o rapi.deb "$url"
          sudo apt install -y ./rapi.deb

      - name: Run the collection (JUnit report)
        run: |
          rapi run smoke-api --vault "$GITHUB_WORKSPACE/vault" \
            --env staging --report junit --out junit.xml

      - name: Publish JUnit report
        if: always()
        uses: mikepenz/action-junit-report@v4
        with:
          report_paths: junit.xml
```

The `--report junit` exit code (`1` on failures) fails the job; the
`if: always()` publish step still uploads the report so failures show up as
annotations. Note on secrets: keyring-backed secret variables resolve from
the OS keyring, which a bare CI runner does not provide (missing values are
left empty). Keep CI credentials in a dedicated environment's plain
variables, or commit an environment file meant for CI.

## 6. Cookies in runs

- **UI runs** use the app's persistent cookie jar: cookies received during a
  run are stored and sent on later steps of that run (login → session, no
  scripting needed). A request's `settings.sendCookies: false` opts that
  request out entirely — nothing sent, nothing stored.
- **CLI runs never touch a cookie jar** — no store is read, no jar is
  attached, nothing is persisted. The `--cookies` flag is accepted but
  unwired so existing scripts pass it harmlessly. If your flow needs cookies,
  run it in the UI or chain tokens explicitly (§3).
- The jar lives outside the git vault on purpose (cookies are credentials):
  `$XDG_STATE_HOME/rapi/cookies/<sha256(vault path)>.json`
  (`XDG_STATE_HOME` defaults to `~/.local/state`). The Cookies drawer in the
  UI can inspect and clear it.
