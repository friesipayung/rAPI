# rAPI — UI Design

## 0. Theme system

- **Light (default)**: white surfaces, soft shadows, a radiant red accent
  (`#9B111E`) reserved for actions and highlights.
- **Dark**: deep-blue glassy surfaces (blur retained), raised red variant for contrast.
- Theme switch lives in the topbar; the choice persists per install (`.rapi/ui-state.yml`).

## 1. Design tokens

### Light (default)

| Token | Value |
|---|---|
| bg base | `#f7f8fa` |
| surface | `#ffffff` (shadow `0 1px 2px rgba(16,24,40,.06)`) |
| surface-2 | `#f1f3f6` |
| border | `#e4e7ec` |
| text primary | `#1b2330` |
| text muted | `#64748b` |
| accent (radiant red) | `#9B111E` |
| accent hover | `#7E0D18` |
| accent soft (tint) | `rgba(155,17,30,.08)` |
| on-accent | `#ffffff` |
| success | `#1a7f37` |
| warning | `#9a6700` |
| danger | `#d1242f` |
| radius | `10px` cards, `8px` controls |
| font | Inter / system-ui; mono for URL & body (JetBrains Mono) |

### Dark (deep blue)

| Token | Value |
|---|---|
| bg base | `#0b1524` |
| surface | `#111f33` with `backdrop-filter: blur(12px)` |
| surface-2 | `#16283f` |
| border | `rgba(125,165,225,.14)` |
| text primary | `#dfe7f2` |
| text muted | `#8ba0bd` |
| accent (raised red) | `#e8394a` |
| accent soft | `rgba(232,57,74,.12)` |
| success | `#3fb950` |
| warning | `#d29922` |
| danger | `#f85149` |

Method color coding (GET/POST/PUT/DELETE/PATCH) uses the same hue in both themes,
with light-mode text colors darkened one step for AA contrast on tint backgrounds.

## 2. Workspace layout

```
┌──────────────────────────────────────────────────────────────────┐
│ Topbar: [rAPI] [vault name ▾] [env switcher ▾] [sync ●] [+] [☀/☾] │
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
- `RequestEditor`: method dropdown + URL bar + tabs (Params/Headers/Auth/Body/Scripts/Tests/Examples/Docs).
- `DocsTab`: optional markdown docs per collection/folder/request/example —
  CommonMark editor with Edit ↔ Preview toggle; empty = quiet "add docs" affordance.
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

The wireframes ship both themes: light is the default, the topbar toggle switches
to the dark-blue night theme, and the choice is remembered (localStorage).

## 8. Design recommendations

1. **Accent vs danger.** Both are red by nature. Rule: the accent is only used for
   *interactive* elements (buttons, active tabs, links, focus); danger always ships
   with an icon and never appears as a default button fill. Prevents "everything
   looks like an error".
2. **Syntax highlighting per theme.** Light: warm-light highlighting tuned for the
   red accent; dark: a blue-based scheme. Ship as two token sets, not libraries.
3. **Motion budget.** 150–200ms ease-out for drawers/tabs/toasts; support
   `prefers-reduced-motion` (no pulses, no slide-ins).
4. **Density toggle.** Comfortable (default) and compact spacing scale, remembered
   in `.rapi/ui-state.yml` — API clients are long-session tools.
5. **Empty states as onboarding.** First-run vault, first collection, first request
   each get a one-card guide with the exact next action.
6. **Theme tokenization from day one.** All colors flow through CSS custom
   properties (light default + dark-blue night); no hard-coded hex in components.
7. **Status pill vocabulary is fixed.** Exactly four sync states and four HTTP
   status color bands (2xx/3xx/4xx/5xx) everywhere in the app.
8. **Markdown docs are static.** Docs render as sanitized CommonMark (no HTML
   passthrough), never interpolate `{{var}}`, and never execute scripts — docs are
   content, not code.
