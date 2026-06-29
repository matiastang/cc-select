.PHONY: all build frontend test vet clean install e2e

# 默认目标：构建含最新前端的二进制。
all: frontend build

# 构建前端（产物输出到 internal/web/assets，被 Go embed）。
frontend:
	cd internal/frontend && npm install && npm run build

# 构建 cc-select 二进制到 ./bin/cc-select。
build:
	mkdir -p bin
	CGO_ENABLED=0 go build -ldflags "-X github.com/cc-select/cc-select/internal/version.Version=$$(git describe --tags --always --dirty 2>/dev/null || echo dev)" -o bin/cc-select .

# 快速开发构建：跳过前端，用已有/占位 assets。
dev:
	CGO_ENABLED=0 go build -o bin/cc-select .

test:
	go test ./internal/...

# 集成测试（跨进程 shell 行为，仅 Unix）。
integration:
	go test -tags integration ./...

# 端到端测试：Playwright 驱动真实浏览器访问真二进制 serve 的配置页。
# globalSetup 会自动构建前端 + 二进制，故无需先 make all。
# 首次运行需先 `cd internal/frontend && npm ci && npx playwright install chromium`。
e2e:
	cd internal/frontend && npm run test:e2e

vet:
	go vet ./...

install: all
	ln -sf $$(pwd)/bin/cc-select $$(go env GOPATH)/bin/cc-select

clean:
	rm -rf bin internal/frontend/node_modules internal/frontend/dist
