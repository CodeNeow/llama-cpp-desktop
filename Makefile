# Llama GUI 组合验证门（POSIX）
# 用法：make check           # 全量（后端 + 前端）
#       make check-backend  # 仅后端
#       make check-frontend # 仅前端
#
# 前端产物 frontend/dist 是 go:embed 的编译依赖（已 gitignore）。
# 本机前端未构建时，先执行 make check-frontend 或 npm run build。

.PHONY: check check-backend check-frontend

check: check-backend check-frontend

check-backend:
	go build ./...
	go test ./...
	@test -z "$$(gofmt -l .)" || (gofmt -l .; echo "gofmt: 存在未格式化文件"; exit 1)
	golangci-lint run

check-frontend:
	cd frontend && npm run build
