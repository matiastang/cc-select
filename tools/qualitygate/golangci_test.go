package main

import (
	"strings"
	"testing"
)

func TestParseGolangciOutput(t *testing.T) {
	t.Parallel()

	input := `internal/cli/add.go:211:1: calculated cyclomatic complexity for function readProviderInput is 26, max is 15 (cyclop)
func readProviderInput(r *bufio.Reader, out io.Writer, fl addFlags, id string, presetID string) (addFlags, error) {
^
internal/router/server.go:148:1: The line is 212 characters long, which exceeds the maximum of 160 characters. (lll)
	if _, err := fmt.Fprintf(w, "x")
^
desktop/bridge_test.go:20:2: field reply is unused (unused)
	type bridgeReply struct {
^
internal/shell/powershell.go:1: 1-71 lines are duplicate of ` + "`internal/shell/zsh.go:1-71`" + ` (dupl)
package shell
^
2 issues:
* cyclop: 1
* lll: 1
`

	findings, err := parseGolangciOutput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parseGolangciOutput: %v", err)
	}
	if len(findings) != 4 {
		t.Fatalf("findings = %d, want 4", len(findings))
	}

	first := findings[0]
	if first.File != "internal/cli/add.go" || first.Line != 211 || first.Rule != "cyclop" {
		t.Fatalf("first = %+v", first)
	}
	if !strings.HasPrefix(first.Message, "calculated cyclomatic complexity for function readProviderInput") {
		t.Fatalf("first.message = %q", first.Message)
	}

	third := findings[2]
	if third.File != "desktop/bridge_test.go" || third.Line != 20 || third.Rule != "unused" {
		t.Fatalf("third = %+v", third)
	}

	// dupl 输出没有列号（file:line: 而非 file:line:col:）——回归锁定。
	fourth := findings[3]
	if fourth.File != "internal/shell/powershell.go" || fourth.Line != 1 || fourth.Rule != "dupl" {
		t.Fatalf("fourth = %+v", fourth)
	}
}

func TestParseGolangciOutput_EmptyAndGarbage(t *testing.T) {
	t.Parallel()

	t.Run("空输出（干净通过）零 findings", func(t *testing.T) {
		t.Parallel()
		findings, err := parseGolangciOutput(strings.NewReader(""))
		if err != nil || len(findings) != 0 {
			t.Fatalf("err=%v findings=%d, want 0/nil", err, len(findings))
		}
	})

	t.Run("摘要行与上下文行被忽略", func(t *testing.T) {
		t.Parallel()
		input := "3 issues:\n* lll: 2\n\tindented context\n^ caret line\n"
		findings, err := parseGolangciOutput(strings.NewReader(input))
		if err != nil || len(findings) != 0 {
			t.Fatalf("err=%v findings=%d, want 0/nil", err, len(findings))
		}
	})

	t.Run("非 Go 路径行被忽略", func(t *testing.T) {
		t.Parallel()
		input := "some/random text without findings\n"
		if _, err := parseGolangciOutput(strings.NewReader(input)); err != nil {
			t.Fatalf("垃圾行不应报错: %v", err)
		}
	})
}
