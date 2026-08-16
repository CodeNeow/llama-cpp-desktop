# Llama Desktop combined validation gate (POSIX)
# Usage: make check           # full (backend + frontend)
#       make check-backend   # backend only
#       make check-frontend  # frontend only
#
# Frontend artifact frontend/dist is a go:embed compile dependency (gitignored).
# If frontend is not yet built locally, run make check-frontend or npm run build first.

.PHONY: check check-backend check-frontend

check: check-backend check-frontend

check-backend:
	go build ./...
	go test ./...
	@test -z "$$(gofmt -l .)" || (gofmt -l .; echo "gofmt: unformatted files found"; exit 1)
	golangci-lint run

check-frontend:
	cd frontend && npm run build
