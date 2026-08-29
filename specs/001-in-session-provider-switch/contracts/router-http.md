# Contract: 路由 daemon HTTP 行为（loopback only）

> 守护进程仅绑定 `127.0.0.1`；对 Claude Code 而言它就是「一个 Anthropic 兼容端点」。契约面向实现与排障。

## 1. 端点

| 路径 | 方法 | 鉴权 | 行为 |
|---|---|---|---|
| `/healthz` | GET | 无（仅返回存活与版本，无敏感信息） | `{"status":"ok","version":"0.0.6"}`；ensureDaemon 的探测与关闭侧信道复用此路径（内部 token 头） |
| `/v1/messages` 及其余全部路径 | 透传 | Bearer 伪 token 必需 | 反向代理至当前路由 provider（见 §2） |

## 2. 转发规则（每请求）

1. **归属**：取 `Authorization: Bearer <tid>` → 查 `routes.json`（每请求重读，D10）。未知/缺失 tid → `401`，响应体含人类可读指引（「请先 `ccs use <provider>`；裸启 claude 需清理全局 settings env」——L1）。
2. **上游地址**：provider env `ANTHROPIC_BASE_URL`（占位符先经 keychain 解析，内存缓存，D8）。
3. **认证注入**：剥离入站 Authorization，按 provider env 注入：`ANTHROPIC_AUTH_TOKEN` → `Authorization: Bearer <真值>`；`ANTHROPIC_API_KEY` → `x-api-key: <真值>`（两者可并存，随 provider 配置）。
4. **model 改写**（D9）：请求 body 为 JSON 时，若 provider 定义 `ANTHROPIC_MODEL` 则替换 `model` 字段后重建 body（`Content-Length` 相应更新）；非 JSON body 透传不改。
5. **流式**：SSE 响应直通，flush 即时（`FlushInterval` 立即）；响应体不做任何改写。
6. **上游失败**：非 2xx 原样透传状态码与 body（保留 provider 错误语义，便于排障）；连接失败 → `502` + 明确「上游连接失败」消息。

## 3. 安全不变式

- 仅 loopback 监听；不提供任何列出/修改路由的 HTTP 接口（管理面 = CLI + 文件，见 cli.md）。
- 日志字段：时间、tid 短码、provider、路径、状态码、耗时；**任何情况下不记录**真 token、keychain 真值、完整请求/响应 body。
- 密钥真值仅存在于 daemon 进程内存（解析后缓存）；磁盘上只有 keychain 占位符（D8）。

## 4. 状态文件与自愈

- `router.json` 记录 addr/pid/version（data-model §3）；重启沿用既有 addr（L2 自愈前提）。
- ensureDaemon 版本不匹配（升级后）→ 优雅停旧起新。

## 5. 合规检查点（宪法）

- I：daemon 不写任何调用方 shell 的 env（它只是 HTTP 服务）。
- VI：安全性 ≥ 现状（token 明文面收敛为 0，Mode P 内）。
