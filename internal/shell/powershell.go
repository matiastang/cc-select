package shell

import (
	_ "embed"
	"fmt"
	"strings"
	"text/template"
)

// PowerShellEmitter 生成 PowerShell 语句：$env:NAME='val' / Remove-Item Env:\NAME。
// 由 Invoke-Expression (iex) 执行——PowerShell 的 eval 等价物。
// 详见 docs/windows-support.md §3。
var (
	//go:embed init_powershell.tmpl
	initPwshTmpl string

	pwshTmpl *template.Template
)

func init() {
	pwshTmpl = template.Must(template.New("pwsh-init").Parse(initPwshTmpl))
}

// PowerShellEmitter 是 Windows PowerShell 的 Emitter。
type PowerShellEmitter struct{}

// Emit 生成可被 Invoke-Expression 执行的语句。
// PowerShell 单引号串内部的单引号转义为两个单引号（”）。
func (PowerShellEmitter) Emit(changes []Change) string {
	var b strings.Builder
	for _, c := range changes {
		switch c.Op {
		case OpSet:
			fmt.Fprintf(&b, "$env:%s = %s\n", c.Name, pwshSingleQuote(c.Value))
		case OpUnset:
			// -ErrorAction SilentlyContinue：变量不存在时不报错（幂等清理）。
			fmt.Fprintf(&b, "Remove-Item Env:\\%s -ErrorAction SilentlyContinue\n", c.Name)
		}
	}
	return b.String()
}

// pwshInitComment 刻意保持 ASCII-only（英文）。cc-select init 的输出会被 shell 重定向
// 捕获（>> $PROFILE）。Windows PowerShell 5.1 按控制台代码页（中文系统=GBK）解码外部
// 命令 stdout，本地化/CJK 注释会被错位解码、吞掉换行，破坏生成的 $PROFILE 语法。
// 生成代码的注释必须与 locale 无关。见 docs/windows-support.md。
const pwshInitComment = "# ccs is the short alias for cc-select: use is injected via Invoke-Expression, other subcommands are forwarded directly."

// InitSnippet 渲染 PowerShell 版 ccs 函数。
func (PowerShellEmitter) InitSnippet(binaryPath string) string {
	var b strings.Builder
	data := map[string]string{
		"BinaryPath": binaryPath,
		"Comment":    pwshInitComment,
	}
	if err := pwshTmpl.Execute(&b, data); err != nil {
		return ""
	}
	return b.String()
}

// pwshSingleQuote 把字符串转成 PowerShell 单引号字面量。
func pwshSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
