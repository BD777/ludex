# Ludex

Local-only game metadata indexer and adapter-backed source browser.

First version:

- Go backend with SQLite storage.
- Vue frontend for browsing games, built-in adapters, and imported source items.
- Built-in F95zone transcript adapter for turning forum thread HTML into structured metadata.
- URL fetch with optional per-import proxy, plus raw HTML paste fallback.

## Run

Backend:

```sh
cd backend
go run ./cmd/server
```

Frontend:

```sh
cd frontend
npm install
npm run dev
```

Open the Vite URL, usually `http://localhost:5173`.

## Browser Bridge

Ludex ships a Tampermonkey userscript for importing the currently loaded
logged-in F95zone thread HTML into the local service.

- Install from the Ludex `Adapters` page with `Install userscript`.
- Direct URL: `http://127.0.0.1:8787/userscripts/f95zone.user.js`
- The script posts the current page URL and HTML to `/api/import/f95zone`.

## Environment

- `LUDEX_ADDR`: backend listen address, default `127.0.0.1:8787`
- `LUDEX_DATA_DIR`: local data directory, default `.data`

Legacy `GMB_ADDR` and `GMB_DATA_DIR` names are still accepted.
