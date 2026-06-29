# cc-select

**[English](./README.md) | [中文](./docs/language/README.zh.md) | [日本語](./docs/language/README.ja.md)**

Shell-level AI provider isolation — each terminal window picks its own.

`cc-select` lets different terminal windows on the same machine use different AI model providers with Claude Code. It is the shell-scoped counterpart to [cc-switch](https://github.com/farion1231/cc-switch), which changes providers globally by rewriting `~/.claude/settings.json`.

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

## How it works

A child process cannot modify its parent shell's environment. `cc-select` therefore splits into two layers:

1. The `cc-select` binary prints shell statements (notably `export CLAUDE_CONFIG_DIR=...`).
2. The `ccs()` shell function (injected by `cc-select init`) `eval`s those statements in the current shell.

`cc-select use <provider>` exports `CLAUDE_CONFIG_DIR` to point at an isolated profile directory (`~/.cc-select/profiles/<provider>/settings.json`). Claude Code reads the env from that directory, giving per-terminal provider isolation.

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
