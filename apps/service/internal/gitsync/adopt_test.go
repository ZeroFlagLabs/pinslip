// adopt_test.go — 本地认领（add-sync-local-adoption）：
// vault 已是用户自建 git 仓库（无 .pinslip-repo 标记）时，守卫默认拒绝接入；
// 用户显式确认（adopt=true）后创建标记并提交，完成接入。
package gitsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// newForeignRepo 把 vault 造成「用户自建仓库」：已 git init、有一次提交
//（含自定义 .gitignore 与内容文件）、另留一个未提交变更；无同步标记。
func newForeignRepo(t *testing.T, vault string) {
	t.Helper()
	gr, err := git.PlainInit(vault, false)
	if err != nil {
		t.Fatalf("git init 失败: %v", err)
	}
	writeVaultFile(t, vault, ".gitignore", []byte("node_modules/\n*.log\n"))
	writeVaultFile(t, vault, "notes/existing.md", []byte("用户已有内容\n"))
	w, err := gr.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Add(".gitignore"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Add("notes/existing.md"); err != nil {
		t.Fatal(err)
	}
	sig := &object.Signature{Name: "User", Email: "user@example.com", When: time.Now()}
	if _, err := w.Commit("user init", &git.CommitOptions{Author: sig}); err != nil {
		t.Fatalf("用户首次提交失败: %v", err)
	}
	// 未提交变更：认领时会被 CommitAll 一并带走（与首次接入空远端语义一致）
	writeVaultFile(t, vault, "notes/dirty.md", []byte("尚未提交\n"))
}

// markerCommitted 报告 HEAD 提交树里是否已有标记文件。
func markerCommitted(t *testing.T, vault string) bool {
	t.Helper()
	gr, err := git.PlainOpen(vault)
	if err != nil {
		t.Fatal(err)
	}
	head, err := gr.Head()
	if err != nil {
		t.Fatalf("读取 HEAD 失败: %v", err)
	}
	c, err := gr.CommitObject(head.Hash())
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.File(markerFile)
	return err == nil
}

func TestAdoptLocalRepo(t *testing.T) {
	tests := []struct {
		name  string
		adopt bool
		// check 仅在认领成功路径后运行
		check func(t *testing.T, vault string)
	}{
		{
			name:  "不带 adopt 守卫不削弱",
			adopt: false,
		},
		{
			name:  "adopt 认领成功且标记已提交",
			adopt: true,
			check: func(t *testing.T, vault string) {
				if _, err := os.Stat(filepath.Join(vault, markerFile)); err != nil {
					t.Fatalf("工作区应有标记文件: %v", err)
				}
				if !markerCommitted(t, vault) {
					t.Fatal("标记文件应已提交进 HEAD")
				}
				// 未提交变更被 CommitAll 一并带走：工作区应收干净
				r, err := git.PlainOpen(vault)
				if err != nil {
					t.Fatal(err)
				}
				w, err := r.Worktree()
				if err != nil {
					t.Fatal(err)
				}
				st, err := w.Status()
				if err != nil {
					t.Fatal(err)
				}
				if !st.IsClean() {
					t.Fatalf("认领后工作区应收干净: %v", st)
				}
				// adopt 是一次性标志：不落盘，JSON 里连 key 都不能有
				//（注意只匹配带引号的 key：临时目录路径本身可能含 adopt 字样）
				data, err := os.ReadFile(configPath(vault))
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(data), `"adopt"`) {
					t.Fatalf("adopt 不应写入 git-sync.json: %s", data)
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vault := newVault(t)
			newForeignRepo(t, vault)
			eng, err := NewEngine(vault, tLogger{t})
			if err != nil {
				t.Fatal(err)
			}

			cfg := testConfig(filepath.Join(t.TempDir(), "unused-remote.git"))
			cfg.Adopt = tc.adopt
			err = eng.Reconfigure(cfg)

			if !tc.adopt {
				if codeOf(err) != CodeSyncLocalNotPinslipRepo {
					t.Fatalf("codeOf = %q, want %q (err: %v)", codeOf(err), CodeSyncLocalNotPinslipRepo, err)
				}
				if _, serr := os.Stat(filepath.Join(vault, markerFile)); !os.IsNotExist(serr) {
					t.Fatal("守卫拒绝时不得创建标记文件")
				}
				return
			}
			if err != nil {
				t.Fatalf("adopt 认领应接入成功: %v", err)
			}
			defer eng.Stop()
			if tc.check != nil {
				tc.check(t, vault)
			}
		})
	}
}

// .gitignore 合并语义：不覆盖用户已有内容，只追加缺失的必需条目。
func TestEnsureGitignoreMerge(t *testing.T) {
	tests := []struct {
		name     string
		existing *string // nil = 文件不存在
		want     func(t *testing.T, got, original string)
	}{
		{
			name:     "不存在则创建标准内容",
			existing: nil,
			want: func(t *testing.T, got, _ string) {
				if got != gitignoreContent {
					t.Fatalf("新建内容不符:\n%q", got)
				}
			},
		},
		{
			name: "已存在则追加缺失条目且原内容不动",
			existing: func() *string {
				s := "node_modules/\n*.log" // 故意不带结尾换行
				return &s
			}(),
			want: func(t *testing.T, got, original string) {
				if !strings.HasPrefix(got, original) {
					t.Fatalf("原有内容必须原样保留为前缀:\n%q", got)
				}
				for _, entry := range strings.Fields(gitignoreContent) {
					if !strings.Contains(got, "\n"+entry+"\n") && !strings.HasPrefix(got, entry+"\n") {
						t.Fatalf("缺失必需条目 %q:\n%q", entry, got)
					}
				}
				if !strings.Contains(got, "node_modules/") || !strings.Contains(got, "*.log") {
					t.Fatalf("用户条目丢失:\n%q", got)
				}
			},
		},
		{
			name: "条目齐全时 no-op",
			existing: func() *string {
				s := "custom/\n" + gitignoreContent
				return &s
			}(),
			want: func(t *testing.T, got, original string) {
				if got != original {
					t.Fatalf("条目齐全应一字节不动:\n got %q\nwant %q", got, original)
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			var original string
			if tc.existing != nil {
				original = *tc.existing
				if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(original), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := ensureGitignore(dir); err != nil {
				t.Fatalf("ensureGitignore: %v", err)
			}
			data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
			if err != nil {
				t.Fatal(err)
			}
			tc.want(t, string(data), original)
		})
	}
}
