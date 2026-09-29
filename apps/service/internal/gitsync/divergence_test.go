// divergence_test.go — 分叉（unrelated histories）人工接管测试：
// status 分叉参考字段的出现/缺省、local 策略 force-push、remote 策略
// 备份+检出、来历不明远端与非分叉状态拒绝 resolve。
package gitsync

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

// seedDivergedVault 在 vault 里手工建一套与 remote 无共同祖先的本地历史
// （模拟 .git 撕裂重建/换机未克隆直接 init）：标准 .gitignore + 标记文件 +
// 现有内容全量提交，origin 指向 remote。返回本地 head 哈希。
func seedDivergedVault(t *testing.T, vault, remote string) plumbing.Hash {
	t.Helper()
	gr, err := git.PlainInitWithOptions(vault, &git.PlainInitOptions{
		InitOptions: git.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName("main")},
	})
	if err != nil {
		t.Fatal(err)
	}
	writeVaultFile(t, vault, ".gitignore", []byte(gitignoreContent))
	writeVaultFile(t, vault, markerFile, []byte(markerContent))
	w, err := gr.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Add("."); err != nil {
		t.Fatal(err)
	}
	hash, err := w.Commit("local rebuild", &git.CommitOptions{Author: commitAuthor})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gr.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{remote}}); err != nil {
		t.Fatal(err)
	}
	return hash
}

// seedForeignRemote 把 remote 推进成一个无 pinslip 标记的普通仓库（来历不明）。
func seedForeignRemote(t *testing.T, remote string) {
	t.Helper()
	seed := t.TempDir()
	gr, err := git.PlainInitWithOptions(seed, &git.PlainInitOptions{
		InitOptions: git.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName("main")},
	})
	if err != nil {
		t.Fatal(err)
	}
	writeVaultFile(t, seed, "README.md", []byte("hello\n"))
	w, _ := gr.Worktree()
	if _, err := w.Add("README.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Commit("init", &git.CommitOptions{Author: commitAuthor}); err != nil {
		t.Fatal(err)
	}
	if _, err := gr.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{remote}}); err != nil {
		t.Fatal(err)
	}
	if err := gr.Push(&git.PushOptions{}); err != nil {
		t.Fatal(err)
	}
}

// newDivergedEngine 起一个引擎并等待它进入分叉状态（lastErrorCode 命中）。
func newDivergedEngine(t *testing.T, vault, remote string) *Engine {
	t.Helper()
	eng, err := NewEngine(vault, tLogger{t})
	if err != nil {
		t.Fatal(err)
	}
	eng.pushInterval = time.Hour // 关掉定时 push，避免测试期定时循环干扰
	if err := eng.Reconfigure(testConfig(remote)); err != nil {
		t.Fatalf("Reconfigure 失败: %v", err)
	}
	waitForStatusCode(t, eng, CodeSyncUnrelatedHistories)
	return eng
}

