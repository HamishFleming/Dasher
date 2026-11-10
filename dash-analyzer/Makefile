.PHONY: run api frontend build test fmt check docker
run: frontend/node_modules/.modules.yaml
	bash scripts/dev.sh
api:
	go run ./cmd/server
frontend: frontend/node_modules/.modules.yaml
	pnpm --dir frontend dev
frontend/node_modules/.modules.yaml: frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml
	pnpm --dir frontend install --frozen-lockfile
build: frontend/node_modules/.modules.yaml
	go build -o bin/dash-analyzer ./cmd/server
	pnpm --dir frontend build
test: frontend/node_modules/.modules.yaml
	go test ./...
	pnpm --dir frontend test
fmt:
	gofmt -w cmd internal pkg
check: fmt test
	go build ./...
	pnpm --dir frontend typecheck
docker:
	docker compose up --build -d
