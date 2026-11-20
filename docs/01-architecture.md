# rAPI — Architecture

## 1. Overview

Wails v3 bundles the Go backend and the SvelteKit frontend into a single desktop binary.
The frontend does **not** send requests itself (webview → CORS, limited localhost access);
all network and git execution happens in the Go backend.

```
┌────────────────────────────── Wails v3 window ──────────────────────────────┐
│  SvelteKit SPA (ssr=false, adapter-static)                                    │
│  ┌──────────┐ ┌─────────────────────────────┐ ┌───────────────────────────┐ │
│  │ Sidebar  │ │ Tab bar + Request editor    │ │ Response viewer           │ │
│  │ tree/env │ │ params/headers/auth/body    │ │ status/time/headers/body  │ │
│  └──────────┘ └─────────────────────────────┘ └───────────────────────────┘ │
└───────────────────────────────────┬─────────────────────────────────────────┘
                                    │ Wails bindings (generated TS)
┌───────────────────────────────────▼─────────────────────────────────────────┐
│  Go backend (package root: app.go + service structs)                         │
│  VaultService  TreeService  RequestService  EnvService  RunService  SyncSvc  │
│                                                                              │
│  internal/vault   internal/store   internal/engine   internal/git            │
│   (path, init,    (YAML load/save, (net/http client, (shell `git` CLI,       │
│    remote, status) tree, validation) interpolate, auth) commit/push/pull)     │
└───────────────────────────────────┬─────────────────────────────────────────┘
                                    │
                    ┌───────────────┴───────────────┐
                    ▼                               ▼
          <vault-path>/ (git repo)          remote (GitHub/GitLab/Gitea/etc)
```

## 2. Boundaries

| Unit | Responsibility | Depends on |
|---|---|---|
| `internal/model` | Go structs for OpenCollection + run results | YAML lib only |
| `internal/store` | Load/save/validate vault files, collection tree | `model` |
| `internal/git` | All git operations (shell CLI) | `os/exec` |
| `internal/engine` | HTTP execution, interpolation, auth | `model` |
| `internal/vault` | Vault lifecycle, remotes, sync status | `store`, `git` |
| `internal/sync` | Debounced auto-commit/push, status events | `git` |
| service structs | API exposed by Wails to the frontend | the above |

Rule: services contain no business logic; they delegate to `internal/`. The frontend only
calls binding methods, never reads files directly.

## 3. Wails services (bindings)

| Service | Key methods |
|---|---|
| `VaultService` | `Init(path)`, `Open(path)`, `Status()`, `AddRemote(url)`, `ListRemotes()`, `RemoveRemote(name)` |
| `SyncService` | `Status()`, `CommitNow(msg)`, `PushNow()`, `PullNow()` + `sync-status` event |
| `TreeService` | `ListCollections()`, `GetTree(id)`, `CreateCollection`, `RenameCollection`, `DeleteCollection`, `CreateFolder`, `MoveItem`, `Reorder` |
| `RequestService` | `Get(id)`, `Save(id)`, `Create(collectionID, folder, req)`, `Delete(id)`, `SetActiveRequest(id)` |
| `EnvService` | `List(collectionID)`, `Create`, `Save`, `Delete`, `SetActive(collectionID, env)` |
| `RunService` | `Execute(collectionID, requestID, envName)` → `RunResult` |
| `ExampleService` | `List(requestID)`, `Save(requestID, example)`, `Delete(requestID, name)`, `SaveResponseAsExample(requestID, name)` |

`RunResult`: `{ status, statusText, headers[], body{type,data}, timeMs, sizeBytes, requestSnapshot }`.

## 4. Data flow

**Saving a request**
```
editor → RequestService.Save
       → store.WriteYAML(<vault>/<col>/<folder>/<req>.yml)
       → sync.ScheduleCommit (debounce 800ms)
       → sync.SchedulePush   (debounce 3s, if a remote exists)
       → sync-status event → UI indicator
```

**Running a request**
```
RunService.Execute(id, env)
  → store loads request YAML + env YAML + defaults (collection/folder)
  → engine.interpolate ({{var}}) with precedence
  → engine.BuildRequest (method/url/params/headers/body/auth)
  → http.Client.Do (timeout, redirect policy, encodeUrl)
  → RunResult → UI
  → (optional) ExampleService.SaveResponseAsExample
```

**Onboarding**
```
wizard → VaultService.Init(path)
  → if the folder is empty: git init + write .rapi/config.yml + sample collection
  → if it is already a repo: detect it, ask for a remote (optional)
  → test: git status + (if a remote exists) ls-remote
```

## 5. Git engine

**Decision: shell out to the `git` CLI**, not a pure library (go-git).

