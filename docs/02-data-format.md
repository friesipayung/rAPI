# berAPI — Format Data & Vault

## 1. Vault layout

Satu repo git = satu vault berisi banyak collection. Path repo ditentukan user saat
onboarding (bukan path dalam aplikasi).

```
<vault-path>/
├── .git/
├── .berapi/
│   ├── config.yml                 # versi format vault, nama vault
│   └── ui-state.yml               # preferensi UI: active env per collection, tab terakhir
├── <collection-slug>/
│   ├── opencollection.yml         # root: info, config, request defaults, docs
│   ├── environments/
│   │   ├── local.yml
│   │   ├── staging.yml
│   │   └── production.yml
│   ├── <folder-slug>/
│   │   ├── folder.yml             # Folder item + request defaults folder
│   │   └── <request-slug>.yml
│   └── <request-slug>.yml
└── <collection-slug-2>/...
```

Konvensi penamaan file: slug lowercase (`get-orders.yml`). `opencollection: "1.0.0"`
di root file. `bundled: false` (filesystem layout).

## 2. Contoh file

### `<collection>/opencollection.yml`

```yaml
opencollection: "1.0.0"
info:
  name: Sample API
  summary: Contoh koleksi
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
  Koleksi contoh berAPI.
```

Catatan: `config.environments` di filesystem layout = daftar **nama** environment
(konvensi berAPI, mengikuti Bruno YAML layout). File environment lengkap berada di
`environments/<name>.yml`. Untuk file `bundled: true`, env ditulis inline lengkap.

### `<collection>/environments/staging.yml`

```yaml
name: staging
color: "#f59e0b"
description: Server staging
variables:
  - name: baseUrl
    value: https://staging.example.com
  - name: token
    value: ""
    description: Token staging
extends: local          # opsional: warisi variable env lain
```

### `<collection>/<folder>/folder.yml`

```yaml
info:
  name: Users
  type: folder
  seq: 1
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
  description: Ambil daftar order
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
```

## 3. Sample response (examples)

- Satu request → **banyak** `examples[]` (native `HttpRequest.examples`).
- Tiap example = `{ name, request, response }`:
  - `request` = snapshot request saat itu (url/method/headers/params/body) → bisa dikirim ulang.
  - `response` = snapshot response (status/statusText/headers/body{type,data}).
- Alur UI: jalankan request → "Save as example" → nama example → tersimpan di file request.
- Saat memilih example di UI: request ikut ditampilkan/dapat di-load ke editor.

## 4. Variabel & precedence

- Sintaks: `{{name}}` di url, params, headers, body, auth.
- Scope: collection (`request.variables`), folder (`request.variables`), environment
  (`variables`), runtime (hasil script/assertion — fase 4).
- Precedence: **runtime > environment > folder > collection** (env menang).
- Variable value: string atau typed (`{type, data}`). `disabled: true` = diabaikan.
- Secret: `SecretVariable` (`secret: true`) tanpa value di YAML → keyring (fase 4).

## 5. Auth

Schema `Auth` = `inherit` | `basic` | `bearer` | `apikey` | `digest` | `ntlm` |
`wsse` | `oauth1` | `oauth2` | `awsv4`.

Fase 1–2: `inherit`, `basic`, `bearer`, `apikey` (header/query), `digest`.
Fase 4: `oauth2`, `awsv4`, `ntlm`, `wsse`, client certificates.

## 6. Environment

- CRUD per collection; field: `name`, `color`, `description`, `variables[]`,
  `extends` (warisi env lain), `dotEnvFilePath` (opsional).
- `SetActive(collectionID, env)` → ditulis di `.berapi/ui-state.yml`.
- Env switch = ganti precedence + interpolasi; URL preview ikut berubah.

## 7. Validasi & migrasi

- `berapi validate` / saat save: parser YAML → cek struct berAPI (superset schema resmi).
- File `bundled: true` divalidasi penuh terhadap official JSON Schema draft-07.
- File filesystem: aturan berAPI (root `opencollection.yml`, `folder.yml`, request file,
  env file) — official schema dipakai per-objek (HttpRequest, Environment, Folder).
- Versi format vault di `.berapi/config.yml`; migrasi naik versi bila berubah.

## 8. Import / export

- Import: Postman v2.1 (JSON), Bruno (`.bru` dan YAML), Insomnia v4 → OpenCollection.
  Hasil: tree folder/request + file YAML per request + env.
- Export: bundel satu file `opencollection.yml` (`bundled: true`) dari tree.
- Converter pakai pustaka `@opencollection/converters` (TS) atau implementasi Go
  setara; pilihan final di fase 3.
