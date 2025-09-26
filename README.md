# EndlessWiki

EndlessWiki is a tiny Go service that renders AI-generated Wikipedia-style pages on demand. Pages are persisted in MySQL, and every internal link triggers generation of a new page the first time it is visited.

## High-level flow
1. Normalize the requested slug (case fold, replace spaces with underscores, strip unsafe characters).
2. Look for an existing row in the `pages` table.
3. If found, render the stored HTML.
4. If missing, call Groq to synthesize page content, persist the new row, then render.

## Data model
`pages` table:
- `slug` (PK, varchar) — normalized slug.
- `content` (MEDIUMTEXT) — rendered HTML for the requested slug.
- `created_at` (TIMESTAMP) — default `CURRENT_TIMESTAMP`.

Bootstrap SQL lives in `db/migrations/001_create_pages.sql`.

## Page generation
- Prompt Groq (initial target: `moonshotai/kimi-k2-instruct-0905`) with the slug and instructions to emit HTML. The special `main_page` slug renders a handcrafted EndlessWiki overview instead of calling the model. New slugs are only minted when navigated from an existing page that explicitly links to them.
- Output contains a `<h1>` heading and a `<div class="endlesswiki-body">` wrapping the body.
- Prompt nudges the model to include 3–6 internal wiki links using `<a href="/wiki/...">` anchors.
- If `GROQ_API_KEY` is missing, a deterministic stub generator returns placeholder content for local development.
- A lightweight search endpoint (`/search?q=`) surfaces previously generated pages via a simple MySQL `LIKE` query.
- A constellation exporter (`go run ./cmd/constellation`) snapshots the wiki link graph into `static/constellation.json` for visualisation.

## Running locally
```bash
# set up a MySQL instance and export a DSN the Go driver understands
export MYSQL_DSN="user:pass@tcp(127.0.0.1:3306)/endlesswiki?parseTime=true"
export GROQ_API_KEY="sk_your_groq_key"  # optional; stub content without it
export PORT=8080

# run the server
GOCACHE=$(pwd)/.gocache go run ./cmd/endlesswiki
```

Open `http://localhost:8080/wiki/main_page` (or hit `/`, which redirects there) and follow internal links to generate pages. The chrome exposes search, random (`/random`), most-recent (`/recent`), and the constellation map (`/constellation`) once a snapshot has been generated.

### Constellation exporter

```bash
# produce static/constellation.json
scripts/constellation.sh

# optional: choose a different destination
scripts/constellation.sh -out /tmp/constellation.json
```

Run the exporter before building/deploying to refresh `static/constellation.json`. The `/constellation` page serves `static/constellation.html`, which visualises the generated snapshot directly in the browser.

## Railway deployment
- Railway typically exposes `PORT` automatically.
- Set `DATABASE_URL` to Railway's MySQL connection string (the loader accepts both driver DSNs and `mysql://` URLs) and store `GROQ_API_KEY` as a secret.
- Use `go build ./cmd/endlesswiki` for deployment or rely on Railway’s Go buildpack.
- Make sure migrations run once — e.g. via a Railway job running the SQL in `db/migrations/001_create_pages.sql`.

## Error handling & observability
- `404` for invalid slugs, `500` for DB/Groq failures.
- JSON logging can be layered in later; currently plain-text logs capture key errors.
- TODO: metrics endpoint or structured logging for production visibility.

## Future work
- Rate limiting per IP to prevent abuse.
- Cache Groq responses across instances (e.g. Redis-backed singleflight) and track generation latency metrics.
- Serve shared CSS and assets via static file handler.
