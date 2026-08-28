<!--
Sync Impact Report (v1.1.0, 2026-08-28)
- Version change: 1.0.0 → 1.1.0 (MINOR: new principle added + materially expanded workflow guidance)
- Modified principles: none renamed or removed.
- Added: Principle VIII 测试先行（TDD）; Development Workflow gains TDD ordering, review-loop and
  commit-granularity rules.
- Source: docs/requirements.md「新增需求记录 → 开发基本要求」（用户确认的流程要求，2026-08-28）.
- Follow-up TODOs: none.
-->

<!--
Sync Impact Report (v1.0.0, 2026-08-28)
- Version change: (unratified template stub) → 1.0.0
- Modified principles: none (initial creation).
- Added sections: Core Principles I–VII, Technology Constraints, Development Workflow, Governance.
- Removed sections: none.
- Sources distilled (nothing invented): CLAUDE.md, docs/README.md (governance rules),
  docs/engineering-decisions.md, docs/tech-stack.md, docs/roadmap.md.
- Follow-up TODOs: none — no intentionally deferred placeholders.
-->

# cc-select Constitution

## Core Principles

### I. Eval/Wrapper 拆分不可破坏（NON-NEGOTIABLE）

子进程无法修改父 shell 的环境变量——这是本项目存在的架构基石。

- `cc-select` 二进制**只允许打印** shell 语句（stdout 输出 `export`/`unset`），MUST NOT 尝试从子进程直接修改调用方 shell 的环境。
- 一切 shell 集成 MUST 经 `ccs()` wrapper（由 `cc-select init` 注入 rc 文件 / `$PROFILE`）`eval` 这些语句实现（nvm/direnv 同款模式）。
- 任何"绕过 wrapper 直接改环境"的方案一律不采纳。

理由：违反此原则的代码在 Unix 语义下根本无法工作（见 CLAUDE.md「Preserve the eval/wrapper split」）。

### II. CLAUDE_CONFIG_DIR 隔离机制（NON-NEGOTIABLE）

- provider 路由 MUST 通过 `export CLAUDE_CONFIG_DIR` 指向 `~/.cc-select/profiles/<id>/` 实现；MUST NOT 通过直接 export `ANTHROPIC_*` 切换 provider——实测证明 `~/.claude/settings.json` 的 `env` 会覆盖 shell 变量，该路径对 claude 完全失效（engineering-decisions §6，2026-06-28 真机实测）。
- 官方 Claude provider 的语义是 `unset CLAUDE_CONFIG_DIR`（回默认 `~/.claude`），即「空 provider」，MUST NOT 写入任何 env。
- `cc-select current` MUST 读 shell 环境变量 `$CC_SELECT_ACTIVE`，MUST NOT 读磁盘配置——磁盘配置是全局共享的模板，读它会误报当前 shell 的激活状态（engineering-decisions §3）。
- 隔离粒度维持双模式（Mode A 全隔离 / Mode B 仅 settings.json 隔离，默认 B）；`use` 每次幂等重建 profile（自愈语义）MUST 保留。

### III. 跨平台三 OS 不可遗漏

- 目标 OS 为 macOS / Linux / Windows，任何功能 MUST 三平台同时成立；Windows 仅支持 PowerShell，CMD 明确不支持。
- 涉及文件系统、编码、shell 的改动 MUST 显式考虑平台差异（PowerShell `$PROFILE` BOM 与双 profile 检测、Unix 软链 vs Windows junction、路径分隔符）。
- 新增 shell 支持按 emitter 机制扩展（现有 zsh/bash 共用 + PowerShell；fish 为已规划扩展，见 roadmap 阶段 8）。

### IV. 已定技术选型不漂移

已定选型（真值在 docs/tech-stack.md）：Go 单二进制、本地 Web 服务 GUI（`cc-select gui` + 嵌入式前端）、JSON 两层存储（`providers.json` 元信息 + `profiles/<id>/settings.json` 真值，原子写 0600）、zsh/bash/PowerShell、en/zh i18n。

- 更改任一选型属于重大决策：MUST 先更新 docs/requirements.md 的开放问题表与 docs/tech-stack.md，再动代码。
- 备选方案（桌面 App、SQLite、fish、Node/Rust）保留在文档中备查；在决策反转前 MUST NOT 混入实现。

### V. 质量门禁与验收同步

- 提交前 `make check` MUST 通过；lefthook pre-commit（gofmt/vet/mod-tidy + 前端 typecheck/lint/format）强制拦截。
- 行为变更 MUST 同步更新 docs/acceptance-tests.md——验收用例与实现同 PR 演进。
- 测试分层：Go 单测（`internal/...`）+ 集成测试 + Playwright e2e；切换机制与隔离语义的改动 MUST 有测试覆盖。

### VI. 安全基线不降级

