import { test as base } from "@playwright/test";
import { spawn, type ChildProcessWithoutNullStreams } from "child_process";
import { mkdtemp, rm } from "fs/promises";
import { tmpdir } from "os";
import path from "path";

// 二进制相对 cwd（internal/frontend）的位置：repoRoot/bin/cc-select。
const BIN = path.resolve(process.cwd(), "../../bin/cc-select");

// 启动一个隔离的 cc-select gui 进程，从 stderr 解析实际端口。
// configPath/providers.json 落 configDir；可选 homeDir 隔离 HOME/USERPROFILE。
async function startServer(opts: { configPath: string; homeDir?: string; stderr?: boolean }) {
  const env: NodeJS.ProcessEnv = { ...process.env, CC_SELECT_CONFIG: opts.configPath };
  if (opts.homeDir) {
    // 隔离 home：shell 集成检测/写入的 rc 落在临时目录，不碰真实 ~/.zshrc。
    env.HOME = opts.homeDir;
    env.USERPROFILE = opts.homeDir;
    env.CC_SELECT_SHELL = "zsh"; // 固定 zsh，使 banner 行为确定
  }
  const proc: ChildProcessWithoutNullStreams = spawn(
    BIN,
    ["gui", "--no-browser", "--port", "0"],
    { env },
  );
  if (opts.stderr) proc.stderr.pipe(process.stderr); // 诊断：转发 server 日志

  const baseURL = await new Promise<string>((resolve, reject) => {
    let buf = "";
    const onData = (d: Buffer) => {
      buf += d.toString();
      const m = buf.match(/http:\/\/127\.0\.0\.1:(\d+)/);
      if (m) {
        proc.stderr.off("data", onData);
        resolve(`http://127.0.0.1:${m[1]}`);
      }
    };
    proc.stderr.on("data", onData);
    proc.once("exit", (code) => reject(new Error(`服务进程提前退出（code=${code}）：${buf}`)));
    setTimeout(() => reject(new Error(`服务启动超时：${buf}`)), 10_000);
  });
  return { proc, baseURL };
}

type ServerFixture = {
  baseURL: string;
  configPath: string;
  configDir: string;
};

// isolatedServer 额外带 homeDir（shell 集成 rc 落点）。
type IsolatedFixture = ServerFixture & { homeDir: string };

type Fixtures = {
  server: ServerFixture;
  isolatedServer: IsolatedFixture;
};

export const test = base.extend<Fixtures>({
  // server：仅隔离 CC_SELECT_CONFIG，行为最接近生产（providers 类用例用）。
  server: async ({}, use) => {
    const configDir = await mkdtemp(path.join(tmpdir(), "ccs-e2e-"));
    const configPath = path.join(configDir, "providers.json");
    const { proc, baseURL } = await startServer({ configPath });
    await use({ baseURL, configPath, configDir });
    proc.kill("SIGTERM");
    await rm(configDir, { recursive: true, force: true });
  },

  // isolatedServer：额外隔离 home + 固定 zsh，供 shell 集成 banner 用例。
  isolatedServer: async ({}, use) => {
    const configDir = await mkdtemp(path.join(tmpdir(), "ccs-e2e-"));
    const configPath = path.join(configDir, "providers.json");
    const homeDir = await mkdtemp(path.join(tmpdir(), "ccs-home-"));
    const { proc, baseURL } = await startServer({ configPath, homeDir });
    await use({ baseURL, configPath, configDir, homeDir });
    proc.kill("SIGTERM");
    await rm(configDir, { recursive: true, force: true });
    await rm(homeDir, { recursive: true, force: true });
  },
});

export { expect } from "@playwright/test";
