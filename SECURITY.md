# Security Policy

Ludex is a local-first application, but it can store sensitive data such as
browser cookies, Telegram sessions, source URLs, cached media, and imported
transcripts.

## Reporting A Vulnerability

Please do not publish secrets, cookies, Telegram session files, private group
content, or downloaded attachments in public issues.

If GitHub private vulnerability reporting is enabled for this repository, use
that channel. Otherwise, open a public issue with a minimal description that
does not include sensitive values, and maintainers can coordinate privately if
more detail is needed.

## Sensitive Local Files

Keep these out of git, issue attachments, logs, and release archives:

- `.data/`
- `backend/.data/`
- Docker `/data` volumes
- `.env` and `.env.*`
- Telegram `session.bin` and `pending-code.json`
- Browser cookie exports or Ludex auth-profile database rows
- Cached source media and raw imported source content

## Supported Versions

Ludex is still pre-1.0. Security fixes are expected to land on `main` until a
release branch policy exists.
