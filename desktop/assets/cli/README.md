# 构建期注入目录（do not delete）

`make desktop-build`（desktop-cli 步骤）会把与桌面端同 GOOS/GOARCH 的 `cc-select`
二进制拷入本目录，经 `desktop/cliembed.go` 的 `//go:embed assets/cli` 嵌入 App。

本 README 是**可嵌入占位文件**：保证全新检出（CI / 新克隆，目录里没有二进制）时
embed 模式仍能匹配、`go build ./...` 可通过——此时提取逻辑按文件名找不到
`cc-select(.exe)`，会返回「先跑 make desktop-build」的明确错误（设计语义）。

注意：go:embed 不匹配 `.` 开头的文件，所以占位不能是 .gitignore/.gitkeep。
