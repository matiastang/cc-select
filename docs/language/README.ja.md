# cc-select

**[English](../../README.md) | [中文](./README.zh.md) | [日本語](./README.ja.md)**

シェルレベルの AI プロバイダー分離 —— 各ターミナルウィンドウが独自のプロバイダーを選択。

`cc-select` を使うと、同じマシン上の異なるターミナルウィンドウで、Claude Code と共に異なる AI モデルプロバイダーを使用できます。これは [cc-switch](https://github.com/farion1231/cc-switch) のシェルスコープ版です。cc-switch は `~/.claude/settings.json` を書き換えてグローバルにプロバイダーを切り替えますが、`cc-select` は現在のターミナルとその子プロセスにのみ影響します。

## インストール

### macOS / Linux（Homebrew）

```bash
brew tap matiastang/cc-select
brew install cc-select
```

### Windows（Scoop）

```powershell
scoop bucket add cc-select https://github.com/matiastang/scoop-cc-select
scoop install cc-select
```

### macOS / Linux（インストールスクリプト）

Homebrew を使わない場合は、公式スクリプトでインストールまたは更新できます：

```bash
curl -fsSL https://raw.githubusercontent.com/matiastang/cc-select/main/scripts/install.sh | sh
```

特定のディレクトリにインストールする場合：

```bash
curl -fsSL https://raw.githubusercontent.com/matiastang/cc-select/main/scripts/install.sh | sh -s -- --dir /usr/local/bin
```

スクリプトは既存インストールの場所を検出してそのバイナリを置き換え、なければ `~/.local/bin`（必要に応じて `/usr/local/bin`）にインストールします。

### Windows（インストールスクリプト）

Scoop を使わない場合は、公式 PowerShell スクリプトでインストールまたは更新できます：

```powershell
irm https://raw.githubusercontent.com/matiastang/cc-select/main/scripts/install.ps1 | iex
```

スクリプトは `%LOCALAPPDATA%\cc-select` にインストールし、そのディレクトリをユーザーの PATH に追加して、既存のインストールをその場で更新します。

> 注意: Windows ARM64 はまだサポートされていません。公開されているのは Windows amd64 ビルドのみです。

### 手動インストール

[GitHub Releases](https://github.com/matiastang/cc-select/releases) からプラットフォームに応じたアーカイブをダウンロードし、`cc-select`（Windows の場合は `cc-select.exe`）を `PATH` の通ったディレクトリに展開してから、以下の shell 統合手順を実行してください。

## クイックスタート

### 1. shell 統合

`cc-select init` は `ccs` に必要な shell ラッパーを出力します。お使いの shell の起動ファイルに追記してから再読み込みしてください：

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

> 対応済み shell：**zsh / bash / PowerShell**。Windows の CMD は非対応です。fish はまだ対応していません。

### 2. プロバイダーを追加

最も簡単な方法は Web 設定 UI から行うことです：

```bash
cc-select gui
```

CLI から追加することもできます：

```bash
cc-select add glm
```

### 3. 現在の shell のみでプロバイダーを切り替え

```bash
ccs use glm
```

## Windows での初回実行（SmartScreen / Smart App Control）

cc-select は**未署名のオープンソース**バイナリです。Windows では：

- **SmartScreen**（すべてのユーザー）：初回実行時に「Windows によって PC は保護されました」と表示されることがあります——「詳細情報」→「実行」をクリック。
- **Smart App Control (SAC)**（有効にしている場合のみ）：SAC は未署名・不明な exe をブロックし、「実行」オプションは**ありません**。有効な場合は、SAC をオフにする（永続的・元に戻せない）か、SAC が無効な環境で実行してください。詳細は [docs/windows-support.md §7](../windows-support.md#7-smart-app-control-与未签名可执行文件) を参照。

## 仕組み

子プロセスは親 shell の環境変数を変更できません。そのため `cc-select` は 2 つの層に分かれています：

1. `cc-select` バイナリは shell 文（主に `export CLAUDE_CONFIG_DIR=...`）を**表示するだけ**です。
2. `ccs()` shell 関数（`cc-select init` によって `~/.zshrc` などに注入されます）は、これらの文を現在の shell で `eval` します。

`cc-select use <provider>` は `CLAUDE_CONFIG_DIR` をエクスポートし、独立した profile ディレクトリ（`~/.cc-select/profiles/<provider>/settings.json`）を指します。Claude Code はそのディレクトリの env を読み取るため、ターミナルごとに異なるプロバイダーを使うことができます。

## 分離モード

- **Mode B — `settings-only`（デフォルト）**: 各プロバイダーごとに `settings.json` のみ分離します。履歴・プラグイン・commands などは `~/.claude` へのリンクで共有されます。
- **Mode A — `full`**: profile ディレクトリ全体を完全に分離します。

グローバル既定値は `cc-select mode` で確認・設定できます。per-provider 上書きや一度きりの上書きには `cc-select edit <id> --mode ...` または `ccs use <id> --mode ...` を使ってください。詳細は [docs/isolation-modes.md](../isolation-modes.md) を参照。

## セキュリティに関する注意

API キーは現在、`~/.cc-select/profiles/<id>/settings.json` に**平文**で保存されています（ファイル権限 `0600`、ディレクトリ権限 `0700`）。リスクレベルは `~/.claude/settings.json` と同じです。今後、システム Keychain への対応を予定しています。keychain プレースホルダー機構と `internal/secrets` パッケージはすでに実装済みで、CLI/Web の書き込みパスに接続する予定です。

## ビルド

```bash
make all      # フロントエンド + バイナリをビルド → ./bin/cc-select
make test     # Go ユニットテストを実行
make vet      # go vet を実行
make e2e      # Playwright エンドツーエンドテストを実行
```

## ライセンス

Apache License 2.0
