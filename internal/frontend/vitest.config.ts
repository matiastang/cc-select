import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

// 组件单测配置（与 vite.config.ts 的 build 配置分离，互不干扰）。
// e2e 仍由 playwright.config.ts 驱动真实浏览器。
export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test-setup.ts"],
    include: ["src/**/*.{test,spec}.{ts,tsx}"],
    coverage: {
      provider: "v8",
      reporter: ["text", "html"],
      // 全量 src（修复 R11 遗留的收窄：原 include 仅单文件，覆盖率恒为全 0，
      // 门禁因此失真——specs/005-quality-gates research §0.2）。
      include: ["src/**/*.{ts,tsx}"],
      exclude: ["src/**/*.{test,spec}.{ts,tsx}", "src/test-setup.ts", "src/main.tsx"],
      // 覆盖率棘轮（specs/005-quality-gates FR-011）：当前实测基线向下取整起步，
      // 只升不降。维护规则见 docs/quality-gates.md。
      thresholds: { statements: 79, branches: 80, functions: 77, lines: 79 },
    },
  },
});
