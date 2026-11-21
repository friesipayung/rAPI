# 05 · Code generation ("Generate code")

Postman-style snippet generation: turn a saved request into ready-to-paste
client code for other languages and libraries.

## Need (from research)

- Postman generates snippets for 30+ language/variant combos in a right-pane
  dialog: pick language → read snippet → copy. The useful core for an API
  client's users is much smaller; the long tail (jQuery, XHR, Unirest, R,
  OCaml, …) is rarely used and expensive to maintain.
- The snippet must reflect the *effective* request: method, full URL with
  query params, headers, auth and body.
- Snippets are copied into real projects, so they must be **runnable or one
  obvious step away from runnable**: no pseudo-code, no unescaped strings.

## Scope decisions

**Targets (first cut — 14):**

| Key | Label | Library |
|---|---|---|
| curl | cURL | shell command |
| httpie | HTTPie | shell command |
| http | HTTP | raw request |
| python | Python | requests |
| js-fetch | JavaScript | fetch |
| node-native | Node.js | http module |
| node-axios | Node.js | axios |
| go | Go | net/http |
| java | Java | OkHttp |
| csharp | C# | HttpClient |
| php | PHP | ext-curl |
| ruby | Ruby | Net::HTTP |
| rust | Rust | reqwest |
| swift | Swift | URLSession |

Adding a target = one file + one registry line + one test. The long tail can
be added on demand.

**Variables.** `{{var}}` tokens are kept **verbatim** inside the generated
code strings, and every snippet starts with a comment listing the
placeholders found. Substituting them automatically (e.g. to `os.environ`
reads) would silently invent environment conventions; verbatim + comment is
predictable.

**Auth mapping.**

| rAPI auth | Snippet behavior |
|---|---|
| bearer | `Authorization: Bearer <token>` |
| basic | native basic-auth API of the target (or manual header) |
| apikey (header) | plain header |
| apikey (query) | query parameter |
| digest | manual header + comment (challenge/response can't be precomputed) |
| oauth2 | comment "obtain a token first" + `Authorization: Bearer {{token}}` |
| awsv4 / wsse / ntlm | comment explaining the handshake is signing-based |

**Body mapping.** json/text/xml/html → raw string payload (JSON gets
pretty-printed when parseable); form-urlencoded → native params/dict of the
target; multipart → fields via the target's form API, `@file` entries become
the target's file-upload call.

**Settings.** timeout is emitted where the target has a natural timeout knob;
follow-redirects is emitted only when it differs from the library default.

## Architecture

- `internal/codegen` — pure functions, no wails deps:
  - `Snippet` — intermediate representation (method, raw URL, headers, body…)
    built once from `model.HttpRequest`; emitters never touch the model.
  - one emitter file per language family; each registered in `targets`.
  - `Generate(targetKey, req, settings) (string, error)`; `Targets()` for the UI list.
- `CodegenService` (services3.go) — `Targets()`, `Generate(requestID, target)`; loads
  the request doc from the vault. Read-only: no vault writes.
- UI: new **Code** tab in the request editor: target dropdown + snippet + Copy.

## Testing

Each target has at least one golden-ish test against a canonical request
(POST JSON + bearer + query param) and one against a form/multipart case,
asserting on key substrings (URL, header, body) so refactors don't rot output.
