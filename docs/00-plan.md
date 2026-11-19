# berAPI — Full Plan

## Tujuan

Membangun API client desktop (Go + SvelteKit + Wails v3) bernama **berAPI** dengan:

- Vault = repo git private (user tentukan path lokalinya; remote opsional, hosting-agnostic).
- Tanpa akun / registrasi.
- Format request = OpenCollection v1.0.0 (YAML).
- Environment per collection (local / staging / production / bebas).
- 1 request bisa punya banyak **sample response**, dan tiap sample response membawa
  **request sample**-nya sendiri (native lewat `HttpRequest.examples[]`).

## Non-goals (fase 1)

- Sinkronisasi multi-device real-time (cukup git push/pull manual + auto).
- gRPC & WebSocket execution engine (fase lanjutan).
- Kolaborasi tim / share link / cloud sync.
- Mobile app (arsitektur disiapkan, build mobile = fase 5).

## Prinsip eksekusi

- **Fase 0 dulu**: plan + arsitektur + desain + wireframe, disetujui user, baru kode.
- Setiap fase punya exit criteria yang bisa diverifikasi (build hijau, app jalan, file nyata).
- YAGNI: fitur di luar fase tidak dibangun lebih awal.
- Repositori public: `github.com/friesipayung/berAPI` (dibuat saat fase 1, push manual oleh user).

---

## Fase 0 — Fondasi dokumen & repo

**Scope**: repo lokal, README, plan, arsitektur, format data, desain UI, wireframe HTML interaktif.

**Deliverable**:
- `README.md`, `docs/00-plan.md`, `docs/01-architecture.md`, `docs/02-data-format.md`, `docs/03-design.md`
- `docs/wireframes/*.html` (index + 5 halaman)
- `git init` + `.gitignore`

**Exit criteria**: dokumen konsisten, wireframe bisa dibuka di browser, tidak ada placeholder.

**Status**: ✅ selesai.

---

## Fase 1 — Scaffold + Onboarding + Git sync

**Scope**:
- `wails3 init` (template svelte) → konversi frontend ke SvelteKit SPA (ssr=false, adapter-static).
- Go module `github.com/friesipayung/berapi`, service Wails: `VaultService`, `SyncService`, `TreeService`.
- Onboarding wizard: input path vault lokal → `git init` (atau deteksi repo existing) → opsional remote → tes koneksi.
- Git engine: shell ke `git` CLI (init/add/commit/pull/push/status/remote), debounce auto-commit + auto-push, indikator status, deteksi conflict.
- UI: shell aplikasi (sidebar + tab bar + status sync), tree koleksi read-only dari file.

**Exit criteria**:
- `wails3 dev` jalan; wizard bisa init vault di path pilihan user.
- Mengubah file vault memicu commit; remote tersedia → push; `git log` di repo membuktikannya.
- `npm run check` + `go build .` hijau.

## Fase 2 — Editor request + eksekusi HTTP

**Scope**:
- Request editor: method, URL, params, headers, auth, body (json/text/form/multipart), settings.
- Env engine: CRUD env, `{{var}}` interpolation dengan precedence, env switcher.
- HTTP engine di Go: timeout, follow redirects, encodeUrl, auth (inherit/basic/bearer/apikey/digest), response viewer (status/time/size/headers/body).
- Save = tulis YAML OpenCollection + commit.

**Exit criteria**: bisa create collection/folder/request, jalankan request publik (mis. httpbin), lihat response, ganti env mengubah URL.

## Fase 3 — Sample response & import

**Scope**:
- Examples: list/edit/hapus; "save current response as example"; request sample per example.
- Import converter: Postman collection v2.1, Bruno (`.bru` & YAML), Insomnia v4 → OpenCollection + penempatan file.
- Validasi file (schema longgar berAPI + official schema untuk bundled).

**Exit criteria**: import Postman collection menghasilkan tree + file YAML yang valid; example tersimpan native di `examples[]`.

## Fase 4 — Script, assertion, auth lanjutan, secret

**Scope**: pre/post script (goja), assertions, OAuth2/client cert, `SecretVariable` (nilai di OS keyring, bukan git), proxy.

## Fase 5 — Polish, build, rilis

**Scope**: packaging Linux (deb/AppImage), CI, overlay mobile (opsional), dokumentasi pakai.

---

## Dependency graph

```
F0 ──> F1 ──> F2 ──> F3 ──> F4 ──> F5
             └──────> F3 (import butuh parser, bisa paralel setelah F1)
```

Paralel: F3 (import) bisa dikerjakan bersamaan F2 setelah model data (F1) stabil.
F4 sebagian (assertion) bisa masuk F2 jika engine sudah stabil.

## Risiko & mitigasi

| Risiko | Mitigasi |
|---|---|
| Wails v3 + SvelteKit SPA quirks | ikuti skill `wails-v3-sveltekit-go`; `frontend/dist` placeholder |
| Git auth di host beragam | shell ke `git` CLI (SSH/credential helper existing), jangan simpan token |
| Conflict saat push | push = `pull --rebase` dulu; conflict → surface ke UI, jangan auto-resolve |
| Node 22 vs syarat Wails (Node 24) | pakai `~/.nvm/versions/node/v24.21.0` via PATH prefix |
| cakupan melebar | non-goals fase 1 dilock; fitur baru = fase baru |