Why: hosting-agnostic (GitHub/GitLab/Gitea/self-hosted) and auth uses the
credentials already on the host (SSH keys, credential helpers, tokens in remote URLs).
rAPI **does not store tokens**. Tradeoff: the desktop binary requires `git` installed
(usually present on Linux/macOS/Windows); a later mobile build can switch to go-git.

Operations: `init`, `add -A`, `commit -m`, `pull --rebase`, `push`, `status --porcelain`,
`remote add/list/remove`, `ls-remote` (connection test), `rev-parse`.

**Sync policy**
- Local auto-commit: 800ms debounce after file changes.
- Auto-push: 3s debounce, only if a remote exists; push = `pull --rebase` first
  (skipped when the remote is still empty — nothing to pull on first push).
- Manual buttons: commit now, push now, pull now.
- GPG: commits respect the host's signing config; when GPG signing fails (e.g.
  pinentry unavailable), the commit retries unsigned and the fallback is visible
  in the sync log.
- Conflict: stop, set status `conflict`, show conflicting files in the UI, **no**
  auto-resolve. The user resolves manually (git CLI) then clicks pull.
- Environment secrets: `SecretVariable` values never enter commits (see §7).

## 6. HTTP engine

- `net/http` client per request; timeout from `settings.timeout` (0 = default 30s).
- `followRedirects` + `maxRedirects` → custom `CheckRedirect`.
- `encodeUrl`: escape path/params before the request.
- Auth: `inherit` (from folder/collection defaults), `basic`, `bearer`, `apikey`
  (header/query), `digest`. OAuth2/AWSv4/NTLM/WSSE = phase 4.
- Body: `json`, `text`, `xml`, `html`, `binary`, `form-urlencoded`, `multipart-form`,
  `file`.
- `{{var}}` interpolation in URL, params, headers, body, auth values.
- Variable precedence: **runtime > environment > folder > collection**.
- A request snapshot is returned so it can be used directly as the `request` of an example.

## 7. Secrets

- `SecretVariable` (OpenCollection schema) = variable with the `secret: true` flag, **no value** in YAML.
- Secret values are stored in the OS keyring (phase 4); in phase 2 regular secrets (Variable) stay in the file
  because the repo is private — documented as an accepted risk.
- The UI marks `secret` env vars with a lock icon and a hidden input.

## 8. Error handling

- All service methods return `error` → the Wails binding throws it to the frontend.
- The frontend shows a toast + error state in the panel; no silent crashes.
- Git command errors (auth failure, conflict, network) → sync status `error` + the raw message
  from git stderr (so the user can paste it into a terminal).

## 9. Testing

- `internal/store`: YAML read/write roundtrip tests + validation of real examples.
- `internal/engine`: httptest server (interpolation, auth, redirect, timeout).
- `internal/git`: temp repo in `t.TempDir()` (init/commit/push to a local bare remote).
- Frontend: `npm run check`; light tests for critical components (tree, env switcher).
- Final verification per phase: `wails3 dev` + documented manual steps.

## 10. ADR summary

| # | Decision | Alternatives | Rationale |
|---|---|---|---|
| A1 | Git CLI, not go-git | pure go-git | hosting-agnostic auth, familiar to users, no token storage |
| A2 | Execution in Go, not the webview | fetch from SvelteKit | no CORS issues, localhost access, proxy/client certs |
| A3 | Vault = local repo, user picks the path | local DB / cloud | git is the vault, undo = history |
| A4 | Auto-commit + auto-push + manual buttons | all manual | comfortable defaults, still under control |
| A5 | `examples[]` native to OpenCollection | custom format | already in the v1.0.0 spec |
| A6 | HTTP + GraphQL in phase 1 | HTTP only | GraphQL = POST JSON, cheap once the engine exists |
| A7 | SvelteKit SPA (ssr=false) | full SSR | webview = SPA, client-side routing |

## 11. Known limitations

- Requires the `git` CLI on the host (phase 1 desktop).
- gRPC / WebSocket / full OAuth2 = phase 4.
- Mobile: Wails overlay prepared in phase 5; git CLI may be swapped for go-git.

## 12. Recommendations

- **Golden-file tests for interpolation.** A table of `{{var}}` edge cases with
  expected outputs in `internal/engine` — the cheapest regression net for the
  feature every request depends on.
- **Interface over exec.** Wrap git CLI calls behind a small `Gitter` interface so
  go-git can replace it for mobile builds without touching services.
- **Keep git stderr verbatim.** Ring-buffer the last N git outputs and show them in
  the sync-error drawer — users can copy-paste the exact failure into a terminal.
- **Don't serialize run results into the vault.** Responses live in memory only;
  the vault holds requests and examples, nothing else — keeps diffs readable.
