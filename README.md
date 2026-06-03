# Ludex

[![CI](https://github.com/BD777/ludex/actions/workflows/ci.yml/badge.svg)](https://github.com/BD777/ludex/actions/workflows/ci.yml)

Local-only game metadata indexer and adapter-backed source browser.

Ludex keeps game metadata, imported source transcripts, cached images, auth
profiles, and Telegram sessions on your own machine. It is intended to be run on
`localhost` or inside a private Docker deployment.

Current scope:

- Go backend with SQLite storage.
- Vue frontend for browsing games, adapters, imports, review items, and tasks.
- Built-in F95zone adapter for forum list browsing, thread import, metadata
  extraction, auth-cookie bridge support, and cached attachments.
- Built-in Telegram adapter for account login, group/channel source selection,
  message browsing/search, generic transcript extraction, and cached media.

## Development

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

## Docker

Build and run a single-container production image:

```sh
docker build -t ludex:local .
docker run --rm \
  -p 8787:8787 \
  -v ludex-data:/data \
  ludex:local
```

Open `http://127.0.0.1:8787`.

With Docker Compose:

```sh
docker compose up --build
```

The container serves both the Vue UI and the Go API on port `8787`. Persistent
data lives in `/data`; the examples above store it in the `ludex-data` Docker
volume.

Optional Telegram API credentials can be supplied through environment variables:

```sh
LUDEX_TELEGRAM_API_ID=123456 \
LUDEX_TELEGRAM_API_HASH=your_api_hash \
docker compose up --build
```

You can also enter Telegram API credentials in the Ludex UI.

Copy `.env.example` to `.env` if you want Docker Compose to pick up local
environment values:

```sh
cp .env.example .env
```

## Browser Bridge

Ludex ships a Chromium/Edge browser extension for syncing supported source auth
profiles into the local service.

- Download from the Ludex `Adapters` page with `Extension ZIP`.
- Direct URL: `http://127.0.0.1:8787/extensions/ludex-browser-bridge.zip`
- The current bridge targets `http://127.0.0.1:8787`, so keep the Docker host
  port mapped to `8787` unless you update the extension.
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
- `LUDEX_PUBLIC_DIR`: optional directory for serving the built frontend from the
  backend, used by the Docker image
- `LUDEX_TELEGRAM_API_ID`: optional Telegram API ID fallback
- `LUDEX_TELEGRAM_API_HASH`: optional Telegram API hash fallback

Legacy `GMB_ADDR` and `GMB_DATA_DIR` names are still accepted.

## Open Source

- License: MIT. See `LICENSE`.
- CI: GitHub Actions runs backend tests, frontend builds, and Docker image
  builds on pushes and pull requests.
- Dependency updates: Dependabot watches Go modules, npm packages, GitHub
  Actions, and Docker base images.
- Contributions: See `CONTRIBUTING.md`.
- Security reporting and sensitive-file guidance: See `SECURITY.md`.
- Contributor behavior expectations: See `CODE_OF_CONDUCT.md`.

## Data And Secrets

Do not publish or commit the Ludex data directory. It may contain:

- SQLite database files.
- Cached source attachments and images.
- Browser auth-cookie profiles.
- Telegram session files and pending login state.

The default local data directory is `.data/`, and the Docker data directory is
`/data`.

## Source-Site And Content Notes

Ludex adapters can connect to third-party websites and Telegram groups using
your own account and local configuration. Make sure your usage respects the
terms, access rules, and content restrictions of each source. Ludex stores
cached media locally for speed; do not redistribute cached source content unless
you have the right to do so.