- token 当前明文落 profile `settings.json`（文件 0600、目录 0700）是**已知且已在文档标注的风险**；任何新的凭证写入路径 MUST NOT 比现状更不安全。
- keychain 占位机制（`$keychain:cc-select:<id>:<var>`，`internal/secrets`）是既定升级路径（roadmap 阶段 6）；涉及凭证存储的改动 SHOULD 向该方向收敛而非另起炉灶。

### VII. 文档链真值与 spec 边界

- docs/ 是项目级真值：docs/requirements.md 是用户原始诉求的**唯一记录源**，推导链为 需求 → 分析 → 架构 → 设计 → 验收；MUST NOT 反向改写需求来配合方案。
- specs/（Spec Kit）是特性级工作区：单次迭代的 spec/plan/tasks。特性验收后，结论 MUST 按 docs/README.md「更新协作规范」回流 docs/；specs/ 留存为过程记录，MUST NOT 作为长期真值维护。
- 本宪法从 docs/ 提炼而来；两者冲突时以 docs/ 为准，并 SHOULD 随即修订宪法。

### VIII. 测试先行（TDD，NON-NEGOTIABLE）

- 行为变更 MUST 先写测试并确认其失败（red），再写实现使其通过（green），随后重构（refactor）——严格遵循 red-green-refactor 顺序。
- Bug 修复 MUST 先写能复现该 bug 的失败测试，再修复实现；MUST NOT「先合实现、后补测试」。
- 与原则 V 的关系：V 定义质量门禁（提交前必须通过什么），本原则定义顺序（测试先于实现）；两者叠加生效。

## Technology Constraints（技术约束）

- 语言/运行时：Go 1.24；依赖从简（cobra、zalando/go-keyring、标准库 `net/http` + `embed`、react-i18next 等，真值见 go.mod 与 docs/tech-stack.md）。
- 简单性/YAGNI：标准库优先、单二进制、避免重依赖；引入复杂度 MUST 给出理由。
- 构建/发布：Makefile + GoReleaser + GitHub Actions；dev build 与 release 均未签名，Smart App Control（SAC）限制是已记录的已知问题（docs/windows-support.md §7），不是待修 bug。
- i18n 约定：根目录 `README.md` 为默认版本；翻译放 `docs/language/README.<lang>.md`；每次改 README MUST 同步更新各语言切换器相对路径。
- Spec Kit 工具链：specify-cli 锁定 **v0.16.5**（skills 模式，`.claude/skills/`）；升级版本属于流程决策，MUST 更新本节版本号。

## Development Workflow（开发工作流）

- 分支：特性开发在独立分支进行，PR 目标分支为 `main`；提交信息沿用 Conventional Commits（`feat:`/`fix:`/`chore:`/`docs:`）。
- **TDD 顺序**：先写失败测试再写实现（见原则 VIII）；PR 中测试与实现 MUST 同提交。
- **commit 粒度**：一个 commit 只承载一个功能点；多功能点改动 MUST 拆分为多个 commit。commit message 由 lefthook（commit-msg hook）调用 commitlint 按 Conventional Commits 规则校验。
- **循环 code review**：每个特性/版本完成后 MUST 执行循环 code review（静态检查 + 代码评审），修复所有**中等严重及以上**问题并复评，直至无中等严重问题方可合入。
- 常规改动（小修、文档）：直接遵循 CLAUDE.md「When making changes」即可，无需走 SDD 流程。
- SDD 全流程（`/speckit-specify` → `/speckit-plan` → `/speckit-tasks` → `/speckit-implement`，可选 `/speckit-clarify`/`/speckit-analyze`）：仅用于**预计超过一天或跨模块的特性**（如 roadmap 阶段 6–9 的 keychain 收尾、PS1 集成、fish 支持）。
- SDD 产物流向：spec/plan/tasks 生成于 `specs/<NNN-feature>/`；实现完成、验收回流 docs/ 后，spec 目录不再作为维护对象。
- 用户新诉求一律先进 docs/requirements.md（唯一入口），再决定是否为其发起一个 spec。

## Governance

- 宪法地位：SDD 流程（尤其 `/speckit-plan` 与 `/speckit-implement`）执行期间 MUST 校验方案是否符合本宪法；宪法与 docs/ 链冲突时，以 docs/ 为最终真值并修订宪法。
- 修订流程：版本按语义化版本演进——MAJOR=原则删除或重定义，MINOR=新增原则/实质扩展，PATCH=措辞澄清；每次修订 MUST 在文件头部 Sync Impact Report 记录变更与来源。
- 合规审查：PR review 与 `/speckit-analyze` 检查宪法符合性；引入复杂度或偏离选型 MUST 在 PR 描述中给出理由。
- 运行期开发指引：见仓库根 CLAUDE.md；文档治理规则：见 docs/README.md。

**Version**: 1.1.0 | **Ratified**: 2026-08-28 | **Last Amended**: 2026-08-28
