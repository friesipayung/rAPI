# rAPI — Data Format & Vault

## 1. Vault layout

One git repo = one vault containing many collections. The repo path is chosen by
the user during onboarding (not a path inside the app).

```
<vault-path>/
├── .git/
├── .rapi/
│   ├── config.yml                 # vault format version, vault name
│   └── ui-state.yml               # UI preferences: active env per collection, last open tab
├── <collection-slug>/
│   ├── opencollection.yml         # root: info, config, request defaults, docs
│   ├── environments/
│   │   ├── local.yml
│   │   ├── staging.yml
│   │   └── production.yml
│   ├── <folder-slug>/
│   │   ├── folder.yml             # Folder item + folder request defaults
│   │   └── <request-slug>.yml
│   └── <request-slug>.yml
└── <collection-slug-2>/...
```

File naming convention: lowercase slugs (`get-orders.yml`). `opencollection: "1.0.0"`
at the root of every file. `bundled: false` (filesystem layout).

## 2. Example files

### `<collection>/opencollection.yml`

```yaml
opencollection: "1.0.0"
info:
  name: Sample API
  summary: Sample collection
  version: "1.0.0"
config:
  environments:
    - local
    - staging
    - production
request:
  headers:
    - name: Accept
      value: application/json
  variables:
    - name: baseUrl
      value: http://localhost:3000
  auth: inherit
  settings:
    http:
      encodeUrl: true
      timeout: 0
      followRedirects: true
      maxRedirects: 5
docs: |
  Sample rAPI collection.
```

Note: in the filesystem layout, `config.environments` is a list of environment
**names** (rAPI convention for the filesystem layout). Full environment
files live in `environments/<name>.yml`. For `bundled: true` files, environments
are written inline in full.

### `<collection>/environments/staging.yml`

```yaml
name: staging
color: "#f59e0b"
description: Staging server
variables:
  - name: baseUrl
    value: https://staging.example.com
  - name: token
    value: ""
    description: Staging token
extends: local          # optional: inherit variables from another environment
```

### `<collection>/<folder>/folder.yml`

```yaml
info:
  name: Users
  type: folder
  seq: 1
docs: |
  ## Users folder

  Endpoints for user management. All requests here inherit bearer auth
  from the folder defaults.
request:
  auth:
    type: bearer
    token: "{{token}}"
  variables:
    - name: userId
      value: "42"
```

### `<collection>/<folder>/<request>.yml`

```yaml
info:
  name: Get Orders
  type: http
  seq: 1
  description: Fetch the order list
http:
  method: GET
  url: "{{baseUrl}}/orders"
  params:
    - name: limit
      value: "10"
      type: query
  headers:
    - name: Authorization
      value: "Bearer {{token}}"
  auth:
    type: inherit
examples:
  - name: 200 OK
    description:
      content: |
        Returns the full order list for the authenticated user.
        Rate limit: **60 req/min**.
      type: text/markdown
    request:
      url: "https://api.example.com/orders?limit=10"
      method: GET
      headers:
        - name: Authorization
          value: "Bearer eyJhbGciOi..."
      params:
        - name: limit
          value: "10"
          type: query
    response:
      status: 200
      statusText: OK
      headers:
        - name: content-type
          value: application/json
      body:
        type: json
        data: '{"orders":[{"id":1},{"id":2}]}'
  - name: 401 Unauthorized
    request:
      url: "https://api.example.com/orders?limit=10"
      method: GET
      headers:
        - name: Authorization
          value: "Bearer invalid"
      params: []
    response:
      status: 401
      statusText: Unauthorized
      headers:
        - name: content-type
          value: application/json
      body:
        type: json
        data: '{"error":"unauthorized"}'
runtime:
  scripts: []
  assertions:
    - expression: res.status
      operator: equal
      value: "200"
settings:
  encodeUrl: true
  timeout: 0
  followRedirects: true
  maxRedirects: 5
  sendCookies: true
docs: |
  ## Get Orders

  Returns the paginated order list for the authenticated user.

  - `limit` — max 100, default 10
  - Requires the `orders:read` scope
```

`settings.sendCookies` opts a request out of the app cookie jar
(`sendCookies: false`): the request neither sends nor stores cookies. Absent
or `true` means cookies enabled (the default). UI runs persist cookies per
vault outside git; CLI runs never touch the jar — see
[07-runner.md](07-runner.md).

## 3. Sample responses (examples)

- One request → **many** `examples[]` (native `HttpRequest.examples`).
- Each example = `{ name, request, response }`:
  - `request` = snapshot of the request at send time (url/method/headers/params/body) → can be re-sent.
  - `response` = snapshot of the response (status/statusText/headers/body{type,data}).
