# signsafe-api

SignSafe.io REST API server written in Go.

## Getting Started

```bash
cp .env.example .env
go run ./cmd/server
```

The server listens on port `8080` by default. Health check: `GET /health`

## Project Structure

```
cmd/server/       - entrypoint
internal/
  handler/        - HTTP handlers
  service/        - business logic
  repository/     - database access
  middleware/      - HTTP middleware
  queue/          - message queue integration
  storage/        - object storage integration
  model/          - domain models
migrations/       - SQL migration files
```