// waitForStatusCode 轮询状态直到 lastErrorCode 命中（超时失败）。
func waitForStatusCode(t *testing.T, eng *Engine, code string) Status {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		st := eng.GetStatus()
		if st.LastErrorCode == code {
			return st
		}
		if time.Now().After(deadline) {
			t.Fatalf("等待 lastErrorCode=%s 超时: %+v", code, st)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// assertNoDivergenceFields 断言分叉参考字段全部缺省（非分叉状态零计算）。
func assertNoDivergenceFields(t *testing.T, st Status) {
	t.Helper()
	if st.RemoteIsPinslip != nil || st.RemoteLastCommitAt != nil || st.LocalNotes != 0 || st.RemoteNotes != 0 {
		t.Errorf("非分叉状态分叉字段应全部缺省: %+v", st)
	}
}

// 分叉时 status 带 4 个参考字段；正常同步时不带（零额外计算）。
func TestDivergenceStatusFields(t *testing.T) {
	remote := newBareRemote(t)
	vaultA := newVault(t)
	writeVaultFile(t, vaultA, "notes/a1.md", []byte("远端笔记 1\n"))
	writeVaultFile(t, vaultA, "notes/a2.md", []byte("远端笔记 2\n"))
	connectVault(t, vaultA, remote) // 远端：标记 + 2 条笔记

	vaultB := newVault(t)
	for _, name := range []string{"b1.md", "b2.md", "b3.md"} {
		writeVaultFile(t, vaultB, "notes/"+name, []byte("本地笔记\n"))
	}
	seedDivergedVault(t, vaultB, remote)

	eng := newDivergedEngine(t, vaultB, remote)
	defer eng.Stop()

	before := time.Now().Add(-time.Minute)
	st := waitForStatusCode(t, eng, CodeSyncUnrelatedHistories)
	if st.RemoteIsPinslip == nil || !*st.RemoteIsPinslip {
		t.Errorf("分叉时 remoteIsPinslip 应为 true: %+v", st)
	}
	if st.RemoteLastCommitAt == nil || st.RemoteLastCommitAt.Before(before) {
		t.Errorf("remoteLastCommitAt 应为远端最近提交时间: %+v", st.RemoteLastCommitAt)
	}
	if st.LocalNotes != 3 {
		t.Errorf("localNotes 应为 3, got %d", st.LocalNotes)
	}
	if st.RemoteNotes != 2 {
		t.Errorf("remoteNotes 应为 2, got %d", st.RemoteNotes)
	}

	// 正常同步的引擎（vaultA 侧）：分叉字段全部缺省
	engA, err := NewEngine(vaultA, tLogger{t})
	if err != nil {
		t.Fatal(err)
	}
	if err := engA.Reconfigure(testConfig(remote)); err != nil {
		t.Fatal(err)
	}
	defer engA.Stop()
	deadline := time.Now().Add(5 * time.Second)
	for {
		stA := engA.GetStatus()
		if stA.LastErrorCode == "" && stA.Configured {
			assertNoDivergenceFields(t, stA)
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("vaultA 应正常同步: %+v", stA)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// local 策略：force-push 后远端 head == 本地 head，分叉解除，状态回无错态。
func TestResolveDivergenceLocal(t *testing.T) {
	remote := newBareRemote(t)
	vaultA := newVault(t)
	writeVaultFile(t, vaultA, "notes/a-remote.md", []byte("远端旧快照\n"))
	connectVault(t, vaultA, remote)

	vaultB := newVault(t)
	writeVaultFile(t, vaultB, "notes/b-local.md", []byte("本地新内容\n"))
	localHead := seedDivergedVault(t, vaultB, remote)

	eng := newDivergedEngine(t, vaultB, remote)
	defer eng.Stop()

	if err := eng.ResolveDivergence("local"); err != nil {
		t.Fatalf("ResolveDivergence(local) 失败: %v", err)
	}

	// 远端 head 被覆盖为本地 head，远端可见本地内容
	bare, err := git.PlainOpen(remote)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := bare.Reference(plumbing.NewBranchReferenceName("main"), true)
	if err != nil {
		t.Fatal(err)
	}
	if ref.Hash() != localHead {
		t.Errorf("force-push 后远端 head 应等于本地 head: remote=%s local=%s", ref.Hash(), localHead)
	}
	if content, ok := remoteFileContent(t, remote, "notes/b-local.md"); !ok || content != "本地新内容\n" {
		t.Errorf("远端应有本地笔记, ok=%v content=%q", ok, content)
	}

	// 接回正常循环：无错态 + 分叉字段缺省
	st := eng.GetStatus()
	if st.LastErrorCode != "" || st.LastError != "" {
		t.Errorf("接管后应回无错态: code=%q err=%q", st.LastErrorCode, st.LastError)
	}
	assertNoDivergenceFields(t, st)

	// 分叉已解除：再次 resolve 应被前置校验拒绝
	if err := eng.ResolveDivergence("local"); !errors.Is(err, errResolvePrecondition) {
		t.Errorf("非分叉状态 resolve 应拒绝, got %v", err)
	}
}

// remote 策略：先备份 notes//inbox//attachments/ 再检出远端历史，
// 本地独占文件从工作区移除（备份里留存），分叉解除。
func TestResolveDivergenceRemote(t *testing.T) {
	remote := newBareRemote(t)
	vaultA := newVault(t)
	writeVaultFile(t, vaultA, "notes/a-remote.md", []byte("远端版本\n"))
	connectVault(t, vaultA, remote)

	vaultB := newVault(t)
	writeVaultFile(t, vaultB, "notes/b-local.md", []byte("本地独占\n"))
	seedDivergedVault(t, vaultB, remote)

	eng := newDivergedEngine(t, vaultB, remote)
	defer eng.Stop()

	if err := eng.ResolveDivergence("remote"); err != nil {
		t.Fatalf("ResolveDivergence(remote) 失败: %v", err)
	}

	// 备份目录存在且含本地独占文件
	backups, err := filepath.Glob(filepath.Join(vaultB, ".pinslip", "backups", "divergence-*"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("应有且仅有一个 divergence 备份目录: %v (err=%v)", backups, err)
	}
	if got := readVaultFile(t, backups[0], "notes/b-local.md"); got != "本地独占\n" {
		t.Errorf("备份应保留本地独占文件, got %q", got)
	}

	// 工作区已检出远端版本：远端文件在、本地独占文件移除
	if got := readVaultFile(t, vaultB, "notes/a-remote.md"); got != "远端版本\n" {
		t.Errorf("工作区应检出远端文件, got %q", got)
	}
	if _, err := os.Stat(filepath.Join(vaultB, "notes", "b-local.md")); !os.IsNotExist(err) {
		t.Error("本地独占文件应从工作区移除（已备份）")
	}

	// 接回正常循环：无错态 + 分叉字段缺省
	st := eng.GetStatus()
	if st.LastErrorCode != "" || st.LastError != "" {
		t.Errorf("接管后应回无错态: code=%q err=%q", st.LastErrorCode, st.LastError)
	}
	assertNoDivergenceFields(t, st)

	// 运行时文件不被检出触碰（applyTreeDiff 纪律：树外文件不动）
	writeVaultFile(t, vaultB, ".pinslip/pinslip.db", []byte("SQLITE-RUNTIME"))
	if _, err := eng.repo.Pull(); err != nil {
		t.Fatalf("接管后 Pull 应正常: %v", err)
	}
	if got := readVaultFile(t, vaultB, ".pinslip/pinslip.db"); got != "SQLITE-RUNTIME" {
		t.Errorf("运行时文件不应被检出触碰, got %q", got)
	}
}

// 来历不明远端（无 .pinslip-repo 标记）：status 报 remoteIsPinslip=false，
// 两种策略与非法 strategy 一律拒绝（前置校验 400 语义）。
func TestResolveDivergenceForeignRemoteRejected(t *testing.T) {
	remote := newBareRemote(t)
	seedForeignRemote(t, remote)

	vaultB := newVault(t)
	writeVaultFile(t, vaultB, "notes/b-local.md", []byte("本地内容\n"))
	seedDivergedVault(t, vaultB, remote)

	eng := newDivergedEngine(t, vaultB, remote)
	defer eng.Stop()

	st := waitForStatusCode(t, eng, CodeSyncUnrelatedHistories)
	if st.RemoteIsPinslip == nil || *st.RemoteIsPinslip {
		t.Errorf("来历不明远端 remoteIsPinslip 应为 false: %+v", st)
	}

	for _, strategy := range []string{"local", "remote", "bogus"} {
		if err := eng.ResolveDivergence(strategy); !errors.Is(err, errResolvePrecondition) {
			t.Errorf("strategy=%q 应被前置校验拒绝, got %v", strategy, err)
		}
	}
	// 拒绝后状态仍是分叉（未被接管动作扰动）
	if got := eng.GetStatus().LastErrorCode; got != CodeSyncUnrelatedHistories {
		t.Errorf("拒绝后应保持分叉状态, got %q", got)
	}
}
