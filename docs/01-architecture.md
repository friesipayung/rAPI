# berAPI — Arsitektur

## 1. Gambaran

Wails v3 membundel backend Go dan frontend SvelteKit ke satu binary desktop.
Frontend **tidak** mengirim request sendiri (webview → CORS, rendah akses localhost);
semua eksekusi jaringan dan git dilakukan di backend Go.

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
          <vault-path>/ (repo git)          remote (GitHub/GitLab/Gitea/dst)
```

## 2. Batasan (boundaries)

| Unit | Tanggung jawab | Bergantung pada |
|---|---|---|
| `internal/model` | Struct Go untuk OpenCollection + hasil run | hanya YAML lib |
| `internal/store` | Load/save/validasi file vault, tree koleksi | `model` |
| `internal/git` | Semua operasi git (shell CLI) | `os/exec` |
| `internal/engine` | Eksekusi HTTP, interpolasi, auth | `model` |
| `internal/vault` | Lifecycle vault, remote, status sync | `store`, `git` |
| `internal/sync` | Debounce auto-commit/push, event status | `git` |
| service structs | API yang diekspos Wails ke frontend | di atas |

Aturan: service tidak berisi logika bisnis; delegate ke `internal/`. Frontend hanya
memanggil method binding, tidak membaca file langsung.

## 3. Service Wails (binding)

| Service | Method kunci |
|---|---|
| `VaultService` | `Init(path)`, `Open(path)`, `Status()`, `AddRemote(url)`, `ListRemotes()`, `RemoveRemote(name)` |
| `SyncService` | `Status()`, `CommitNow(msg)`, `PushNow()`, `PullNow()` + event `sync-status` |
| `TreeService` | `ListCollections()`, `GetTree(id)`, `CreateCollection`, `RenameCollection`, `DeleteCollection`, `CreateFolder`, `MoveItem`, `Reorder` |
| `RequestService` | `Get(id)`, `Save(id)`, `Create(collectionID, folder, req)`, `Delete(id)`, `SetActiveRequest(id)` |
| `EnvService` | `List(collectionID)`, `Create`, `Save`, `Delete`, `SetActive(collectionID, env)` |
| `RunService` | `Execute(collectionID, requestID, envName)` → `RunResult` |
| `ExampleService` | `List(requestID)`, `Save(requestID, example)`, `Delete(requestID, name)`, `SaveResponseAsExample(requestID, name)` |

`RunResult`: `{ status, statusText, headers[], body{type,data}, timeMs, sizeBytes, requestSnapshot }`.

## 4. Alur data

**Menyimpan request**
```
editor → RequestService.Save
       → store.WriteYAML(<vault>/<col>/<folder>/<req>.yml)
       → sync.ScheduleCommit (debounce 800ms)
       → sync.SchedulePush   (debounce 3s, bila remote ada)
       → event sync-status → UI indicator
```

**Menjalankan request**
```
RunService.Execute(id, env)
  → store load request YAML + env YAML + defaults (collection/folder)
  → engine.interpolate ({{var}}) dengan precedence
  → engine.BuildRequest (method/url/params/headers/body/auth)
  → http.Client.Do (timeout, redirect policy, encodeUrl)
  → RunResult → UI
  → (opsional) ExampleService.SaveResponseAsExample
```

**Onboarding**
```
wizard → VaultService.Init(path)
  → bila folder kosong: git init + tulis .berapi/config.yml + koleksi contoh
  → bila sudah repo: deteksi, tanya remote (opsional)
  → tes: git status + (bila ada remote) ls-remote
