import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// 构建产物输出到 ../web/assets，由 Go //go:embed 打进二进制。
// 因此 base 设为 "./"，确保资源用相对路径（不依赖部署域名）。
export default defineConfig({
  plugins: [react()],
  base: "./",
  build: {
    outDir: "../web/assets",
    emptyOutDir: true,
  },
  server: {
    // 开发时把 /api 代理到本地 cc-select gui 服务（默认端口 7799）。
    proxy: {
      "/api": "http://127.0.0.1:7799",
    },
  },
});
