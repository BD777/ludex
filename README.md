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

Ludex ships a Chromium/Edge browser extension for syncing supported source auth
profiles into the local service.

- Download from the Ludex `Adapters` page with `Extension ZIP`.
- Direct URL: `http://127.0.0.1:8787/extensions/ludex-browser-bridge.zip`
- The extension uses `chrome.cookies` so it can read HttpOnly cookies such as
  F95zone `xf_user` and `xf_session`.
- Cookie values are persisted locally but are not returned by the status API.
  Username and cookie metadata such as names and expiry are exposed for status.
- Chrome and Edge require local extensions to be loaded manually through
  Developer mode / Load unpacked unless the extension is published through the
  browser's extension store.

## Environment

- `LUDEX_ADDR`: backend listen address, default `127.0.0.1:8787`
- `LUDEX_DATA_DIR`: local data directory, default `.data`

Legacy `GMB_ADDR` and `GMB_DATA_DIR` names are still accepted.
