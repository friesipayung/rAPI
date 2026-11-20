# User guide

This is the practical guide to using rAPI day to day. For architecture and the
data format internals, see [01-architecture.md](01-architecture.md) and
[02-data-format.md](02-data-format.md).

## 1. First launch

rAPI keeps everything in a **git repository of your choice** — the *vault*.
There is no account and no cloud.

1. On first launch rAPI asks for the vault location. Point it at an empty
   folder (rAPI initializes it as a git repo) or an existing rAPI vault.
2. Everything you create is written as OpenCollection YAML under `.rapi/` and
   committed automatically (≈1s debounce). Git history is your undo log.
3. To sync between machines, add a remote (GitHub, GitLab, Gitea, a bare repo
   on a server — anything git speaks) and use the sync controls. Push uses
   whatever credentials your host git already has (SSH key, credential helper).

## 2. Collections, folders, requests

- A **collection** is a folder in the sidebar. Right-click / the sidebar
  actions create collections, folders and requests.
- Every request is a single YAML file: URL, method, headers, body, auth,
  scripts, tests. Open it in the editor via the tree.
- Body types: JSON, text, XML, HTML, form-urlencoded, multipart form.
- Requests support variables anywhere with `{{variable}}` syntax. Unknown
  variables are left verbatim so you can spot typos.

## 3. Environments & variables

Each collection can define environments (local, staging, production, or any
custom name). Environments can *extend* a base environment.

- **Variables** are plain values stored in the collection YAML.
- **Secrets** (API keys, passwords) are stored in the OS keyring — never in
  the YAML or git. Mark a variable as *secret* in the environment editor;
  the YAML keeps only the name and a flag, and rAPI injects the value from
  the keyring at execution time.
- Precedence: collection < folder < environment < pre-request script.

## 4. Authentication

Auth is configured per request or folder (folders/collections can inherit
from their parent):

| Type | Fields | Notes |
|---|---|---|
| Bearer | token | `Authorization: Bearer …` |
| Basic | username, password | |
| Digest | username, password | RFC 2617 challenge/response |
| API key | key, value, in (header/query) | |
| OAuth 2 | grant, token URL, client id/secret, scopes | `client_credentials` and `password` grants; tokens cached in memory only |
| AWS SigV4 | access key, secret key, session token, region, service | signs with `x-amz-date`; all fields accept `{{vars}}` |
| WSSE | username, password | `X-WSSE` UsernameToken with per-call nonce |
| NTLM | username (`DOMAIN\\user`), password | NTLM/Negotiate handshake handled by the transport |

Secrets in auth fields can also be supplied via `{{secretName}}` variables
stored in the keyring.

## 5. Scripts & tests

Each request can carry **pre-request** and **post-response** scripts
(JavaScript, run in the Go backend):

```js
// pre-request
rapi.vars.set('timestamp', Date.now());
console.log('sending request');

// post-response
const data = rapi.response.body;
console.log('status', rapi.response.status);
```

- `rapi.request` — `method`, `url`, `headers`
- `rapi.env.get/set` — active environment variables
- `rapi.vars.get/set` — request-scoped variables (merged into interpolation
  scope for the actual request)
- `rapi.response` — `status`, `statusText`, `headers` (lowercased keys),
  `body` (parsed JSON when possible), `timeMs`
- `console.log` output appears in the response console log panel.

**Tests** are declarative assertions evaluated against the response:

- Expression examples: `res.status`, `res.body.id`, `res.headers['content-type']`
- Operators: `equal`, `notEqual`, `contains`, `matches` (regex),
  `greaterThan`, `lessThan`, `exists`
- Results (pass/fail per assertion) show in the Tests tab of the response.

## 6. Import

The Import button accepts:

- **Postman** collection export v2.1 (JSON)
- **Bruno** collection folder (`.bru` files)
- **Insomnia** export v4 (JSON)

rAPI converts them to OpenCollection YAML in your vault, then runs a
validation pass and reports any problems (per item, with severity).

## 7. Examples

Any successful response can be saved as a **named example** on the request
("Save as example" in the response panel). Examples are stored next to the
request in the vault and are useful for documenting expected responses or
mocking later.

## 8. Settings (per request)

- Timeout (seconds, default 30) and max redirects
- Follow redirects on/off
- HTTP(S) proxy URL
- TLS client certificate (PEM cert + key file paths — the files stay on disk,
  never in the vault)

## 9. Keyboard / UI notes

- The response panel shows Body, Headers, Console log, Tests and saved
  Examples.
- Light theme is the default; dark (deep blue) can be toggled in the header.
- The vault sync indicator shows commit/push state in real time.
