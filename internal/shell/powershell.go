package shell

import (
	_ "embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/cc-select/cc-select/internal/i18n"
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
// PowerShell 单引号串内部的单引号转义为两个单引号（''）。
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

// InitSnippet 渲染 PowerShell 版 ccs 函数。
func (PowerShellEmitter) InitSnippet(binaryPath string) string {
	var b strings.Builder
	data := map[string]string{
		"BinaryPath": binaryPath,
		"Comment":    i18n.T("shell.init.powershellComment"),
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
