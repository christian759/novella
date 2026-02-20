# Novella Backend

Go backend for a novel reading/writing app.

## Run

```bash
go run ./cmd/server
```

Server starts on `:8080` by default (`PORT` env var overrides).

## Data Model

- `User`: account for readers/writers.
- `Novel`: metadata and publication status (`draft` or `published`).
- `Chapter`: ordered content units for a novel.
- `Comment`: discussion on a novel or specific chapter.
- `Bookmark`: reader progress per novel.

## Auth Flow

1. Register: `POST /auth/register`
2. Login: `POST /auth/login`
3. Use `Authorization: Bearer <token>` for protected routes.
4. Check current user: `GET /me`

## API Routes

### Auth

- `POST /auth/register`
- `POST /auth/login`
- `GET /me` (auth)

### Novels

- `GET /novels?q=&author_id=&include_drafts=&limit=&offset=`
- `POST /novels` (auth)
- `GET /novels/{novelId}`
- `PATCH /novels/{novelId}` (auth, owner)
- `DELETE /novels/{novelId}` (auth, owner)

### Chapters

- `GET /novels/{novelId}/chapters`
- `POST /novels/{novelId}/chapters` (auth, owner)
- `GET /novels/{novelId}/chapters/{chapterId}`
- `PATCH /novels/{novelId}/chapters/{chapterId}` (auth, owner)
- `DELETE /novels/{novelId}/chapters/{chapterId}` (auth, owner)

### Comments

- `GET /novels/{novelId}/comments?chapter_id=`
- `POST /novels/{novelId}/comments` (auth)

### Bookmarks

- `POST /novels/{novelId}/bookmark` (auth)
- `GET /me/bookmarks` (auth)

## Example cURL

```bash
# register
curl -s http://localhost:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"author1","email":"author1@example.com","password":"secret"}'

# login
curl -s http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"author1@example.com","password":"secret"}'

# create novel (replace TOKEN)
curl -s http://localhost:8080/novels \
  -H 'Authorization: Bearer TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{"title":"Skyfall","description":"Epic fantasy","genre":"Fantasy","status":"draft"}'
```

## Notes

- Storage is in-memory for now (data resets on restart).
- Password hashing uses salted SHA-256 in this scaffold. For production, switch to `bcrypt` or `argon2`.
