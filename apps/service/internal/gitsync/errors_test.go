package gitsync

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

func TestWithCodeRoundTrip(t *testing.T) {
	base := errors.New("boom")
	err := withCode(CodeSyncPushFailed, base)
	if got := codeOf(err); got != CodeSyncPushFailed {
		t.Fatalf("codeOf = %q, want %q", got, CodeSyncPushFailed)
	}
	if !errors.Is(err, base) {
		t.Fatal("errors.Is should unwrap to the base error")
	}
	if err.Error() != "boom" {
		t.Fatalf("Error() = %q, want original message", err.Error())
	}
	if codeOf(base) != "" {
		t.Fatal("plain error should have no code")
	}
}

func TestWithCodeKeepsInnerCode(t *testing.T) {
	inner := withCode(CodeSyncRemoteAccess, errors.New("denied"))
	err := withCode(CodeSyncPullFailed, inner)
	if got := codeOf(err); got != CodeSyncRemoteAccess {
		t.Fatalf("codeOf = %q, want inner %q", got, CodeSyncRemoteAccess)
	}
	if got := codeOr(err, CodeSyncPullFailed); got != CodeSyncRemoteAccess {
		t.Fatalf("codeOr = %q, want inner %q", got, CodeSyncRemoteAccess)
	}
	if got := codeOr(fmt.Errorf("wrap: %w", errors.New("x")), CodeSyncPullFailed); got != CodeSyncPullFailed {
		t.Fatalf("codeOr = %q, want fallback %q", got, CodeSyncPullFailed)
	}
}

func TestSyncWriteErrorIncludesCode(t *testing.T) {
	rec := httptest.NewRecorder()
	syncWriteError(rec, 400, withCode(CodeSyncURLRequired, errors.New("启用同步必须提供仓库地址 url")))
	body := rec.Body.String()
	if !strings.Contains(body, `"code":"`+CodeSyncURLRequired+`"`) {
		t.Fatalf("body missing code: %s", body)
	}
	if !strings.Contains(body, "启用同步必须提供仓库地址 url") {
		t.Fatalf("body missing message: %s", body)
	}

	rec2 := httptest.NewRecorder()
	syncWriteError(rec2, 500, errors.New("plain"))
	if strings.Contains(rec2.Body.String(), `"code"`) {
		t.Fatalf("plain error should not carry code: %s", rec2.Body.String())
	}
}

func TestConnectEmptyURLCode(t *testing.T) {
	_, err := Connect(newVault(t), SyncConfig{Enabled: true, URL: ""})
	if err == nil {
		t.Fatal("Connect with empty URL should fail")
	}
	if got := codeOf(err); got != CodeSyncURLRequired {
		t.Fatalf("codeOf = %q, want %q", got, CodeSyncURLRequired)
	}
}

func TestConnectUnreachableRemoteCode(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "not-exist", "remote.git")
	_, err := Connect(newVault(t), testConfig(missing))
	if err == nil {
		t.Fatal("Connect should fail for unreachable remote")
	}
	if got := codeOf(err); got != CodeSyncRemoteAccess {
		t.Fatalf("codeOf = %q, want %q", got, CodeSyncRemoteAccess)
	}
}

