# cc-select

**[English](./README.md) | [中文](./docs/language/README.zh.md) | [日本語](./docs/language/README.ja.md)**

Shell-level AI provider isolation for Claude Code — each terminal window picks its own.

`cc-select` lets different terminal windows on the same machine use different AI model providers with Claude Code. It is the shell-scoped counterpart to global-switch tools like [cc-switch](https://github.com/farion1231/cc-switch), which changes providers globally by rewriting `~/.claude/settings.json`.

## Install

### macOS / Linux (Homebrew)

One-line install (no `brew tap` needed):

```bash
brew install matiastang/cc-select/cc-select
```

Or add the tap explicitly first:

```bash
brew tap matiastang/cc-select
brew install cc-select
```

### Windows (Scoop)

One-line install (no `scoop bucket add` needed):

```powershell
scoop install https://raw.githubusercontent.com/matiastang/scoop-cc-select/main/cc-select.json
```

Or add the bucket explicitly first:

```powershell
scoop bucket add cc-select https://github.com/matiastang/scoop-cc-select
scoop install cc-select
```

### macOS / Linux (Install script)

If you don't use Homebrew, you can install or update with the official script:

```bash
curl -fsSL https://raw.githubusercontent.com/matiastang/cc-select/main/scripts/install.sh | sh
```

To install to a specific directory:

```bash
curl -fsSL https://raw.githubusercontent.com/matiastang/cc-select/main/scripts/install.sh | sh -s -- --dir /usr/local/bin
```

The script detects the location of an existing installation and replaces the binary in that location; otherwise it installs to `~/.local/bin` (or `/usr/local/bin` if necessary).

### Windows (Install script)

If you don't use Scoop, you can install or update with the official PowerShell script:

```powershell
irm https://raw.githubusercontent.com/matiastang/cc-select/main/scripts/install.ps1 | iex
```

The script installs to `%LOCALAPPDATA%\cc-select`, adds that directory to your user PATH, and updates an existing installation in place.

> Note: Windows ARM64 is not yet supported; only Windows amd64 builds are published.

### Manual

Download the archive for your platform from [GitHub Releases](https://github.com/matiastang/cc-select/releases), extract `cc-select` (or `cc-select.exe` on Windows) to a directory on your `PATH`, then follow the shell integration step below.

## Quick start

### 1. Shell integration

`cc-select init` prints the shell wrapper that makes `ccs` work. Append it to your shell's startup file, then reload:

```bash
# macOS / Linux — zsh
cc-select init >> ~/.zshrc && source ~/.zshrc

# macOS / Linux — bash
cc-select init >> ~/.bashrc && source ~/.bashrc
```

```powershell
# Windows — PowerShell
cc-select init >> $PROFILE
```

> Supported shells: **zsh / bash / PowerShell**. CMD is not supported on Windows; fish is not yet supported.

### 2. Add a provider

The easiest way is through the web UI:

```bash
cc-select gui
```

You can also add a provider from the CLI:

```bash
cc-select add glm
```

### 3. List providers

```bash
ccs list
```

### 4. Switch provider in this shell only

```bash
ccs use glm
```

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
make check    # run all static checks (format, type, lint, scripts, mod tidy)
```

## Development

Install dependencies and git hooks (hooks are installed automatically when you run `npm install` in `internal/frontend`):

```bash
cd internal/frontend && npm install
```

Run all static checks locally:

```bash
make check
```

Auto-format everything:

```bash
make fmt
```

`git commit` will run a pre-commit hook that blocks the commit if any static check fails.

## License

Apache License 2.0
