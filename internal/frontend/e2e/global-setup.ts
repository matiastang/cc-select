import { execSync } from "child_process";
import path from "path";

// globalSetup：在跑任何 e2e 之前，构建最新前端 assets + 含这些 assets 的二进制。
// 这样 e2e 测的永远是当前代码（embed 的前端 + 真后端），而非某个陈旧的 bin。
//
// 执行时 cwd = playwright 配置所在目录（internal/frontend）。
export default async function globalSetup() {
  const frontend = process.cwd(); // internal/frontend
  const repoRoot = path.resolve(frontend, "../.."); // 仓库根

  // 1) 构建前端，产物输出到 ../web/assets（被 Go embed）。
  execSync("npm run build", { cwd: frontend, stdio: "inherit" });
  // 2) 构建含最新 assets 的二进制到 bin/cc-select。
  execSync("go build -o bin/cc-select .", { cwd: repoRoot, stdio: "inherit" });
}
