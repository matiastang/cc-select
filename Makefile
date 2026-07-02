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
.PHONY: all build frontend dev test integration e2e vet install clean

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
