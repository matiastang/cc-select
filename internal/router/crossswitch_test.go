//go:build integration

// crossswitch_test.go 是 SC-003 的脚本化：双终端交叉热切各 ≥10 次，
// 断言 fake upstream 收到的每笔请求都按各自路由表的当前值归属——串扰为 0。
package router

import (
	"fmt"
	"testing"

	"github.com/cc-select/cc-select/internal/routes"
)

func TestIntegration_CrossSwitchZeroCrosstalk(t *testing.T) {
	glm, minimax := setupTwoProviders(t)
	addr := startDaemon(t)
	tidA, _ := routes.NewTID()
	tidB, _ := routes.NewTID()
	_ = routes.Set(tidA, "glm")
	_ = routes.Set(tidB, "minimax")

	// 期望序列：A/B 交替各发一笔，随后同时交叉切换，再各发一笔；重复 10 轮。
	for i := range 10 {
		// 切换前：按当前路由断言归属。
		code, body := claudePost(t, addr, tidA, "claude-sonnet-5")
		if code != 200 || !contains(body, `"glm"`) {
			t.Fatalf("轮 %d：A 应走 glm（body=%s）", i, body)
		}
		code, body = claudePost(t, addr, tidB, "claude-sonnet-5")
		if code != 200 || !contains(body, `"minimax"`) {
			t.Fatalf("轮 %d：B 应走 minimax（code=%d body=%s）", i, code, body)
		}
		// 交叉切换：A→minimax，B→glm。
		if err := routes.Set(tidA, "minimax"); err != nil {
			t.Fatal(err)
		}
		if err := routes.Set(tidB, "glm"); err != nil {
			t.Fatal(err)
		}
		// 切换后：下一笔立即按新路由。
		if _, body = claudePost(t, addr, tidA, "claude-sonnet-5"); !contains(body, `"minimax"`) {
			t.Fatalf("轮 %d：切换后 A 应走 minimax（body=%s）", i, body)
		}
		if _, body = claudePost(t, addr, tidB, "claude-sonnet-5"); !contains(body, `"glm"`) {
			t.Fatalf("轮 %d：切换后 B 应走 glm（body=%s）", i, body)
		}
		// 换回，进入下一轮。
		_ = routes.Set(tidA, "glm")
		_ = routes.Set(tidB, "minimax")
	}

	// 计数核对：每轮每上游收 2 笔（A 首笔 + B 切后笔）× 10 轮 = 20 笔。
	if len(glm.bodies) != 20 || len(minimax.bodies) != 20 {
		t.Errorf("请求计数不符 SC-003 预期：glm=%d minimax=%d", len(glm.bodies), len(minimax.bodies))
	}
	// 真实 token 归属核验：glm 上游只见过 sk-glm，反之亦然。
	for _, a := range glm.authz {
		if a != "Bearer sk-glm" {
			t.Fatalf("glm 上游收到异样凭证：%q（串扰！）", a)
		}
	}
	for _, a := range minimax.authz {
		if a != "Bearer sk-mm" {
			t.Fatalf("minimax 上游收到异样凭证：%q（串扰！）", a)
		}
	}
	_ = fmt.Sprintf("")
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexOfSub(s, sub) >= 0
}

func indexOfSub(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
