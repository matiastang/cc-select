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
      include: ["src/ShellIntegrationBanner.tsx"],
      exclude: ["src/**/*.{test,spec}.{ts,tsx}", "src/test-setup.ts", "src/main.tsx"],
    },
  },
});
