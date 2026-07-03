# cc-select

**[English](../../README.md) | [中文](./README.zh.md) | [日本語](./README.ja.md)**

シェルレベルの AI プロバイダー分離 —— 各ターミナルウィンドウが独自のプロバイダーを選択。

`cc-select` を使うと、同じマシン上の異なるターミナルウィンドウで、Claude Code と共に異なる AI モデルプロバイダーを使用できます。これは [cc-switch](https://github.com/farion1231/cc-switch) のシェルスコープ版です。cc-switch は `~/.claude/settings.json` を書き換えてグローバルにプロバイダーを切り替えますが、`cc-select` は現在のターミナルとその子プロセスにのみ影響します。

## クイックスタート

```bash
# バイナリをインストールして PATH に追加し、shell ラッパーを注入する
cc-select init >> ~/.zshrc
source ~/.zshrc

# プロバイダーを追加
cc-select add glm

# 現在の shell のみでプロバイダーを切り替え
ccs use glm

# Web 設定 UI を開く
cc-select gui
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
