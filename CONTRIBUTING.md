# Contributing

Thanks for helping improve Ludex.

## Local Setup

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

## Checks

Run the same checks used by CI:

```sh
make test
```

This runs `go test ./...` in `backend/` and `npm run build` in `frontend/`.

## Adapter Work

- Keep adapters explicit and built-in unless there is a clear plugin boundary.
- Do not commit sample data that contains cookies, Telegram sessions, private
  group content, downloaded media, or personal account identifiers.
- Prefer parser fixtures that are minimal, synthetic, and safe to publish.
- Document source-specific limitations in the adapter manifest and README.

## Pull Requests

- Keep changes scoped to one behavior or feature.
- Include focused tests when changing parsing, storage, deletion, task retry, or
  media caching behavior.
- Mention any source-site assumptions that could change over time.
