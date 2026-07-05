# cc-select

**[English](./README.md) | [中文](./docs/language/README.zh.md) | [日本語](./docs/language/README.ja.md)**

Shell-level AI provider isolation — each terminal window picks its own.

`cc-select` lets different terminal windows on the same machine use different AI model providers with Claude Code. It is the shell-scoped counterpart to global-switch tools like [cc-switch](https://github.com/farion1231/cc-switch), which changes providers globally by rewriting `~/.claude/settings.json`.

## Install

### macOS / Linux (Homebrew)

```bash
brew tap matiastang/cc-select
brew install cc-select
```

### Windows (Scoop)

```powershell
scoop bucket add cc-select https://github.com/matiastang/scoop-cc-select
scoop install cc-select
```

### Manual

Download the archive for your platform from [GitHub Releases](https://github.com/matiastang/cc-select/releases), extract `cc-select` (or `cc-select.exe` on Windows) to a directory on your `PATH`, then follow the shell integration step below.

## Quick start

```bash
# Install the binary and add it to your PATH, then inject the shell wrapper:
cc-select init >> ~/.zshrc
source ~/.zshrc

# Add a provider
cc-select add glm

# Switch provider in this shell only
ccs use glm

# Open the web configuration UI
cc-select gui
```

> Supported shells: zsh / bash / PowerShell. CMD is not supported on Windows.

## First run on Windows (SmartScreen / Smart App Control)

cc-select is an **unsigned open-source** binary. On Windows:

- **SmartScreen** (all users): the first run may show "Windows protected your PC" — click **More info** → **Run anyway**.
- **Smart App Control (SAC)** (only if you enabled it): SAC blocks unsigned/unknown exes with **no** "run anyway" option. If enabled, you must turn SAC off (permanent, irreversible) or run on a machine without SAC. See [docs/windows-support.md §7](./docs/windows-support.md#7-smart-app-control-与未签名可执行文件).

## How it works

A child process cannot modify its parent shell's environment. `cc-select` therefore splits into two layers:

1. The `cc-select` binary prints shell statements (notably `export CLAUDE_CONFIG_DIR=...`).
2. The `ccs()` shell function (injected by `cc-select init`) `eval`s those statements in the current shell.

`cc-select use <provider>` exports `CLAUDE_CONFIG_DIR` to point at an isolated profile directory (`~/.cc-select/profiles/<provider>/settings.json`). Claude Code reads the env from that directory, giving per-terminal provider isolation.

## Isolation modes

- **Mode B — `settings-only` (default)**: only `settings.json` is isolated per provider; history, plugins, commands, etc. are shared via links back to `~/.claude`.
- **Mode A — `full`**: the entire profile directory is isolated.

Use `cc-select mode` to view/set the global default, or `cc-select edit <id> --mode ...` / `ccs use <id> --mode ...` for per-provider or one-time overrides. See [docs/isolation-modes.md](docs/isolation-modes.md) for details.

## Security note

API keys are currently stored **in plaintext** inside `~/.cc-select/profiles/<id>/settings.json` (file permissions `0600`, directory `0700`). This is the same risk level as `~/.claude/settings.json`. A keychain-backed storage upgrade is planned; the placeholder mechanism and `internal/secrets` package are already in place.

## Build

```bash
make all      # build frontend + binary -> ./bin/cc-select
make test     # run Go unit tests
make vet      # run go vet
make e2e      # run Playwright e2e tests
```

## License

Apache License 2.0
