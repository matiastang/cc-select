import { test as base } from "@playwright/test";
import { spawn, type ChildProcessWithoutNullStreams } from "child_process";
import { mkdtemp, rm } from "fs/promises";
import { tmpdir } from "os";
import path from "path";

// server fixture：每个测试独占一个真实 cc-select 二进制进程。
// - CC_SELECT_CONFIG 指向独立临时目录，互不污染、可并行；
// - profiles 落在 configDir/profiles/<id>/settings.json（与生产布局一致），
//   便于"手改文件"类用例直接操作磁盘；
// - 用 --port 0 让系统分配端口，从 stderr 解析实际端口，避免端口冲突。
type ServerFixture = {
  baseURL: string;
  configPath: string;
  configDir: string;
};

type Fixtures = {
  server: ServerFixture;
};

// 二进制相对 cwd（internal/frontend）的位置：repoRoot/bin/cc-select。
const BIN = path.resolve(process.cwd(), "../../bin/cc-select");

export const test = base.extend<Fixtures>({
  server: async ({}, use) => {
    const configDir = await mkdtemp(path.join(tmpdir(), "ccs-e2e-"));
    const configPath = path.join(configDir, "providers.json");

    const proc: ChildProcessWithoutNullStreams = spawn(
      BIN,
      ["gui", "--no-browser", "--port", "0"],
      { env: { ...process.env, CC_SELECT_CONFIG: configPath } },
    );

    // 从 stderr 的"已启动：http://127.0.0.1:PORT"提取实际端口。
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

    await use({ baseURL, configPath, configDir });

    proc.kill("SIGTERM");
    await rm(configDir, { recursive: true, force: true });
  },
});

export { expect } from "@playwright/test";
