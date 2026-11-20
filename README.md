# rAPI

A desktop API client with **git as the vault**.
No account, no registration, no cloud. All collections are stored as OpenCollection
YAML in your own private git repo, and git is the source of truth.

## Features

- Collections, folders and requests stored as readable, diffable YAML files
- Environments (with inheritance) + OS-keyring-backed secrets
- Auth: Bearer, Basic, Digest, API key, OAuth 2 (client credentials / password),
  AWS SigV4, WSSE, NTLM — plus TLS client certificates
- Pre-request / post-response scripts and declarative response tests
- Import from Postman v2.1, Bruno, and Insomnia v4 exports
- Named response examples, response console log, per-request settings
  (timeout, redirects, proxy)
- Automatic git commits; optional remote sync with the git CLI

## Stack

| Layer | Technology |
|---|---|
| Desktop shell | Wails v3 |
| Backend | Go 1.26+ (HTTP engine, git, vault) |
| Frontend | SvelteKit + Svelte 5 (SPA, adapter-static) |
| Data format | OpenCollection v1.0.0 (YAML) |
| Vault | Local git repo (path of your choice) + optional remote (GitHub / GitLab / Gitea / etc.) |

## Principles

1. **Git is the vault.** No local DB required; files in the repo = state. Git history = undo.
2. **No account.** Identity = git credentials already on the host (SSH key / credential helper).
3. **Open format.** OpenCollection YAML — readable, diffable, and usable by other tools.
4. **Execution in the backend.** Requests are sent by Go, not the webview (no CORS issues, can reach localhost).
5. **Environments stay.** local / staging / production / custom env per collection.
6. **No secrets in git.** Secret variable values and OAuth tokens live in the OS keyring / memory, never in the vault.

## Install

Grab a build from [Releases](https://github.com/friesipayung/rAPI/releases):

- **Debian/Ubuntu (amd64)**: `sudo apt install ./rapi_*.deb` (GTK4 + WebKitGTK 6.0)
- **Linux portable**: `chmod +x rapi-x86_64.AppImage && ./rapi-x86_64.AppImage` (needs `libfuse2`, or run with `--appimage-extract`)
- **macOS**: unzip and open `rapi.app` (unsigned — right-click → Open on first launch, or allow it in System Settings → Privacy & Security)

The git CLI must be installed; the vault is your own git repo.

## Documentation

- [User guide](docs/04-usage.md)
- [Full phase-by-phase plan](docs/00-plan.md)
- [Architecture](docs/01-architecture.md)
- [Data format & vault](docs/02-data-format.md)
- [UI design](docs/03-design.md)
- [Interactive wireframe](docs/wireframes/index.html)

## Building from source

Requirements: Go 1.26+, Node 22+, wails3 CLI (`go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0`),
and on Linux the GTK4 / WebKitGTK 6.0 dev packages.

```sh
wails3 task linux:build          # or darwin:build / windows:build
./bin/rapi
```

## Status

Phases 0–4 complete: spec, core vault/editing, import, runtime (scripts,
tests, auth, secrets, proxy). Phase 5 packaging & docs complete.

## License

TBD
