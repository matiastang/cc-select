import js from "@eslint/js";
import tseslint from "typescript-eslint";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import prettierConfig from "eslint-config-prettier";
import prettierPlugin from "eslint-plugin-prettier";

export default tseslint.config(
  js.configs.recommended,
  tseslint.configs.recommended,
  {
    files: ["src/**/*.{ts,tsx}"],
    plugins: {
      "react-hooks": reactHooks,
      "react-refresh": reactRefresh,
      prettier: prettierPlugin,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      "react-refresh/only-export-components": "warn",
      "prettier/prettier": "error",

      // 规模与复杂度门禁（specs/005-quality-gates R12，FR-003~006）。
      // 阈值与 Go 侧对齐（research.md §0/§3 实测基线，棘轮只紧不松）；
      // 维护规则见 docs/quality-gates.md。
      "max-lines": ["error", { max: 500, skipBlankLines: true, skipComments: true }],
      "max-lines-per-function": ["error", { max: 150, skipBlankLines: true, skipComments: true }],
      complexity: ["error", 15],
      "max-depth": ["error", 4],
      "max-params": ["error", 4],
    },
  },
  {
    // 测试文件豁免规模/复杂度类规则（表驱动测试天然长；测试质量由覆盖率门禁约束）。
    // 与 Go 侧 `_test.go` 豁免口径一致（FR-002）。
    files: ["src/**/*.{test,spec}.{ts,tsx}"],
    rules: {
      "max-lines": "off",
      "max-lines-per-function": "off",
      complexity: "off",
      "max-depth": "off",
      "max-params": "off",
    },
  },
  {
    ignores: ["node_modules", "dist", "coverage", "test-results", "../web/assets"],
  },
  prettierConfig,
);