// TestConnectLocalPermissionCode 权限错误（EACCES/EPERM）必须归类为
// SYNC_LOCAL_PERMISSION，而非误报 SYNC_LOCAL_NOT_PINSLIP_REPO 或落入兜底码。
// Windows 上 chmod 0o000 不产生 EACCES（NTFS 权限不映射 POSIX 模式位），
// 故经 osStat/osWriteFile 替换点注入权限错误，双平台行为一致。
func TestConnectLocalPermissionCode(t *testing.T) {
	permErr := &os.PathError{Op: "mock", Err: os.ErrPermission} // 贴近 os.Stat/WriteFile 真实返回

	cases := []struct {
		name      string
		setup     func(t *testing.T, vault string)
		failStat  func(path string) bool // 命中时 osStat 注入权限错误
		failWrite func(path string) bool // 命中时 osWriteFile 注入权限错误
	}{
		{
			name:     "stat .git 权限拒绝",
			setup:    func(t *testing.T, vault string) {},
			failStat: func(p string) bool { return filepath.Base(p) == ".git" },
		},
		{
			name: "标记文件 stat 权限拒绝",
			setup: func(t *testing.T, vault string) {
				if _, err := git.PlainInit(vault, false); err != nil {
					t.Fatalf("初始化仓库失败: %v", err)
				}
			},
			failStat: func(p string) bool { return filepath.Base(p) == markerFile },
		},
		{
			name: "补齐 meta 文件写入权限拒绝",
			setup: func(t *testing.T, vault string) {
				if _, err := git.PlainInit(vault, false); err != nil {
					t.Fatalf("初始化仓库失败: %v", err)
				}
				writeVaultFile(t, vault, markerFile, []byte(markerContent))
			},
			failWrite: func(p string) bool { return filepath.Base(p) == ".gitignore" },
		},
		{
			name: "标记文件写入权限拒绝",
			setup: func(t *testing.T, vault string) {
				if _, err := git.PlainInit(vault, false); err != nil {
					t.Fatalf("初始化仓库失败: %v", err)
				}
				writeVaultFile(t, vault, markerFile, []byte(markerContent))
			},
			failWrite: func(p string) bool { return filepath.Base(p) == markerFile },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vault := newVault(t)
			tc.setup(t, vault)

			origStat, origWrite := osStat, osWriteFile
			osStat = func(name string) (os.FileInfo, error) {
				if tc.failStat != nil && tc.failStat(name) {
					return nil, permErr
				}
				return origStat(name)
			}
			osWriteFile = func(name string, data []byte, perm os.FileMode) error {
				if tc.failWrite != nil && tc.failWrite(name) {
					return permErr
				}
				return origWrite(name, data, perm)
			}
			t.Cleanup(func() { osStat, osWriteFile = origStat, origWrite })

			_, err := Connect(vault, testConfig(filepath.Join(t.TempDir(), "remote.git")))
			if err == nil {
				t.Fatal("Connect should fail on permission error")
			}
			if got := codeOf(err); got != CodeSyncLocalPermission {
				t.Fatalf("codeOf = %q, want %q (err: %v)", got, CodeSyncLocalPermission, err)
			}
		})
	}
}

// TestConnectMarkerHistoryReadError markerInHistory 读历史失败（如 .git 内
// 对象不可读）必须上报错误，不能吞成 false 误报 SYNC_LOCAL_NOT_PINSLIP_REPO。
func TestConnectMarkerHistoryReadError(t *testing.T) {
	vault := newVault(t)
	gr, err := git.PlainInit(vault, false)
	if err != nil {
		t.Fatalf("初始化仓库失败: %v", err)
	}
	// HEAD 指向不存在的提交：Head() 成功、CommitObject 失败（跨平台稳定，
	// 不依赖文件权限位）。
	bogus := plumbing.NewHash(strings.Repeat("deadbeef", 5))
	ref := plumbing.NewHashReference(plumbing.NewBranchReferenceName("master"), bogus)
	if err := gr.Storer.SetReference(ref); err != nil {
		t.Fatalf("写入引用失败: %v", err)
	}

	_, err = Connect(vault, testConfig(filepath.Join(t.TempDir(), "remote.git")))
	if err == nil {
		t.Fatal("Connect should fail when history is unreadable")
	}
	if got := codeOf(err); got == CodeSyncLocalNotPinslipRepo {
		t.Fatalf("history read failure must not be misreported as %q", CodeSyncLocalNotPinslipRepo)
	}
	if !strings.Contains(err.Error(), "读取提交历史失败") {
		t.Fatalf("error should mention history read failure, got: %v", err)
	}
}
