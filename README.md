# Novella API

Backend API for the Novella mobile app.

## Project layout

- Project root: `/home/christian/novella`
- Main program (relative): `./cmd/server/main.go`
- Main package: `./cmd/server`

## Tech stack

- Go `1.22.2`
- Standard library `net/http` router
- File-backed persistence (JSON DB file)

## Run locally

```bash
go run ./cmd/server
```

Environment variables:

- `PORT` (default: `8080`)
- `DB_PATH` (default: `./data/novella.db.json`)

Health check:

```bash
curl http://localhost:8080/health
```

## Build binary

```bash
go build -o ./bin/server ./cmd/server
./bin/server
```

## Persistence

Data is persisted to the JSON file at `DB_PATH` and survives restarts.

Persisted entities:

- users
- auth sessions/tokens
- novels
- chapters
- comments
- bookmarks

## Base API conventions

- Base URL (local): `http://localhost:8080`
- Content-Type: `application/json`
- Auth header: `Authorization: Bearer <token>`
- Error format:

```json
{ "error": "message" }
```

## Auth flow for mobile app

1. Register with `POST /auth/register` or login with `POST /auth/login`.
2. Store returned bearer token securely on device.
3. Send token on protected routes.
4. Use `GET /me` to restore user profile on app startup.

## Data models (response shapes)

### User

```json
{
  "id": 1,
  "username": "alice",
  "email": "alice@example.com",
  "created_at": "2026-02-20T12:00:00Z"
}
```

### Novel

```json
{
  "id": 1,
  "author_id": 1,
  "title": "Skybound",
  "description": "A serialized fantasy",
  "genre": "Fantasy",
  "status": "draft",
  "created_at": "2026-02-20T12:00:00Z",
  "updated_at": "2026-02-20T12:00:00Z"
}
```

`status` values:

- `draft`
- `published`

### Chapter

```json
{
  "id": 1,
  "novel_id": 1,
  "title": "Chapter 1",
  "content": "....",
  "position": 1,
  "created_at": "2026-02-20T12:00:00Z",
  "updated_at": "2026-02-20T12:00:00Z"
}
```

### Comment

```json
{
  "id": 1,
  "novel_id": 1,
  "chapter_id": 1,
  "user_id": 2,
  "body": "Great chapter",
  "created_at": "2026-02-20T12:00:00Z"
}
```

### Bookmark

```json
{
  "user_id": 2,
  "novel_id": 1,
  "chapter_id": 1,
  "updated_at": "2026-02-20T12:00:00Z",
  "chapter_position": 1
}
```

## Endpoint reference

### Health

- `GET /health`
- Auth: no
- `200`: `{ "status": "ok" }`

### Auth

- `POST /auth/register`
- Auth: no
- Body:

```json
{
  "username": "alice",
  "email": "alice@example.com",
  "password": "secret"
}
```

- `201` response:

```json
{
  "user": { "...": "User object" },
  "token": "bearer-token"
}
```

- Errors: `400`, `409` (email/username exists)

- `POST /auth/login`
- Auth: no
- Body:

```json
{
  "email": "alice@example.com",
  "password": "secret"
}
```

- `200` response:

```json
{
  "user": { "...": "User object" },
  "token": "bearer-token"
}
```

- Errors: `400`, `401`

### Current user

- `GET /me`
- Auth: yes
- `200`: `User`
- Errors: `401`

- `GET /me/bookmarks`
- Auth: yes
- `200`: `Bookmark[]`
- Errors: `401`

### Novels

- `GET /novels`
- Auth: optional
- Query params:
  - `q` (searches title/description/genre)
  - `author_id` (int64)
  - `include_drafts=true` (still only returns drafts you own)
  - `limit` (int)
  - `offset` (int)
- `200`: `Novel[]` (sorted by `updated_at` desc)

- `POST /novels`
- Auth: yes
- Body:

```json
{
  "title": "Skybound",
  "description": "A serialized fantasy",
  "genre": "Fantasy",
  "status": "draft"
}
```

- `201`: `Novel`
- Errors: `400`, `401`

- `GET /novels/{novelId}`
- Auth: optional
- `200`: `Novel`
- Errors: `403` (draft not owned), `404`

- `PATCH /novels/{novelId}`
- Auth: yes (author only)
- Body (partial):

```json
{
  "title": "New title",
  "description": "Updated",
  "genre": "Sci-Fi",
  "status": "published"
}
```

- `200`: `Novel`
- Errors: `400`, `403`, `404`

- `DELETE /novels/{novelId}`
- Auth: yes (author only)
- `204`
- Errors: `403`, `404`

### Chapters

- `GET /novels/{novelId}/chapters`
- Auth: optional
- `200`: `Chapter[]` (sorted by `position` asc)
- Errors: `403`, `404`

- `POST /novels/{novelId}/chapters`
- Auth: yes (author only)
- Body:

```json
{
  "title": "Chapter 1",
  "content": "Text",
  "position": 1
}
```

- `201`: `Chapter`
- Errors: `400`, `403`, `404`

- `GET /novels/{novelId}/chapters/{chapterId}`
- Auth: optional
- `200`: `Chapter`
- Errors: `403`, `404`

- `PATCH /novels/{novelId}/chapters/{chapterId}`
- Auth: yes (author only)
- Body (partial):

```json
{
  "title": "Retitled",
  "content": "Updated",
  "position": 2
}
```

- `200`: `Chapter`
- Errors: `400`, `403`, `404`

- `DELETE /novels/{novelId}/chapters/{chapterId}`
- Auth: yes (author only)
- `204`
- Errors: `403`, `404`

### Comments

- `GET /novels/{novelId}/comments`
- Auth: optional
- Optional query param: `chapter_id`
- `200`: `Comment[]` (created time ascending)
- Errors: `400`, `403`, `404`

- `POST /novels/{novelId}/comments`
- Auth: yes
- Body:

```json
{
  "body": "Great chapter",
  "chapter_id": 1
}
```

- `201`: `Comment`
- Errors: `400`, `403`, `404`

### Bookmarks

- `POST /novels/{novelId}/bookmark`
- Auth: yes
- Body:

```json
{
  "chapter_id": 1
}
```

- `200`: `Bookmark`
- Upsert behavior: same user + novel updates existing bookmark.
- Errors: `400`, `403`, `404`

## Visibility and authorization rules

- Draft novels are visible only to the author.
- Published novels are public.
- Novel/chapter create/update/delete requires author ownership.
- Comments require auth to create.
- Bookmark create/update requires auth.
- Invalid/missing bearer token on protected routes returns `401`.

## Mobile integration notes

- Persist token securely (Keychain/Keystore).
- On app launch: call `GET /me` with token; if `401`, force re-login.
- Store IDs as 64-bit integers.
- Dates are RFC3339 UTC strings.
- `PATCH` supports partial updates.
- Unknown JSON fields are rejected (`400`), so send only documented fields.
- List endpoints return plain arrays (no pagination metadata object).

## Fly.io deploy

`fly.toml` is configured for this service on port `8080`, with persisted DB at `/data/novella.db.json`.

1. Install Fly CLI and login.
2. Set unique app name in `fly.toml` (`app = "..."`).
3. Create volume once (per region):

```bash
fly volumes create novella_data --region ord --size 1
```

4. Deploy:

```bash
fly deploy
```
