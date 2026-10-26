import type { Config } from "tailwindcss";
import animate from "tailwindcss-animate";

// 设计 token 真值在 src/style.css 的 CSS 变量层（contracts/frontend §2）：
// 组件只消费这里的语义映射（bg-card / text-muted / ...），深浅色由变量翻转实现。
// darkMode 兼顾 data-theme 属性与 class 两种挂载（系统跟随态由变量层自处理）。
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  darkMode: ["class", '[data-theme="dark"]'],
  theme: {
    extend: {
      colors: {
        bg: "var(--bg)",
        card: "var(--card)",
        border: "var(--border)",
        text: "var(--text)",
        muted: "var(--muted)",
        accent: {
          DEFAULT: "var(--accent)",
          hover: "var(--accent-hover)",
          contrast: "var(--accent-contrast)",
        },
        danger: {
          DEFAULT: "var(--danger)",
          hover: "var(--danger-hover)",
          contrast: "var(--danger-contrast)",
        },
        input: "var(--input-bg)",
        notice: "var(--notice-bg)",
        "notice-danger": "var(--notice-danger-bg)",
      },
      radius: {
        sm: "var(--radius-sm)",
        md: "var(--radius-md)",
        lg: "var(--radius-lg)",
      },
      boxShadow: {
        sm: "var(--shadow-sm)",
        md: "var(--shadow-md)",
      },
      fontFamily: {
        sans: ["var(--font-sans)", "system-ui", "sans-serif"],
        mono: ["var(--font-mono)", "ui-monospace", "monospace"],
      },
    },
  },
  plugins: [animate],
} satisfies Config;
