# Novella API

Go backend service for Novella.

## Project root

`/home/christian/novella`

## Main program

- Relative path: `./cmd/server/main.go`
- Package entrypoint: `./cmd/server`

## Requirements

- Go `1.22.2`

## Run locally

```bash
go run ./cmd/server
```

Server listens on `PORT` env var, default `8080`.
Database file path uses `DB_PATH`, default `./data/novella.db.json`.

Health check:

```bash
curl http://localhost:8080/health
```

## Build binary

```bash
go build -o ./bin/server ./cmd/server
```

Run built binary:

```bash
./bin/server
```

## Fly.io deploy

`fly.toml` is included and uses internal port `8080` with persistent DB at `/data/novella.db.json`.

1. Install Fly CLI (`flyctl`) and log in.
2. Create a unique app name.
3. Update `app = "..."` in `fly.toml`.
4. Create volume once (per region):

```bash
fly volumes create novella_data --region ord --size 1
```

5. Deploy:

```bash
fly deploy
```
