# Llama Desktop combined validation gate (POSIX)
# Usage: make check           # full (version sync + backend + frontend)
#       make check-version   # version sync check only
#       make check-backend   # backend only
#       make check-frontend  # frontend only
#
# Frontend artifact frontend/dist is a go:embed compile dependency (gitignored).
# If frontend is not yet built locally, run make check-frontend or npm run build first.

.PHONY: check check-version check-backend check-frontend

check: check-version check-backend check-frontend

check-version:
	node -e "const fs = require('fs');const v = [fs.readFileSync('core/VERSION', 'utf8').trim().replace(/^[vV]/, ''), JSON.parse(fs.readFileSync('wails.json', 'utf8')).info.productVersion, JSON.parse(fs.readFileSync('frontend/package.json', 'utf8')).version].map(s => s.trim());if (v[0] !== v[1] || v[0] !== v[2]) { console.error('Version files are out of sync: core/VERSION = ' + v[0] + ', wails.json info.productVersion = ' + v[1] + ', frontend/package.json version = ' + v[2]);process.exit(1); }console.log('Version sync ok: ' + v[0] + ' (core/VERSION, wails.json info.productVersion, frontend/package.json)');"

check-backend:
	go build ./...
	go test ./...
	@test -z "$$(gofmt -l .)" || (gofmt -l .; echo "gofmt: unformatted files found"; exit 1)
	golangci-lint run

check-frontend:
	cd frontend && npm run build
