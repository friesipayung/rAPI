# rAPI

A desktop API client with **git as the vault**.
No account, no registration, no cloud. All collections are stored as OpenCollection
YAML in your own private git repo, and git is the source of truth.

## Branding

*rAPI* — **api** is Indonesian for *fire*. The identity: a radiant red accent
(`#9B111E`) on a calm light theme by default; dark mode shifts the base to deep
blue. The fire lives in the accent, nowhere else.

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

## Documentation

- [Full phase-by-phase plan](docs/00-plan.md)
- [Architecture](docs/01-architecture.md)
- [Data format & vault](docs/02-data-format.md)
- [UI design](docs/03-design.md)
- [Interactive wireframe](docs/wireframes/index.html)

## Status

Phase 0 (plan + architecture + design) is complete. Phase 1 execution follows once the spec is approved.
