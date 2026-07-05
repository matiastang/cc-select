# 发布流程与包管理器维护

> 本文为维护者文档，说明如何触发 cc-select 的 Release、如何配置包管理器自动发布所需的 Token，以及发版后如何验证。
> 普通用户安装方式见 [distribution.md](./distribution.md)。

---

## 1. 触发 Release

发布完全由 Git tag 驱动：

```bash
git tag v0.1.0
git push origin v0.1.0
```

`.github/workflows/release.yml` 会在 tag 推送后自动：

1. 构建前端（`npm ci && npm run build`）。
2. 调用 GoReleaser 交叉编译并上传二进制到 GitHub Releases。
3. 生成 `checksums.txt` 和 changelog。
4. 将 Homebrew Formula 提交到 `matiastang/homebrew-cc-select`。
5. 将 Scoop manifest 提交到 `matiastang/scoop-cc-select`。

> 官方安装/更新脚本（`scripts/install.sh` 与 `scripts/install.ps1`）随主仓库代码一起发布，无需额外推送。用户始终通过 `raw.githubusercontent.com` 获取最新版脚本，脚本内部再查询 GitHub Releases 下载对应二进制。

> `.goreleaser.yaml` 中 `release.draft: true`，因此 Release 会先以草稿形式创建。维护者需要手动进入 GitHub Release 页面点击 **Publish release**。

---

## 2. 前置条件：包管理器仓库与 Token

在首次发版前，必须完成以下配置。

### 2.1 创建两个空仓库

| 仓库 | 用途 | 默认分支 |
|---|---|---|
| `matiastang/homebrew-cc-select` | Homebrew Tap | `main` |
| `matiastang/scoop-cc-select` | Scoop Bucket | `main` |

创建时勾选「Add a README file」，确保 `main` 分支存在。GoReleaser 会在首次 Release 时自动创建 `Formula/cc-select.rb` 和 `cc-select.json`。

### 2.2 生成 Personal Access Token

GoReleaser 需要**向这两个仓库写入内容**，但 GitHub Actions 默认的 `GITHUB_TOKEN` 只能访问当前仓库，不能跨仓提交。因此需要单独创建一个 PAT。

**推荐：Fine-grained PAT**

1. 打开 GitHub 个人设置 → **Developer settings** → **Personal access tokens** → **Fine-grained tokens**。
2. 点击 **Generate new token**。
3. 填写 Token name，例如 `cc-select-tap-token`。
4. **Resource owner** 选择 `matiastang`。
5. **Repository access** 选择 **Only select repositories**，并勾选：
   - `matiastang/homebrew-cc-select`
   - `matiastang/scoop-cc-select`
6. **Permissions** → **Repository permissions** → **Contents** 选择 **Read and write**。
7. 其余权限保持默认（无需勾选）。
8. 点击 **Generate token**，复制生成的 token。

> 也可以使用 Classic PAT，只需勾选 `repo` 作用域。Fine-grained 更最小权限，推荐。

### 2.3 配置仓库 Secret

将上一步复制的 token 添加到主仓库 `matiastang/cc-select`：

1. 打开 `matiastang/cc-select` 仓库 Settings。
2. 左侧选择 **Secrets and variables** → **Actions**。
3. 点击 **New repository secret**。
4. Name 填 `TAP_GITHUB_TOKEN`。
5. Secret 填刚才复制的 token。
6. 点击 **Add secret**。

`.github/workflows/release.yml` 已配置：

```yaml
env:
  GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
  TAP_GITHUB_TOKEN: ${{ secrets.TAP_GITHUB_TOKEN }}
```

这样 GoReleaser 在发布时就能用 `TAP_GITHUB_TOKEN` 向 Tap/Bucket 仓库提交 manifest。

---

## 3. 发版后验证

Release workflow 完成后，按以下清单检查：

### 3.1 GitHub Releases

- 进入 `matiastang/cc-select/releases`。
- 确认存在对应版本的 Release（draft 状态需手动发布）。
- 确认产物包含：
  - `cc-select_<version>_darwin_amd64.tar.gz`
  - `cc-select_<version>_darwin_arm64.tar.gz`
  - `cc-select_<version>_linux_amd64.tar.gz`
  - `cc-select_<version>_linux_arm64.tar.gz`
  - `cc-select_<version>_windows_amd64.zip`
  - `checksums.txt`

### 3.2 Homebrew Tap

- 进入 `matiastang/homebrew-cc-select`。
- 确认 `main` 分支有最新 commit，更新 `Formula/cc-select.rb`。
- 在 macOS/Linux 干净环境验证：

```bash
brew tap matiastang/cc-select
brew install cc-select
cc-select --version
```

### 3.3 Scoop Bucket

- 进入 `matiastang/scoop-cc-select`。
- 确认 `main` 分支有最新 commit，更新 `cc-select.json`。
- 在 Windows 干净环境验证：

```powershell
scoop bucket add cc-select https://github.com/matiastang/scoop-cc-select
scoop install cc-select
cc-select --version
```

### 3.4 官方安装脚本

- 确认主仓库 `scripts/install.sh` 与 `scripts/install.ps1` 存在且语法正确。
- 在干净环境验证：

```bash
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/matiastang/cc-select/main/scripts/install.sh | sh
cc-select --version
```

```powershell
# Windows
irm https://raw.githubusercontent.com/matiastang/cc-select/main/scripts/install.ps1 | iex
cc-select --version
```

- 更新场景：再次运行同一条命令，应能检测到已有安装并替换为新版本。

### Release workflow 报错：`could not commit tap formula`

- 检查 `TAP_GITHUB_TOKEN` 是否已添加到 `matiastang/cc-select` 的 Actions secrets。
- 检查 token 是否对 `homebrew-cc-select` 和 `scoop-cc-select` 两个仓库有 **Contents: Read and write** 权限。
- 检查两个目标仓库是否存在，且默认分支名为 `main`。

### 包管理器安装的是旧版本

- Homebrew 有本地缓存，尝试 `brew update && brew upgrade cc-select`。
- Scoop 尝试 `scoop update cc-select` 或 `scoop update` 刷新 bucket。

### 本地想预览 manifest（不发布）

如果本地已安装 GoReleaser，可运行：

```bash
goreleaser release --snapshot --clean
```

然后在 `dist/` 目录查看生成的 `.rb` 和 `.json` 文件。

---

## 5. 安全说明

- `TAP_GITHUB_TOKEN` 只对两个包管理器仓库有写权限，**不应对主仓库 `matiastang/cc-select` 开放写入**，遵循最小权限原则。
- 如果未来改为组织仓库或需要更严格的审计，可将 PAT 替换为 GitHub App installation token，并在 workflow 中通过相应 action 获取临时 token。
