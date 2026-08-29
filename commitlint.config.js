// commit message 规范：Conventional Commints（type 可选 scope，如 feat(cli): ...）。
// 由 lefthook 的 commit-msg hook 调用（见 lefthook.yml）；本地与团队统一约束。
// 中文 subject 可直接通过（subject-case 规则不影响 CJK）。
module.exports = {
  extends: ["@commitlint/config-conventional"],
};
