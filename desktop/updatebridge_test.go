package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wailsapp/wails/v3/pkg/updater"
)

// T020（specs/003-desktop-client/tasks.md）：更新桥契约（contracts/desktop-api.md §2）。
// check 与 web 端点同构（前端 CheckUpdateButton 零改动）；POST 202 语义 →
// 实测映射为 web 同构的 200+restartRequired（前端渲染「重启生效」卡片）；
// 并发锁 409；校验失败 422；其他 500；check 失败温和降级不 5xx。

type fakeUpdater struct {
	release *updater.Release
	checkIn error
	instErr error
	block   chan struct{} // 非nil时 DownloadAndInstall 阻塞直到关闭
	instN   int
}

func (f *fakeUpdater) Check(context.Context) (*updater.Release, error) {
	return f.release, f.checkIn
}
func (f *fakeUpdater) DownloadAndInstall(context.Context) error {
	f.instN++
	if f.block != nil {
		<-f.block
	}
	return f.instErr
}

func newRel(version string) *updater.Release {
	return &updater.Release{
		Version: version,
		Notes:   "notes-0.2.0",
		Artifact: updater.Artifact{
			Filename: "cc-select-desktop_0.2.0_darwin_arm64.zip",
		},
	}
}

func doReq(h http.Handler, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestUpdateBridge_CheckHasUpdate(t *testing.T) {
	h := newUpdateBridge("0.1.0", &fakeUpdater{release: newRel("0.2.0")})
	rec := doReq(h, http.MethodGet, "/api/v1/update/check")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var out map[string]any
	json.NewDecoder(rec.Body).Decode(&out)
	if out["currentVersion"] != "0.1.0" || out["latestVersion"] != "0.2.0" || out["hasUpdate"] != true {
		t.Fatalf("body = %v", out)
	}
	if out["assetName"] != "cc-select-desktop_0.2.0_darwin_arm64.zip" || out["releaseNotes"] != "notes-0.2.0" {
		t.Fatalf("asset/notes mapping wrong: %v", out)
	}
}

func TestUpdateBridge_CheckUpToDate(t *testing.T) {
	h := newUpdateBridge("0.1.0", &fakeUpdater{release: nil})
	rec := doReq(h, http.MethodGet, "/api/v1/update/check")
	var out map[string]any
	json.NewDecoder(rec.Body).Decode(&out)
	if out["hasUpdate"] != false || out["latestVersion"] != "0.1.0" {
		t.Fatalf("up-to-date mapping wrong: %v", out)
	}
}

func TestUpdateBridge_CheckFailureDegrades(t *testing.T) {
	h := newUpdateBridge("0.1.0", &fakeUpdater{checkIn: context.DeadlineExceeded})
	rec := doReq(h, http.MethodGet, "/api/v1/update/check")
	if rec.Code != http.StatusOK {
		t.Fatalf("check 失败应温和降级 200, got %d", rec.Code)
	}
	var out map[string]any
	json.NewDecoder(rec.Body).Decode(&out)
	if out["hasUpdate"] != false {
		t.Fatalf("降级响应 hasUpdate 应为 false: %v", out)
	}
}

func TestUpdateBridge_InstallSuccess(t *testing.T) {
	f := &fakeUpdater{release: newRel("0.2.0")}
	h := newUpdateBridge("0.1.0", f)
	rec := doReq(h, http.MethodPost, "/api/v1/update")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	json.NewDecoder(rec.Body).Decode(&out)
	if out["status"] != "installed" || out["fromVersion"] != "0.1.0" || out["toVersion"] != "0.2.0" {
		t.Fatalf("body = %v", out)
	}
	if out["restartRequired"] != true {
		t.Fatalf("restartRequired 必须为 true（前端渲染重启卡片）: %v", out)
	}
	if f.instN != 1 {
		t.Fatalf("DownloadAndInstall 调用 %d 次", f.instN)
	}
}

func TestUpdateBridge_InstallLocked409(t *testing.T) {
	f := &fakeUpdater{block: make(chan struct{})}
	defer close(f.block)
	h := newUpdateBridge("0.1.0", f)
	go func() { _ = doReq(h, http.MethodPost, "/api/v1/update") }()
	// 等第一个请求确实进入安装（阻塞中）再发第二个。
	for i := 0; i < 100 && f.instN == 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if f.instN == 0 {
		t.Fatal("第一个安装请求未进入 DownloadAndInstall")
	}
	rec2 := doReq(h, http.MethodPost, "/api/v1/update")
	if rec2.Code != http.StatusConflict {
		t.Fatalf("并发安装应 409, got %d", rec2.Code)
	}
}

func TestUpdateBridge_InstallChecksum422(t *testing.T) {
	h := newUpdateBridge("0.1.0", &fakeUpdater{instErr: errString("checksum mismatch: SHA256SUMS")})
	rec := doReq(h, http.MethodPost, "/api/v1/update")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("checksum 失败应 422, got %d", rec.Code)
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func TestUpdateBridge_InstallOtherError500(t *testing.T) {
	h := newUpdateBridge("0.1.0", &fakeUpdater{instErr: errString("network down")})
	rec := doReq(h, http.MethodPost, "/api/v1/update")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("其他错误应 500, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "network down") {
		t.Fatalf("错误信息应透传: %s", rec.Body.String())
	}
}

func TestUpdateBridge_MethodGuards(t *testing.T) {
	h := newUpdateBridge("0.1.0", &fakeUpdater{})
	if rec := doReq(h, http.MethodPost, "/api/v1/update/check"); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST check = %d, want 405", rec.Code)
	}
	if rec := doReq(h, http.MethodGet, "/api/v1/update"); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET update = %d, want 405", rec.Code)
	}
}
