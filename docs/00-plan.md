# rAPI — Full Plan

## Goal

Build a desktop API client (Go + SvelteKit + Wails v3) named **rAPI** with:

- Vault = private git repo (user picks the local path; remote optional, hosting-agnostic).
- No account / registration.
- Request format = OpenCollection v1.0.0 (YAML).
- Environment per collection (local / staging / production / custom).
- 1 request can have multiple **sample responses**, and each sample response carries
  its own **sample request** (native via `HttpRequest.examples[]`).

## Non-goals (phase 1)

- Real-time multi-device sync (manual + auto git push/pull is enough).
- gRPC & WebSocket execution engine (later phase).
- Team collaboration / share links / cloud sync.
- Mobile app (architecture prepared for it; mobile build = phase 5).

## Execution principles

- **Phase 0 first**: plan + architecture + design + wireframe, approved by the user, then code.
- Every phase has verifiable exit criteria (green build, app runs, real files).
- YAGNI: features outside the current phase are not built early.
- Public repository: `github.com/friesipayung/rapi` (created in phase 1, pushed manually by the user).

---

## Phase 0 — Docs & repo foundation

**Scope**: local repo, README, plan, architecture, data format, UI design, interactive HTML wireframe.

**Deliverables**:
- `README.md`, `docs/00-plan.md`, `docs/01-architecture.md`, `docs/02-data-format.md`, `docs/03-design.md`
- `docs/wireframes/*.html` (index + 5 pages)
- `git init` + `.gitignore`

**Exit criteria**: documents are consistent, wireframes open in the browser, no placeholders.

**Status**: ✅ done.

---

## Phase 1 — Scaffold + Onboarding + Git sync

**Scope**:
- `wails3 init` (svelte template) → convert the frontend to a SvelteKit SPA (ssr=false, adapter-static).
- Go module `github.com/friesipayung/rapi`, Wails services: `VaultService`, `SyncService`, `TreeService`.
- Onboarding wizard: input local vault path → `git init` (or detect an existing repo) → optional remote → connection test.
- Git engine: shell out to the `git` CLI (init/add/commit/pull/push/status/remote), debounced auto-commit + auto-push, status indicator, conflict detection.
- UI: app shell (sidebar + tab bar + sync status), read-only collection tree from files.

**Exit criteria**:
- `wails3 dev` runs; the wizard can init a vault at the user's chosen path.
- Changing a vault file triggers a commit; if a remote exists → push; `git log` in the repo proves it.
- `npm run check` + `go build .` green.

## Phase 2 — Request editor + HTTP execution

**Scope**:
- Request editor: method, URL, params, headers, auth, body (json/text/form/multipart), settings.
- Env engine: env CRUD, `{{var}}` interpolation with precedence, env switcher.
- HTTP engine in Go: timeout, follow redirects, encodeUrl, auth (inherit/basic/bearer/apikey/digest), response viewer (status/time/size/headers/body).
- Save = write OpenCollection YAML + commit.

**Exit criteria**: can create collection/folder/request, run a public request (e.g. httpbin), view the response, and switching env changes the URL.

## Phase 3 — Sample responses & import

**Scope**:
- Examples: list/edit/delete; "save current response as example"; sample request per example.
- Import converters: Postman collection v2.1, Bruno (`.bru` & YAML), Insomnia v4 → OpenCollection + file placement.
- File validation (loose rAPI schema + official schema for bundled files).

**Exit criteria**: importing a Postman collection produces a valid tree + YAML files; examples are stored natively in `examples[]`.

## Phase 4 — Scripts, assertions, advanced auth, secrets

**Scope**: pre/post scripts (goja), assertions, OAuth2/client certs, `SecretVariable` (values in the OS keyring, not git), proxy.

## Phase 5 — Polish, build, release

**Scope**: Linux packaging (deb/AppImage), CI, mobile overlay (optional), user documentation.

---

## Dependency graph

```
F0 ──> F1 ──> F2 ──> F3 ──> F4 ──> F5
             └──────> F3 (import needs the parser; can run parallel after F1)
```

Parallel: F3 (import) can be worked on alongside F2 once the data model (F1) is stable.
Part of F4 (assertions) can move into F2 if the engine is already stable.

## Risks & mitigations

| Risk | Mitigation |
|---|---|
| Wails v3 + SvelteKit SPA quirks | follow the `wails-v3-sveltekit-go` skill; `frontend/dist` placeholder |
| Git auth varies per host | shell out to the `git` CLI (existing SSH/credential helper), never store tokens |
| Conflict on push | push = `pull --rebase` first; on conflict → surface to UI, no auto-resolve |
| Node 22 vs Wails requirement (Node 24) | use `~/.nvm/versions/node/v24.21.0` via PATH prefix |
| Scope creep | phase 1 non-goals are locked; new feature = new phase |
