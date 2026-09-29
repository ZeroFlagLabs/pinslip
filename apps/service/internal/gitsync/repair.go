// repair.go — .git 撕裂自愈（诊断驱动）。
//
// 背景：并发写/断电可把 .git/index 或 refs/heads/<branch> 撕成全零/截断文件，
// go-git 报 malformed index signature / object not found，同步永久卡死。
// 「磁盘即真相」架构下这类损伤可无损修复：索引只是缓存（下次 Add 自动重建），
// 非法分支引用移除后 HEAD 回 unborn、首轮提交从工作区重建——全程不碰工作区。
//
// 纪律（changes/add-git-repo-self-heal/design.md）：
//   - 诊断只读，判定标准只依赖文件本身的可验证状态，绝不匹配错误文本；
//   - 判不准的不碰：stat/读取的非 IsNotExist 错误（典型：权限拒绝）一律跳过，
//     不当作损坏；对象库级损坏（fsck 才能发现）不在判定范围；
//   - 修复 = 隔离（移到 .git/corrupt-<yyyymmdd>-<hhmmss>-<原名>）+ 最小重置，
//     ref 必须移出 refs/ 目录（go-git 遍历 refs 时会读到隔离文件）；
//   - 修复动作写英文诊断日志（[gitsync] 前缀 + 全量路径，便于整段复制排查）。
package gitsync

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// hash40Re 合法提交哈希：40 位小写 hex（trim 后整体匹配）。
var hash40Re = regexp.MustCompile(`^[0-9a-f]{40}$`)

// repairIfBroken 对 vault 的 .git 做只读诊断，仅修复可直接验证的损坏，
// 返回是否有修复动作（调用方据此决定是否重试原操作一次）。
// branch 是 HEAD 撕裂时的重置目标；为空则跳过 HEAD 重置（目标不明不碰）。
func repairIfBroken(dir, branch string) (repaired bool) {
	gitDir := filepath.Join(dir, ".git")
	if repairIndex(gitDir) {
		repaired = true
	}
	if repairHEAD(gitDir, branch) {
		repaired = true
	}
	return repaired
}

// repairIndex 判定并修复撕裂的索引：文件存在且（size==0 或头 4 字节非 DIRC）。
// 索引是缓存，隔离移除后 go-git 下次 Add 自动重建。
func repairIndex(gitDir string) bool {
	p := filepath.Join(gitDir, "index")
	fi, err := os.Stat(p)
	if err != nil {
		return false // 不存在（未建过索引）或读不了（权限）都不是可验证损坏
	}
	corrupt := fi.Size() == 0
	if !corrupt {
		f, err := os.Open(p)
		if err != nil {
			return false
		}
		var sig [4]byte
		n, err := io.ReadFull(f, sig[:])
		f.Close() // 立即关闭：Windows 上句柄未关会锁住后续隔离 rename
		if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
			return false // 读取失败（权限等）不判损坏
		}
		corrupt = n < 4 || string(sig[:]) != "DIRC"
	}
	if !corrupt {
		return false
	}
	dst, err := quarantine(p, gitDir, "index")
	if err != nil {
		log.Printf("[gitsync] self-heal: failed to quarantine torn index %s: %v", p, err)
		return false
	}
	log.Printf("[gitsync] self-heal: quarantined torn git index %s -> %s (index rebuilds on next Add)", p, dst)
	return true
}

// repairHEAD 判定并修复 HEAD 链路的撕裂：
//   - HEAD 为 `ref: refs/heads/X` 且 X 文件存在但内容非 40 hex → 隔离移除该 ref
//     （HEAD 回 unborn，首轮提交从工作区重建）；X 不存在 = unborn，合法不动；
//   - HEAD 内容既非 `ref: ` 前缀也非 40 hex → 重置为 ref: refs/heads/<branch>。
func repairHEAD(gitDir, branch string) bool {
	headPath := filepath.Join(gitDir, "HEAD")
	data, err := os.ReadFile(headPath)
	if err != nil {
		return false // 缺失或不可读（权限）都不判损坏
	}
	content := strings.TrimSpace(string(data))
	if ref, ok := strings.CutPrefix(content, "ref: "); ok {
		ref = strings.TrimSpace(ref)
		if !strings.HasPrefix(ref, "refs/heads/") {
			return false // 非常规符号引用：判不准不碰
		}
		return repairBranchRef(gitDir, ref)
	}
	if hash40Re.MatchString(content) {
		return false // detached HEAD，合法
	}
	if branch == "" {
		return false // 重置目标不明，不碰
	}
	target := "ref: refs/heads/" + branch + "\n"
	if err := os.WriteFile(headPath, []byte(target), 0o644); err != nil {
		log.Printf("[gitsync] self-heal: failed to reset torn HEAD %s: %v", headPath, err)
		return false
	}
	log.Printf("[gitsync] self-heal: reset torn HEAD %s -> %q", headPath, target)
	return true
}

// repairBranchRef refs/heads/X 文件存在但内容非 40 hex 时隔离移除。
// 隔离目标在 .git/ 根——留在 refs/heads/ 下会被 go-git 遍历 refs 时读到。
func repairBranchRef(gitDir, ref string) bool {
	p := filepath.Join(gitDir, filepath.FromSlash(ref))
	data, err := os.ReadFile(p)
	if err != nil {
		return false // 不存在 = unborn（合法新仓库）；读不了（权限）不判损坏
	}
	if hash40Re.MatchString(strings.TrimSpace(string(data))) {
		return false
	}
	name := strings.ReplaceAll(strings.TrimPrefix(ref, "refs/heads/"), "/", "_")
	dst, err := quarantine(p, gitDir, name)
	if err != nil {
		log.Printf("[gitsync] self-heal: failed to quarantine torn ref %s: %v", p, err)
		return false
	}
	log.Printf("[gitsync] self-heal: quarantined torn branch ref %s -> %s (HEAD back to unborn)", p, dst)
	return true
}

// quarantine 把损坏文件移动到 .git/ 根的 corrupt-<yyyymmdd>-<hhmmss>-<原名>
// （重名追加序号），原位置空缺即完成「删除」语义。返回隔离目标路径。
func quarantine(path, gitDir, name string) (string, error) {
	now := time.Now().Format("20060102-150405")
	cand := filepath.Join(gitDir, "corrupt-"+now+"-"+name)
	for i := 2; ; i++ {
		if _, err := os.Stat(cand); os.IsNotExist(err) {
			break
		}
		cand = filepath.Join(gitDir, fmt.Sprintf("corrupt-%s-%s-%d", now, name, i))
	}
	if err := os.Rename(path, cand); err != nil {
		return "", err
	}
	return cand, nil
}
