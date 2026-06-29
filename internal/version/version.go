// Package version 持有 cc-select 的版本信息。
// Version 在构建期由 ldflags 注入（见 .goreleaser.yaml），默认值为 dev。
package version

// Version 是当前二进制版本。构建期可用 -ldflags "-X ...version.Version=..." 覆盖。
var Version = "dev"
