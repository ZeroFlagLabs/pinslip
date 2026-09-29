// repair_test.go — .git 撕裂自愈（add-git-repo-self-heal）：
// 诊断驱动，只对可直接验证的损坏动手；隔离到 .git/corrupt-<时间戳>-<名>
// （ref 必须移出 refs/ 目录）；修复后重试一次；全程不碰工作区。
package gitsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// quarantined 列出 .git/ 根下的隔离文件（corrupt-* 前缀）。
func quarantined(t *testing.T, gitDir string) []string {
	t.Helper()
	entries, err := os.ReadDir(gitDir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "corrupt-") {
			out = append(out, e.Name())
		}
	}
	return out
}

// refsHeadsEntries 列出 refs/heads/ 下的条目名（目录不存在返回空）。
func refsHeadsEntries(t *testing.T, gitDir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(gitDir, "refs", "heads"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

// TestRepairIfBroken 只读诊断的判定标准（design.md 决策 2 的表）：
// 只对可直接验证的损坏动手；unborn / detached / 健康状态一律不碰。
func TestRepairIfBroken(t *testing.T) {
	validHash := strings.Repeat("a", 40)
	tests := []struct {
		name string
		// setup 在新建的空 .git 目录里布置文件
		setup func(t *testing.T, gitDir string)
		// noBranch 以空 branch 调用（HEAD 重置目标不明的场景）
		noBranch     bool
		wantRepaired bool
		check        func(t *testing.T, gitDir string)
	}{
		{
			name:         "空 .git 无任何文件不判损坏",
			setup:        func(t *testing.T, gitDir string) {},
			wantRepaired: false,
		},
		{
			name: "unborn(ref 缺失)不判损坏",
			setup: func(t *testing.T, gitDir string) {
				writeGitFile(t, gitDir, "HEAD", "ref: refs/heads/main\n")
			},
			wantRepaired: false,
			check: func(t *testing.T, gitDir string) {
				if got := readGitFile(t, gitDir, "HEAD"); got != "ref: refs/heads/main\n" {
					t.Fatalf("HEAD 不得改动: %q", got)
				}
			},
		},
		{
			name: "detached HEAD(40 hex)合法",
			setup: func(t *testing.T, gitDir string) {
				writeGitFile(t, gitDir, "HEAD", validHash+"\n")
			},
			wantRepaired: false,
		},
		{
			name: "健康仓库不动",
			setup: func(t *testing.T, gitDir string) {
				writeGitFile(t, gitDir, "HEAD", "ref: refs/heads/main\n")
				writeGitFile(t, gitDir, "refs/heads/main", validHash+"\n")
				writeGitFile(t, gitDir, "index", "DIRC"+strings.Repeat("\x00", 64))
			},
			wantRepaired: false,
		},
		{
			name: "全零 index 判损坏并隔离",
			setup: func(t *testing.T, gitDir string) {
				writeGitFile(t, gitDir, "HEAD", "ref: refs/heads/main\n")
				writeGitFile(t, gitDir, "index", strings.Repeat("\x00", 4096))
			},
			wantRepaired: true,
			check: func(t *testing.T, gitDir string) {
				if _, err := os.Stat(filepath.Join(gitDir, "index")); !os.IsNotExist(err) {
					t.Fatal("损坏 index 原位置应已移除")
				}
				qs := quarantined(t, gitDir)
				if len(qs) != 1 || !strings.HasSuffix(qs[0], "-index") {
					t.Fatalf("应隔离 index 到 .git/ 根: %v", qs)
				}
			},
		},
		{
			name: "空 index(size==0)判损坏",
			setup: func(t *testing.T, gitDir string) {
				writeGitFile(t, gitDir, "HEAD", "ref: refs/heads/main\n")
				writeGitFile(t, gitDir, "index", "")
			},
			wantRepaired: true,
		},
		{
			name: "截断 index(不足 4 字节)判损坏",
			setup: func(t *testing.T, gitDir string) {
				writeGitFile(t, gitDir, "HEAD", "ref: refs/heads/main\n")
				writeGitFile(t, gitDir, "index", "DI")
			},
			wantRepaired: true,
		},
		{
			name: "全零分支引用判损坏并隔离出 refs/",
			setup: func(t *testing.T, gitDir string) {
				writeGitFile(t, gitDir, "HEAD", "ref: refs/heads/main\n")
				writeGitFile(t, gitDir, "refs/heads/main", strings.Repeat("\x00", 4096))
			},
			wantRepaired: true,
			check: func(t *testing.T, gitDir string) {
				if _, err := os.Stat(filepath.Join(gitDir, "refs", "heads", "main")); !os.IsNotExist(err) {
					t.Fatal("损坏 ref 原位置应已移除(HEAD 回 unborn)")
				}
				qs := quarantined(t, gitDir)
				if len(qs) != 1 || !strings.HasSuffix(qs[0], "-main") {
					t.Fatalf("应隔离 ref 到 .git/ 根: %v", qs)
				}
				for _, e := range refsHeadsEntries(t, gitDir) {
					if strings.HasPrefix(e, "corrupt-") {
						t.Fatalf("隔离文件不得留在 refs/heads/ 下: %v", e)
					}
				}
			},
		},
		{
			name: "HEAD 内容非法重置为配置分支",
			setup: func(t *testing.T, gitDir string) {
				writeGitFile(t, gitDir, "HEAD", "\x00\x00garbage")
			},
			wantRepaired: true,
			check: func(t *testing.T, gitDir string) {
				if got := readGitFile(t, gitDir, "HEAD"); got != "ref: refs/heads/main\n" {
					t.Fatalf("HEAD 应重置为配置分支: %q", got)
				}
			},
		},
		{
			name: "HEAD 内容非法但 branch 为空不碰",
			setup: func(t *testing.T, gitDir string) {
				writeGitFile(t, gitDir, "HEAD", "garbage")
			},
			noBranch:     true,
			wantRepaired: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gitDir := filepath.Join(t.TempDir(), ".git")
			if err := os.MkdirAll(gitDir, 0o755); err != nil {
				t.Fatal(err)
			}
			tc.setup(t, gitDir)
			branch := "main"
			if tc.noBranch {
				branch = ""
			}
			if got := repairIfBroken(filepath.Dir(gitDir), branch); got != tc.wantRepaired {
				t.Fatalf("repairIfBroken = %v, want %v", got, tc.wantRepaired)
			}
			if !tc.wantRepaired {
				if qs := quarantined(t, gitDir); len(qs) != 0 {
					t.Fatalf("未判损坏时不得产生隔离文件: %v", qs)
				}
			}
			if tc.check != nil {
				tc.check(t, gitDir)
			}
		})
	}
}

func writeGitFile(t *testing.T, gitDir, rel, content string) {
	t.Helper()
	abs := filepath.Join(gitDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readGitFile(t *testing.T, gitDir, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(gitDir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestCommitAllHealsTornIndex 全零 index：CommitAll 先败后自愈成功，
// 隔离文件在 .git/ 根，工作区文件一字节不动。
func TestCommitAllHealsTornIndex(t *testing.T) {
	vault := newVault(t)
	remote := newBareRemote(t)
	writeVaultFile(t, vault, "notes/a.md", []byte("# 标题\n内容\n"))
	r := connectVault(t, vault, remote)

	before := readVaultFile(t, vault, "notes/a.md")
	gitDir := filepath.Join(vault, ".git")
	if err := os.WriteFile(filepath.Join(gitDir, "index"), make([]byte, 4096), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := r.CommitAll(); err != nil {
		t.Fatalf("自愈后 CommitAll 应成功: %v", err)
	}
	qs := quarantined(t, gitDir)
	if len(qs) != 1 || !strings.HasSuffix(qs[0], "-index") {
		t.Fatalf("应隔离撕裂的 index 到 .git/ 根: %v", qs)
	}
	if got := readVaultFile(t, vault, "notes/a.md"); got != before {
		t.Fatalf("工作区文件不得改动:\n got %q\nwant %q", got, before)
	}
	// 自愈后索引已由本轮提交重建，仓库恢复可用
	gr, err := git.PlainOpen(vault)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gr.Head(); err != nil {
		t.Fatalf("自愈后 HEAD 应可解析: %v", err)
	}
}

// TestCommitAllHealsTornBranchRef 全零 refs/heads/main：Connect（标记在
// 工作区，不解析 HEAD）成功；CommitAll 解析 HEAD 失败 → 隔离 ref（HEAD 回
// unborn）→ 首轮提交从工作区重建成功。
func TestCommitAllHealsTornBranchRef(t *testing.T) {
	vault := newVault(t)
	remote := newBareRemote(t)
	writeVaultFile(t, vault, "notes/a.md", []byte("# 标题\n内容\n"))
	connectVault(t, vault, remote)

	gitDir := filepath.Join(vault, ".git")
	refPath := filepath.Join(gitDir, "refs", "heads", "main")
	if err := os.WriteFile(refPath, make([]byte, 4096), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Connect(vault, testConfig(remote))
	if err != nil {
		t.Fatalf("Connect 不应失败（标记在工作区，不解析 HEAD）: %v", err)
	}
	hash, _, err := r.CommitAll()
	if err != nil {
		t.Fatalf("自愈后首轮提交应成功: %v", err)
	}
	if hash == plumbing.ZeroHash {
		t.Fatal("首轮提交应产生新提交")
	}
	qs := quarantined(t, gitDir)
	if len(qs) != 1 || !strings.HasSuffix(qs[0], "-main") {
		t.Fatalf("应隔离撕裂的 ref 到 .git/ 根: %v", qs)
	}
	for _, e := range refsHeadsEntries(t, gitDir) {
		if strings.HasPrefix(e, "corrupt-") {
			t.Fatalf("隔离文件不得留在 refs/heads/ 下: %v", e)
		}
	}
	// ref 被移除后由首轮提交重建为合法 hash（unborn → born）
	if got := strings.TrimSpace(readGitFile(t, gitDir, "refs/heads/main")); !hash40Re.MatchString(got) {
		t.Fatalf("首轮提交后 ref 应为合法 hash: %q", got)
	}
	if got := readVaultFile(t, vault, "notes/a.md"); got != "# 标题\n内容\n" {
		t.Fatalf("工作区文件不得改动: %q", got)
	}
}

// TestConnectUnrepairableErrorNoQuarantine 健康布局 + 不可修复错误（ref 是
// 合法 40 hex 但对象缺失）：诊断全绿 → 零隔离文件，原错误原样返回。
func TestConnectUnrepairableErrorNoQuarantine(t *testing.T) {
	vault := newVault(t)
	gr, err := git.PlainInit(vault, false)
	if err != nil {
		t.Fatalf("初始化仓库失败: %v", err)
	}
	// 格式合法（40 hex）但指向不存在的对象：诊断判不出损坏，不属于自愈范围。
	// 注意用 master：PlainInit 默认 HEAD 指向 refs/heads/master。
	bogus := plumbing.NewHash(strings.Repeat("deadbeef", 5))
	ref := plumbing.NewHashReference(plumbing.NewBranchReferenceName("master"), bogus)
	if err := gr.Storer.SetReference(ref); err != nil {
		t.Fatalf("写入引用失败: %v", err)
	}

	_, err = Connect(vault, testConfig(filepath.Join(t.TempDir(), "remote.git")))
	if err == nil {
		t.Fatal("对象缺失时 Connect 应失败")
	}
	if !strings.Contains(err.Error(), "读取提交历史失败") {
		t.Fatalf("原错误应原样返回, got: %v", err)
	}
	if qs := quarantined(t, filepath.Join(vault, ".git")); len(qs) != 0 {
		t.Fatalf("不可修复错误不得产生隔离文件: %v", qs)
	}
	if got := strings.TrimSpace(readGitFile(t, filepath.Join(vault, ".git"), "refs/heads/master")); got != bogus.String() {
		t.Fatalf("合法格式的 ref 不得被改动: %q", got)
	}
}