- UI flow: run request → "Save as example" → name the example → saved into the request file.
- When an example is selected in the UI: the request is shown alongside / can be loaded into the editor.
- Each example can carry its own optional markdown docs via `description`
  (`{content, type: text/markdown}`).

### Body: multipart form with file uploads

Multipart form rows are plain KV pairs; a row with `type: file` uploads the
file at `value` (a filesystem path) as a multipart part. `fileName` is the
display name the UI shows for the row (the file picker sets it to the chosen
file's basename; the part on the wire uses the path's basename). Text rows
keep no `type` at all — old documents without the fields keep loading
unchanged:

```yaml
body:
  type: multipart-form
  form:
    - name: user
      value: ali
    - name: attachment
      type: file
      value: /tmp/attachments/report.pdf
      fileName: report.pdf
```

In the UI a file row is created with the file picker; the path is stored in
the request file like any other value (git-diffable, and `{{vars}}` work in
the path).

## 4. Variables & precedence

- Syntax: `{{name}}` in url, params, headers, body, auth.
- Scope: collection (`request.variables`), folder (`request.variables`), environment
  (`variables`), runtime (script/assertion results — phase 4).
- Precedence: **runtime > environment > folder > collection** (environment wins).
  During a collection run the run scope (script-set variables chained from
  earlier steps) sits on top of all of these — see [07-runner.md](07-runner.md).
- Variable value: string or typed (`{type, data}`). `disabled: true` = ignored.
- Secrets: `SecretVariable` (`secret: true`) with no value in YAML → keyring (phase 4).

## 5. Auth

The `Auth` schema = `inherit` | `basic` | `bearer` | `apikey` | `digest` | `ntlm` |
`wsse` | `oauth1` | `oauth2` | `awsv4`.

Phases 1–2: `inherit`, `basic`, `bearer`, `apikey` (header/query), `digest`.
Phase 4: `oauth2`, `awsv4`, `ntlm`, `wsse`, client certificates.

## 6. Environments

- CRUD per collection; fields: `name`, `color`, `description`, `variables[]`,
  `extends` (inherit other environments), `dotEnvFilePath` (optional).
- `SetActive(collectionID, env)` → written to `.rapi/ui-state.yml`.
- Switching environments = swap precedence + interpolation; the URL preview updates too.

`.rapi/ui-state.yml` (UIState) holds the workspace state that survives
restarts: `activeEnvs` (active environment per collection), `theme`,
`openTabs` (request node ids currently open in editor tabs) and `activeTab`
(the focused one). Cookies are *not* part of the vault — see
[07-runner.md](07-runner.md) for where the cookie jar lives.

## 7. Validation & migration

- `rapi validate` / on save: YAML parser → validate against rAPI structs (superset of the official schema).
- `bundled: true` files are fully validated against the official JSON Schema draft-07.
- Filesystem files: rAPI rules (root `opencollection.yml`, `folder.yml`, request
  file, env file) — the official schema is applied per object (HttpRequest, Environment, Folder).
- The vault format version lives in `.rapi/config.yml`; migrate upward when it changes.

## 8. Import / export

- Import: Postman v2.1 (JSON), Bruno (`.bru` and YAML), Insomnia v4 → OpenCollection.
  Result: folder/request tree + one YAML file per request + environments.
- Export: bundle the tree into a single `opencollection.yml` file (`bundled: true`).
- Converters use the `@opencollection/converters` library (TS) or an equivalent Go
  implementation; final decision in phase 3.

## 9. Documentation (markdown)

Every level of the tree can carry **optional additional documentation as markdown**,
stored natively in the same YAML file — no sidecar files, one request stays one
file, everything diffable in git:

| Level | Field | Native spec support |
|---|---|---|
| Collection root (`opencollection.yml`) | `docs` | `Documentation` (string or `{content, type}`) |
| Folder (`folder.yml`) | `docs` | `Folder.docs` |
| Request (`<request>.yml`) | `docs` | `HttpRequest.docs` |
| Example (`examples[]`) | `description` | `HttpRequestExample.description` = `{content, type: text/markdown}` |

Rules:

- Optional: absent or empty → nothing is rendered; the UI shows a quiet "add docs" affordance.
- Content is CommonMark, rendered in the UI (Docs tab / docs panel) with an
  Edit ↔ Preview toggle.
- Docs are static text — **no `{{var}}` interpolation** inside documentation.
- Markdown is written with YAML block scalars (`docs: |`), so multi-line content
  stays readable and git-friendly.
