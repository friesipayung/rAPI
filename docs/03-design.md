# rAPI — UI Design

Visual language: dark glassy surfaces — stat cards, drawers, tabs, search,
toasts. Mobile responsiveness is a requirement, not a final polish.

## 1. Design tokens

| Token | Value |
|---|---|
| bg base | `#0b0e14` |
| surface | `rgba(255,255,255,0.04)` with `backdrop-filter: blur(12px)` |
| border | `rgba(255,255,255,0.08)` |
| text primary | `#e6e9f0` |
| text muted | `#8b93a7` |
| accent | `#6ea8fe` (blue) |
| success | `#3fb950` |
| warning | `#d29922` |
| danger | `#f85149` |
| radius | `10px` cards, `8px` controls |
| font | Inter / system-ui; mono for URL & body (JetBrains Mono) |

## 2. Workspace layout

```
┌──────────────────────────────────────────────────────────────────┐
│ Topbar: [rAPI] [vault name ▾] [env switcher ▾] [sync ●] [+]       │
├───────────────┬──────────────────────────────────┬───────────────┤
│ Sidebar       │ Tab bar request                   │ Response      │
│ ├ Collections │ ┌──────────────────────────────┐ │ ├ status/time │
│ │  tree       │ │ Method [GET ▾] [URL        ] │ │ ├ tabs body/  │
│ │  (search)   │ ├──────────────────────────────┤ │ │  headers/   │
│ ├ Environments │ │ Params|Headers|Auth|Body|   │ │ │  examples   │
│ ├ Examples    │ │ Scripts|Tests|Examples       │ │ └─────────────│
│ └ Sync status │ └──────────────────────────────┘ │               │
└───────────────┴──────────────────────────────────┴───────────────┘
```

- Sidebar is 260px wide, collapsible; on narrow screens it becomes a drawer.
- The tab bar shows open requests; drag to reorder (phase 2).
- The response panel is ~55% of the height; resizable; on mobile it stacks below the editor.

## 3. Pages (wireframes)

1. **setup** — 3-step wizard: local vault path → remote (optional) → test & init.
2. **workspace** — the main screen (layout above).
3. **environments** — drawer/panel for environment CRUD + variable editor (with secret badge).
4. **import** — pick a source (Postman/Bruno/Insomnia), preview the tree, import.
5. **settings** — vault (path, remote, auto-sync on/off), appearance, about.

## 4. Core components

- `CollectionTree`: folder/request tree, search, drag-and-drop move, context menu
  (rename/duplicate/delete/new folder/new request).
- `EnvSwitcher`: environment pill in the topbar; switching env → immediate interpolation.
- `RequestEditor`: method dropdown + URL bar + tabs (Params/Headers/Auth/Body/Scripts/Tests/Examples).
- `ResponseViewer`: color-coded status code, time, size; Body/Headers/Examples tabs.
- `ExampleCard`: example list; click → view request+response; "Load request" / "Send".
- `SyncStatus`: `synced` / `syncing…` / `error` / `conflict` indicator + push/pull buttons.
- `Toast`: success/failure notifications (bottom right), auto-dismiss.
- `StatCard`: summary (number of collections, requests, environments, last sync).

## 5. States & empty states

| State | Treatment |
|---|---|
| loading | skeleton in tree/editor |
| empty vault | CTA "create your first collection" |
| sync error | red banner + details button + git stderr message |
| conflict | yellow banner + file list, resolve instructions |
| failed request | the response panel shows the error (not empty) |

## 6. Accessibility & mobile

- Text contrast ≥ AA; visible focus; every control has a label.
- Keyboard: Ctrl+Enter = send request, Ctrl+S = save, Ctrl+P = command palette (phase 2).
- Mobile: sidebar → drawer, response stacks, editor stays single-column; touch targets ≥ 40px.
- UI language is English by default (standard for API client tooling).

## 7. Wireframes

`docs/wireframes/` folder:

- `index.html` — nav + design token summary (open this first)
- `setup.html`
- `workspace.html`
- `environments.html`
- `import.html`
- `settings.html`