```

## 5. Git engine

**Keputusan: shell ke `git` CLI**, bukan library murni (go-git).

Alasan: hosting-agnostic (GitHub/GitLab/Gitea/self-hosted) dan auth memakai
kredensial host yang sudah ada (SSH key, credential helper, token di remote URL).
berAPI **tidak menyimpan token**. Tradeoff: binary desktop butuh `git` terpasang
(di Linux/macOS/Windows umumnya ada); mobile build nanti bisa pindah go-git.

Operasi: `init`, `add -A`, `commit -m`, `pull --rebase`, `push`, `status --porcelain`,
`remote add/list/remove`, `ls-remote` (tes koneksi), `rev-parse`.

**Sync policy**
- Auto-commit lokal: debounce 800ms setelah perubahan file.
- Auto-push: debounce 3s, hanya jika remote ada; push = `pull --rebase` lalu `push`.
- Tombol manual: commit sekarang, push sekarang, pull sekarang.
- Conflict: berhenti, set status `conflict`, tampilkan file konflik di UI, **tidak**
  auto-resolve. User resolve manual (git CLI) lalu klik pull.
- Environment secret: nilai `SecretVariable` tidak masuk commit (lihat §7).

## 6. HTTP engine

- `net/http` client per-request; timeout dari `settings.timeout` (0 = default 30s).
- `followRedirects` + `maxRedirects` → custom `CheckRedirect`.
- `encodeUrl`: escape path/param sebelum request.
- Auth: `inherit` (dari folder/collection defaults), `basic`, `bearer`, `apikey`
  (header/query), `digest`. OAuth2/AWSv4/NTLM/WSSE = fase 4.
- Body: `json`, `text`, `xml`, `html`, `binary`, `form-urlencoded`, `multipart-form`,
  `file`.
- Interpolasi `{{var}}` di URL, params, headers, body, auth values.
- Precedence variabel: **runtime > environment > folder > collection**.
- Request snapshot dikembalikan agar bisa langsung dijadikan `request` pada example.

## 7. Secret

- `SecretVariable` (schema OpenCollection) = variabel dengan flag `secret: true`, **tanpa value** di YAML.
- Nilai secret disimpan di OS keyring (fase 4); fase 2 secret biasa (Variable) ikut di file
  karena repo private — documented sebagai risiko yang diterima.
- UI menandai env var `secret` dengan ikon gembok dan input tersembunyi.

## 8. Error handling

- Semua service method mengembalikan `error` → binding Wails melempar ke frontend.
- Frontend menampilkan toast + state error di panel; tidak ada crash senyap.
- Git command error (auth gagal, conflict, network) → status sync `error` + pesan asli
  dari stderr git (agar user bisa copas ke terminal).

## 9. Testing

- `internal/store`: test baca/tulis YAML roundtrip + validasi contoh nyata.
- `internal/engine`: httptest server (interpolasi, auth, redirect, timeout).
- `internal/git`: repo temp di `t.TempDir()` (init/commit/push ke bare remote lokal).
- Frontend: `npm run check`; komponen kritis (tree, env switcher) test ringan.
- Verifikasi akhir tiap fase: `wails3 dev` + langkah manual yang didokumentasikan.

## 10. ADR ringkas

| # | Keputusan | Alternatif | Alasan |
|---|---|---|---|
| A1 | Git CLI, bukan go-git | go-git murni | auth hosting-agnostic, dikenal user, tanpa simpan token |
| A2 | Eksekusi di Go, bukan webview | fetch dari SvelteKit | bebas CORS, akses localhost, proxy/client cert |
| A3 | Vault = repo lokal, user tentukan path | DB lokal / cloud | git adalah vault, undo = history |
| A4 | Auto-commit + auto-push + tombol manual | manual semua | default nyaman, tetap terkontrol |
| A5 | `examples[]` native OpenCollection | format sendiri | sudah ada di spec v1.0.0 |
| A6 | HTTP + GraphQL di fase 1 | HTTP saja | GraphQL = POST JSON, murah setelah engine ada |
| A7 | SvelteKit SPA (ssr=false) | SSR penuh | webview = SPA, routing client-side |

## 11. Batasan yang diketahui

- Butuh `git` CLI di host (fase 1 desktop).
- gRPC / WebSocket / OAuth2 penuh = fase 4.
- Mobile: overlay Wails disiapkan di fase 5, git CLI mungkin diganti go-git.
