# Infiniwiki

Infiniwiki is a tiny Go service that renders AI-generated Wikipedia-style pages on demand. Pages are persisted in MySQL, and every internal link triggers generation of a new page the first time it is visited.

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

## Page generation
- Prompt Groq (initial target: `llama3-70b-8192`) with the slug and minimal context/instructions to emit HTML.
- Require output to include a `<h1>` with the title and a `<div class="infiniwiki-body">` wrapping the article body.
- Ask for 3-6 internal wiki links pointing to plausible future slugs (simple `<a href="/wiki/...">` anchors).
- Enforce deterministic formatting via prompt instructions (no scripts, inline styles only where needed).

## Railway deployment considerations
- Environment variables provided via Railway: `DATABASE_URL` (MySQL DSN) and `PORT`.
- Additional secrets: `GROQ_API_KEY`.
- Service listens on `$PORT` with fallback to `8080` locally.

## Error handling & observability
- Serve `404` for invalid slugs, `500` for DB or Groq failures.
- Log structured JSON lines, capturing request slug, generation latency, and Groq usage.
- TODO: add metrics endpoint or integrate with Railway logging in future iteration.

## Future work
- Add auth rate limits to avoid abuse.
- Cache Groq responses or deduplicate concurrent generations across instances.
- Explore static asset hosting for shared CSS mimicking Wikipedia.
