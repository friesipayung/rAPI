# berAPI — Desain UI

Referensi visual: **TeamoRouter ops console** — stat cards, drawer, tabs, search,
toasts, dark glassy theme. Mobile responsive = kebutuhan, bukan polesan akhir.

## 1. Design token

| Token | Nilai |
|---|---|
| bg base | `#0b0e14` |
| surface | `rgba(255,255,255,0.04)` dengan `backdrop-filter: blur(12px)` |
| border | `rgba(255,255,255,0.08)` |
| text primary | `#e6e9f0` |
| text muted | `#8b93a7` |
| accent | `#6ea8fe` (biru) |
| success | `#3fb950` |
| warning | `#d29922` |
| danger | `#f85149` |
| radius | `10px` card, `8px` kontrol |
| font | Inter / system-ui; mono untuk URL & body (JetBrains Mono) |

## 2. Layout workspace

```
┌──────────────────────────────────────────────────────────────────┐
│ Topbar: [berAPI] [vault name ▾] [env switcher ▾] [sync ●] [+]     │
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

- Sidebar lebar 260px, collapsible; di layar sempit jadi drawer.
- Tab bar menampilkan request terbuka; drag untuk reorder (fase 2).
- Response panel tinggi ~55%; bisa di-resize; di mobile menumpuk di bawah editor.

## 3. Halaman (wireframe)

1. **setup** — wizard 3 langkah: path vault lokal → remote (opsional) → tes & init.
2. **workspace** — layar utama (layout di atas).
3. **environments** — drawer/panel CRUD env + editor variabel (dengan badge secret).
4. **import** — pilih source (Postman/Bruno/Insomnia), preview tree, import.
5. **settings** — vault (path, remote, auto-sync on/off), tampilan, tentang.

## 4. Komponen inti

- `CollectionTree`: tree folder/request, search, drag-drop move, context menu
  (rename/duplicate/delete/new folder/new request).
- `EnvSwitcher`: pill env di topbar, ganti env → interpolasi langsung.
- `RequestEditor`: method dropdown + URL bar + tabs (Params/Headers/Auth/Body/Scripts/Tests/Examples).
- `ResponseViewer`: status code berwarna, time, size; tabs Body/Headers/Examples.
- `ExampleCard`: list example; klik → lihat request+response; "Load request" / "Send".
- `SyncStatus`: indikator `synced` / `syncing…` / `error` / `conflict` + tombol push/pull.
- `Toast`: notifikasi sukses/gagal (bawah kanan), auto-dismiss.
- `StatCard`: ringkasan (jumlah collection, request, env, sync terakhir).

## 5. State & empty state

| State | Perlakuan |
|---|---|
| loading | skeleton di tree/editor |
| empty vault | CTA "buat koleksi pertama" |
| sync error | banner merah + tombol detail + pesan stderr git |
| conflict | banner kuning + daftar file, instruksi resolve |
| request gagal | response panel menampilkan error (bukan kosong) |

## 6. Aksesibilitas & mobile

- Kontras teks ≥ AA; fokus visible; semua kontrol punya label.
- Keyboard: Ctrl+Enter = kirim request, Ctrl+S = simpan, Ctrl+P = command palette (fase 2).
- Mobile: sidebar → drawer, response menumpuk, editor tetap satu kolom; target sentuh ≥ 40px.
- Bahasa UI: Inggris (standar tooling API client).

## 7. Wireframe

Folder `docs/wireframes/`:

- `index.html` — nav + ringkasan design token (buka ini dulu)
- `setup.html`
- `workspace.html`
- `environments.html`
- `import.html`
- `settings.html`
