# berAPI

API client desktop — meniru Postman / Apidog / Bruno — dengan **git sebagai vault**.
Tanpa akun, tanpa registrasi, tanpa cloud. Semua koleksi disimpan sebagai YAML
OpenCollection di repo git private milikmu, dan git-lah sumber kebenaran.

## Stack

| Lapisan | Teknologi |
|---|---|
| Shell desktop | Wails v3 |
| Backend | Go 1.26+ (engine HTTP, git, vault) |
| Frontend | SvelteKit + Svelte 5 (SPA, adapter-static) |
| Format data | OpenCollection v1.0.0 (YAML) |
| Vault | Repo git lokal (path kamu tentukan) + remote opsional (GitHub / GitLab / Gitea / dst) |

## Prinsip

1. **Git adalah vault.** DB lokal tidak diperlukan; file di repo = state. Git history = undo.
2. **Tanpa akun.** Identitas = kredensial git yang sudah ada di host (SSH key / credential helper).
3. **Format terbuka.** OpenCollection YAML — bisa dibaca, di-diff, dan dipakai tool lain.
4. **Eksekusi di backend.** Request dikirim oleh Go, bukan webview (bebas CORS, bisa localhost).
5. **Environment tetap.** local / staging / production / env bebas per collection.

## Dokumentasi

- [Full plan per fase](docs/00-plan.md)
- [Arsitektur](docs/01-architecture.md)
- [Format data & vault](docs/02-data-format.md)
- [Desain UI](docs/03-design.md)
- [Wireframe interaktif](docs/wireframes/index.html)

## Status

Fase 0 (plan + arsitektur + desain) selesai. Eksekusi fase 1 menyusul setelah spec disetujui.
