# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project status

**This repository is post-MVP implementation.** `cc-select` is implemented as a Go CLI with an embedded local Web GUI.

- Language/runtime: **Go 1.24** + embedded web frontend (npm/TypeScript/React build).
- Build system: `Makefile` + GoReleaser + GitHub Actions.
- Tests: `go test ./internal/...` + integration tests + Playwright e2e tests.

### Common commands

```bash
# Build everything (frontend + Go binary) into ./bin/cc-select
make all

# Fast dev build (skip frontend rebuild)
make dev

# Run Go unit tests
make test

# Run integration tests (Unix only)
make integration

# Run e2e tests (requires frontend deps + Playwright browsers)
make e2e

# Lint/vet
make vet
```

## What cc-select is

`cc-select` is a CLI for **shell-level (per-terminal) isolation of AI model providers** for Claude Code.

> Shell-level AI provider isolation — each terminal window picks its own.

It is the shell-scoped counterpart to [cc-switch](https://github.com/farion1231/cc-switch), which switches providers **globally** by rewriting `~/.claude/settings.json`. `cc-select` instead lets terminal A run one provider (e.g. GLM) and terminal B run another (e.g. DeepSeek) simultaneously on the same machine.

## The core architectural constraint

The whole project rests on one Unix fact and one consequence:

- **A child process cannot modify its parent shell's environment.** A standalone binary, no matter what it does, cannot `export` variables back into the shell that launched it.
- **Therefore `cc-select` must be split into two cooperating layers** — a binary that *emits* shell statements, and a shell wrapper function (installed into `~/.zshrc` / `~/.bashrc` / `$PROFILE`) that `eval`s them in the caller's own shell.

This is the same `eval "$(tool ...)"` pattern used by `nvm`, `pyenv`, and `direnv`.

## Implemented architecture

1. **The CLI binary** (`cc-select`) owns the provider config store (`~/.cc-select/providers.json` + `~/.cc-select/profiles/<id>/settings.json`) and only *prints* the shell statements a provider needs. It never modifies the caller's environment directly.

2. **The shell wrapper** (`ccs()`) is injected by `cc-select init`. It runs `eval "$(cc-select use <provider>)"` so the exports land in the current shell.

3. **Switching mechanism**: instead of exporting `ANTHROPIC_*` variables directly, `cc-select use X` exports `CLAUDE_CONFIG_DIR` to point at an isolated profile directory (`~/.cc-select/profiles/<id>/`). Claude Code reads the `settings.json` env from that directory. This avoids Claude Code's global `~/.claude/settings.json` overriding shell variables.

4. **`current` command** reads the shell's `$CC_SELECT_ACTIVE` env var (not the on-disk store), because the on-disk store is global/shared and would misreport which provider *this* shell is using.

## Project layout

```
.
├── main.go                     # Entry point
├── Makefile                    # Build/test tasks
├── .goreleaser.yaml            # Cross-platform release builds
├── internal/
│   ├── app/                    # Dependency assembly (config + secrets)
│   ├── cli/                    # Cobra subcommands (use/list/current/add/edit/remove/init/gui)
│   ├── config/                 # providers.json loading/saving
│   ├── profile/                # Per-provider CLAUDE_CONFIG_DIR directories
│   ├── secrets/                # OS keychain abstraction (macOS/Linux/Windows)
│   ├── shell/                  # Shell statement emitters (zsh/bash/powershell)
│   ├── switcher/               # Plans environment variable changes for switching
│   ├── version/                # Build-time version injection
│   ├── web/                    # Local HTTP server + REST API + embedded frontend assets
│   └── frontend/               # React/TypeScript web configuration UI
└── docs/                       # Requirements, architecture, design, and acceptance docs
```

## Key design decisions (already made)

- **Language**: Go (single static binary, cross-compilation, fast startup).
- **GUI**: Local web server (`cc-select gui`) + browser, not a desktop app.
- **Storage**: JSON files (`providers.json` + per-provider `settings.json`). Sensitive values should ideally be stored via keychain placeholders; the keychain infrastructure exists but the CLI/Web flows currently write plaintext tokens to profile `settings.json`.
- **Shell support**: zsh/bash share one emitter; PowerShell emitter exists; fish is not yet supported.
- **OS support**: macOS, Linux, Windows (PowerShell only; CMD is explicitly unsupported).

## Open decisions / known gaps

- **API-key storage consistency**: `internal/secrets/` and keychain placeholders exist, but `add`/`edit`/`web` currently write plaintext `ANTHROPIC_AUTH_TOKEN` into profile `settings.json`. Decide whether to fully switch to keychain placeholders or accept plaintext with documented risks.
- **PS1 integration**: `init` does not yet inject a prompt hook to display the active provider.
- **Fish shell support**: only zsh/bash/PowerShell emitters exist.
- **Windows validation**: PowerShell emitter is implemented but not yet covered by CI integration tests.

## When making changes

- **README i18n convention**: Keep the default `README.md` in the repository root. Place translations under `docs/language/` as `docs/language/README.<lang>.md` (e.g. `docs/language/README.zh.md`). Update the language switcher in every README to point to the correct relative paths. This keeps the root directory clean while still letting GitHub render the translations when visited directly.
- Run `make test` and `make vet` before committing.
- Update `docs/acceptance-tests.md` when behavior changes.
- Preserve the eval/wrapper split — do not try to modify the parent shell environment from the binary.
