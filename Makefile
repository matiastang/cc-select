# 跨平台构建脚本：macOS / Linux / Windows（git bash 与 cmd 均支持）。
#
# 跨平台要点（让同一份 Makefile 不依赖某平台独有 shell）：
#   1. CGO 用 `export` 注入环境，避免 `CGO_ENABLED=0 cmd` 这种 sh 专有写法；
#   2. 建目录用 `-mkdir bin`：mkdir 在 sh/cmd 都存在，`-` 让 Make 忽略"已存在"错误（等价 mkdir -p）；
#   3. 版本号用 `$(shell ...)` 解析期取值，命令替换不写进 recipe；
#   4. 二进制名用 `$(GOEXE)`：Windows 补 .exe，Unix 为空——让 cmd/PowerShell 也能直接调用 bin/ 产物；
#   5. install 用 `go install`，跨平台编译并装入 GOPATH/bin，无需 ln/copy；
#   6. clean 的 rm/rmdir 无跨平台等价：用 MSYSTEM 区分 Windows 上的 git-bash(sh) 与 cmd。
#      （不能用 $(OS)：git bash 也运行在 Windows，$(OS) 同为 Windows_NT，会误选 cmd 分支。）
.PHONY: all build frontend dev test integration e2e vet scripts-check install clean \
  fmt fmt-check check mod-tidy-check \
  frontend-typecheck frontend-lint frontend-format-check frontend-check

# 关闭 CGO（go-keyring 纯 Go 无需 CGO）：export 让所有 recipe 子进程继承，跨平台。
export CGO_ENABLED := 0

# 版本号：无 tag 时回退 dev。$(shell) 解析期执行，git 是跨平台 exe。
VERSION := $(shell git describe --tags --always --dirty || echo dev)
# 二进制后缀：Windows=.exe，Unix=空。用于 -o 让产物名符合平台惯例。
GOEXE := $(shell go env GOEXE)

# Windows 上区分 shell：cmd.exe（MSYSTEM 为空）需用 cmd 命令；
# git bash（MSYSTEM 非空）走 sh 命令。macOS/Linux 无 OS=Windows_NT，天然走 sh 分支。
ifeq ($(OS),Windows_NT)
  ifndef MSYSTEM
    CMD_SHELL := 1
  endif
endif

# 默认目标：构建含最新前端的二进制。
all: frontend build

# 构建前端（产物输出到 internal/web/assets，被 Go embed）。
frontend:
	cd internal/frontend && npm install && npm run build

# 构建 cc-select 二进制到 ./bin/（Windows 为 cc-select.exe，Unix 为 cc-select）。
build:
	-mkdir bin
	go build -ldflags "-X github.com/cc-select/cc-select/internal/version.Version=$(VERSION)" -o bin/cc-select$(GOEXE) .

# 快速开发构建：跳过前端，用已有/占位 assets。
dev:
	-mkdir bin
	go build -o bin/cc-select$(GOEXE) .

# 运行 Go 单元测试。
test:
	go test ./internal/...

# 集成测试（跨进程 shell 行为，仅 Unix）。
integration:
	go test -tags integration ./...

# 端到端测试（Playwright）。首次需：cd internal/frontend && npx playwright install chromium
e2e:
	cd internal/frontend && npm run test:e2e

# 静态检查。
vet:
	go vet ./...

# 安装脚本语法检查（shellcheck 优先，否则 sh -n；Windows 脚本用 PowerShell Tokenize 校验）。
scripts-check:
	@if command -v shellcheck >/dev/null 2>&1; then \
		shellcheck scripts/install.sh; \
	else \
		sh -n scripts/install.sh; \
	fi && \
	echo "scripts/install.sh OK"
	@PS=""; \
	if command -v pwsh >/dev/null 2>&1; then \
		PS=pwsh; \
	elif command -v powershell >/dev/null 2>&1; then \
		PS=powershell; \
	fi; \
	if [ -n "$$PS" ]; then \
		$$PS -NoProfile -NonInteractive -Command '$$tokens = $$null; [System.Management.Automation.PSParser]::Tokenize((Get-Content -Raw "scripts/install.ps1"), [ref]$$tokens) | Out-Null' && \
		echo "scripts/install.ps1 OK"; \
	else \
		echo "pwsh/powershell not found, skipping scripts/install.ps1 syntax check"; \
	fi

# 格式化所有代码
fmt:
	gofmt -w .
	cd internal/frontend && npm run format

# Go 格式检查（跨平台：Unix 用 sh，Windows cmd 用 findstr）
ifdef CMD_SHELL
fmt-check:
	@gofmt -l . | findstr . >nul && (gofmt -l . & exit /b 1) || echo "Go formatting OK"
else
fmt-check:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "The following Go files need formatting:"; \
		echo "$$unformatted"; \
		echo "Run 'make fmt' to fix."; \
		exit 1; \
	fi; \
	echo "Go formatting OK"
endif

# go mod tidy 一致性检查（Go 1.24 的 -diff 不修改文件，退出码非零表示不一致）
mod-tidy-check:
	@echo "Checking go.mod/go.sum consistency..."
	go mod tidy -diff

# 前端格式检查
frontend-format-check:
	cd internal/frontend && npm run format:check

# 前端类型检查
frontend-typecheck:
	cd internal/frontend && npm run typecheck

# 前端 Lint
frontend-lint:
	cd internal/frontend && npm run lint

# 前端静态检查（类型 + Lint + 格式）
frontend-check: frontend-typecheck frontend-lint frontend-format-check

# 统一静态检查入口（本地和 CI 都使用）
check: fmt-check vet frontend-typecheck frontend-lint frontend-format-check scripts-check mod-tidy-check

# 安装到 GOPATH/bin：go install 跨平台编译并装入，无需 ln/copy。
install:
	go install -ldflags "-X github.com/cc-select/cc-select/internal/version.Version=$(VERSION)" .

# 清理构建产物。
clean:
ifdef CMD_SHELL
	-rmdir /s /q bin
	-rmdir /s /q internal\frontend\node_modules
	-rmdir /s /q internal\frontend\dist
else
	rm -rf bin internal/frontend/node_modules internal/frontend/dist
endif
