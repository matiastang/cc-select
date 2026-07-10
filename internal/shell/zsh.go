package shell

import (
	_ "embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/cc-select/cc-select/internal/i18n"
)

// initZshTmpl 是 ccs() 函数体模板（由 //go:embed 嵌入）。
// use 走 eval 注入当前 shell，其余子命令直接转发——满足"ccs 是 cc-select 完整别名"。
var (
	//go:embed init_zsh.tmpl
	initZshTmpl string

	zshTmpl *template.Template
)

func init() {
	zshTmpl = template.Must(template.New("zsh-init").Parse(initZshTmpl))
}

// ZshEmitter 生成 zsh/bash 兼容的 export/unset 语句。
type ZshEmitter struct{}

// Emit 生成可直接 eval 的语句。每个语句独占一行。
// 值用单引号包裹，内部单引号转义为 '\”'——这是 shell 单引号串的标准安全转义，
// 防止 key/URL 中的特殊字符（$、空格、引号、反引号等）被解释。
func (ZshEmitter) Emit(changes []Change) string {
	var b strings.Builder
	for _, c := range changes {
		switch c.Op {
		case OpSet:
			fmt.Fprintf(&b, "export %s=%s\n", c.Name, singleQuote(c.Value))
		case OpUnset:
			fmt.Fprintf(&b, "unset %s\n", c.Name)
		}
	}
	return b.String()
}

// InitSnippet 渲染 ccs() 函数。
func (ZshEmitter) InitSnippet(binaryPath string) string {
	var b strings.Builder
	data := map[string]string{
		"BinaryPath": binaryPath,
		"Comment":    i18n.T("shell.init.zshComment"),
	}
	if err := zshTmpl.Execute(&b, data); err != nil {
		// 模板是静态的，Execute 只在模板语法错时失败，此处不可能。
		return ""
	}
	return b.String()
}

// singleQuote 把任意字符串转成单引号包裹的安全 shell 字面量。
func singleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
