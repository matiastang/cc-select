package updater

// defaultReplacer 返回当前 OS 的 Replacer 实现。
// 具体实现按平台拆分：
//   - replace_unix.go    //go:build !windows  （temp + os.Rename 原子替换）
//   - replace_windows.go //go:build windows   （移植 install.ps1 的 .old rename dance）
