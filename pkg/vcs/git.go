package vcs

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// VCS 版本控制系统类型
type VCS string

const (
	VCSGit VCS = "git"
)

// ChangeType 变更类型
type ChangeType string

const (
	ChangeAdded    ChangeType = "added"
	ChangeModified ChangeType = "modified"
	ChangeDeleted  ChangeType = "deleted"
	ChangeRenamed  ChangeType = "renamed"
)

// Commit 提交信息
type Commit struct {
	Hash      string    `json:"hash"`
	ShortHash string    `json:"short_hash"`
	Author    string    `json:"author"`
	Email     string    `json:"email"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Files     []FileChange `json:"files,omitempty"`
}

// FileChange 文件变更
type FileChange struct {
	Path string     `json:"path"`
	Type ChangeType `json:"type"`
}

// Branch 分支信息
type Branch struct {
	Name      string `json:"name"`
	IsCurrent bool   `json:"is_current"`
	IsRemote  bool   `json:"is_remote"`
	Hash      string `json:"hash"`
}

// Status 工作区状态
type Status struct {
	Branch       string       `json:"branch"`
	Ahead        int          `json:"ahead"`
	Behind       int          `json:"behind"`
	StagedFiles  []FileChange `json:"staged_files"`
	ChangedFiles []FileChange `json:"changed_files"`
	Untracked    []string     `json:"untracked"`
	IsClean      bool         `json:"is_clean"`
}

// Diff 差异信息
type Diff struct {
	Files []DiffFile `json:"files"`
	Stats DiffStats  `json:"stats"`
	Raw   string     `json:"raw"`
}

// DiffFile 差异文件
type DiffFile struct {
	Path     string     `json:"path"`
	Type     ChangeType `json:"type"`
	Additions int       `json:"additions"`
	Deletions int       `json:"deletions"`
}

// DiffStats 差异统计
type DiffStats struct {
	FilesChanged int `json:"files_changed"`
	Additions    int `json:"additions"`
	Deletions    int `json:"deletions"`
}

// GitClient Git 客户端
type GitClient struct {
	workDir string
}

// NewGitClient 创建 Git 客户端
func NewGitClient(workDir string) *GitClient {
	return &GitClient{workDir: workDir}
}

// runGit 运行 git 命令
func (g *GitClient) runGit(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w, output: %s", args[0], err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

// Init 初始化仓库
func (g *GitClient) Init(ctx context.Context) error {
	_, err := g.runGit(ctx, "init")
	return err
}

// Clone 克隆仓库
func (g *GitClient) Clone(ctx context.Context, url, path string) error {
	cmd := exec.CommandContext(ctx, "git", "clone", url, path)
	cmd.Dir = g.workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %w, output: %s", err, string(output))
	}
	return nil
}

// Status 获取工作区状态
func (g *GitClient) Status(ctx context.Context) (*Status, error) {
	// 获取当前分支
	branch, err := g.runGit(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, err
	}

	// 获取 ahead/behind
	ahead, behind := 0, 0
	upstream, err := g.runGit(ctx, "rev-parse", "--abbrev-ref", "@{upstream}")
	if err == nil {
		counts, err := g.runGit(ctx, "rev-list", "--left-right", "--count", upstream+"...HEAD")
		if err == nil {
			fmt.Sscanf(counts, "%d\t%d", &behind, &ahead)
		}
	}

	// 获取文件状态
	statusOutput, err := g.runGit(ctx, "status", "--porcelain")
	if err != nil {
		return nil, err
	}

	status := &Status{
		Branch: branch,
		Ahead:  ahead,
		Behind: behind,
	}

	for _, line := range strings.Split(statusOutput, "\n") {
		if len(line) < 3 {
			continue
		}

		indexStatus := line[0]
		workTreeStatus := line[1]
		path := strings.TrimSpace(line[3:])

		if indexStatus != ' ' && indexStatus != '?' {
			status.StagedFiles = append(status.StagedFiles, FileChange{
				Path: path,
				Type: parseChangeType(indexStatus),
			})
		}

		if workTreeStatus != ' ' && workTreeStatus != '?' {
			status.ChangedFiles = append(status.ChangedFiles, FileChange{
				Path: path,
				Type: parseChangeType(workTreeStatus),
			})
		}

		if indexStatus == '?' && workTreeStatus == '?' {
			status.Untracked = append(status.Untracked, path)
		}
	}

	status.IsClean = len(status.StagedFiles) == 0 &&
		len(status.ChangedFiles) == 0 &&
		len(status.Untracked) == 0

	return status, nil
}

// Add 暂存文件
func (g *GitClient) Add(ctx context.Context, files ...string) error {
	args := append([]string{"add"}, files...)
	_, err := g.runGit(ctx, args...)
	return err
}

// AddAll 暂存所有文件
func (g *GitClient) AddAll(ctx context.Context) error {
	_, err := g.runGit(ctx, "add", "-A")
	return err
}

// Commit 提交
func (g *GitClient) Commit(ctx context.Context, message string) (string, error) {
	_, err := g.runGit(ctx, "commit", "-m", message)
	if err != nil {
		return "", err
	}

	// 提取 commit hash
	hash, err := g.runGit(ctx, "rev-parse", "HEAD")
	if err != nil {
		return "", nil
	}

	return hash, nil
}

// Push 推送
func (g *GitClient) Push(ctx context.Context, remote, branch string) error {
	if remote == "" {
		remote = "origin"
	}
	if branch == "" {
		branch = "HEAD"
	}
	_, err := g.runGit(ctx, "push", remote, branch)
	return err
}

// Pull 拉取
func (g *GitClient) Pull(ctx context.Context, remote, branch string) error {
	if remote == "" {
		remote = "origin"
	}
	_, err := g.runGit(ctx, "pull", remote, branch)
	return err
}

// Fetch 获取远程更新
func (g *GitClient) Fetch(ctx context.Context, remote string) error {
	if remote == "" {
		remote = "origin"
	}
	_, err := g.runGit(ctx, "fetch", remote)
	return err
}

// GetLog 获取提交历史
func (g *GitClient) GetLog(ctx context.Context, limit int) ([]*Commit, error) {
	if limit <= 0 {
		limit = 20
	}

	// 使用自定义格式
	format := "%H|%h|%an|%ae|%ai|%s"
	output, err := g.runGit(ctx, "log", fmt.Sprintf("-%d", limit), "--pretty=format:"+format)
	if err != nil {
		return nil, err
	}

	var commits []*Commit
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 6)
		if len(parts) < 6 {
			continue
		}

		commit := &Commit{
			Hash:      parts[0],
			ShortHash: parts[1],
			Author:    parts[2],
			Email:     parts[3],
			Message:   parts[5],
		}

		// 解析时间
		t, err := time.Parse("2006-01-02 15:04:05 -0700", parts[4])
		if err == nil {
			commit.Timestamp = t
		}

		commits = append(commits, commit)
	}

	return commits, nil
}

// GetDiff 获取差异
func (g *GitClient) GetDiff(ctx context.Context, staged bool) (*Diff, error) {
	args := []string{"diff"}
	if staged {
		args = append(args, "--staged")
	}
	args = append(args, "--stat")

	statOutput, err := g.runGit(ctx, args...)
	if err != nil {
		return nil, err
	}

	// 获取原始 diff
	args = []string{"diff"}
	if staged {
		args = append(args, "--staged")
	}
	rawOutput, _ := g.runGit(ctx, args...)

	diff := &Diff{
		Raw: rawOutput,
	}

	// 解析 stat
	lines := strings.Split(statOutput, "\n")
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, " ") {
			continue
		}

		parts := strings.SplitN(line, "|", 2)
		if len(parts) < 2 {
			continue
		}

		path := strings.TrimSpace(parts[0])
		changeInfo := strings.TrimSpace(parts[1])

		df := DiffFile{
			Path: path,
		}

		// 解析增删行数
		if strings.Contains(changeInfo, "+") && strings.Contains(changeInfo, "-") {
			fmt.Sscanf(changeInfo, "%d insertions(+), %d deletions(-)", &df.Additions, &df.Deletions)
		} else if strings.Contains(changeInfo, "+") {
			fmt.Sscanf(changeInfo, "%d insertions(+)", &df.Additions)
		} else if strings.Contains(changeInfo, "-") {
			fmt.Sscanf(changeInfo, "%d deletions(-)", &df.Deletions)
		}

		diff.Files = append(diff.Files, df)
		diff.Stats.Additions += df.Additions
		diff.Stats.Deletions += df.Deletions
	}
	diff.Stats.FilesChanged = len(diff.Files)

	return diff, nil
}

// GetBranches 获取分支列表
func (g *GitClient) GetBranches(ctx context.Context) ([]*Branch, error) {
	output, err := g.runGit(ctx, "branch", "-a", "--list")
	if err != nil {
		return nil, err
	}

	// 获取当前分支
	currentBranch, _ := g.runGit(ctx, "rev-parse", "--abbrev-ref", "HEAD")

	var branches []*Branch
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}

		isCurrent := strings.HasPrefix(line, "*")
		line = strings.TrimLeft(line, "* ")
		line = strings.TrimSpace(line)

		isRemote := strings.HasPrefix(line, "remotes/")
		name := strings.TrimPrefix(line, "remotes/")
		name = strings.TrimPrefix(name, "origin/")

		// 获取分支 hash
		hash, _ := g.runGit(ctx, "rev-parse", "--short", line)

		branch := &Branch{
			Name:      name,
			IsCurrent: isCurrent || (name == currentBranch),
			IsRemote:  isRemote,
			Hash:      hash,
		}

		branches = append(branches, branch)
	}

	return branches, nil
}

// CreateBranch 创建分支
func (g *GitClient) CreateBranch(ctx context.Context, name string) error {
	_, err := g.runGit(ctx, "branch", name)
	return err
}

// SwitchBranch 切换分支
func (g *GitClient) SwitchBranch(ctx context.Context, name string) error {
	_, err := g.runGit(ctx, "checkout", name)
	return err
}

// CreateAndSwitchBranch 创建并切换分支
func (g *GitClient) CreateAndSwitchBranch(ctx context.Context, name string) error {
	_, err := g.runGit(ctx, "checkout", "-b", name)
	return err
}

// Merge 合并分支
func (g *GitClient) Merge(ctx context.Context, branch string) error {
	_, err := g.runGit(ctx, "merge", branch)
	return err
}

// Stash 暂存工作区
func (g *GitClient) Stash(ctx context.Context, message string) error {
	if message != "" {
		_, err := g.runGit(ctx, "stash", "push", "-m", message)
		return err
	}
	_, err := g.runGit(ctx, "stash")
	return err
}

// StashPop 恢复暂存
func (g *GitClient) StashPop(ctx context.Context) error {
	_, err := g.runGit(ctx, "stash", "pop")
	return err
}

// Tag 创建标签
func (g *GitClient) Tag(ctx context.Context, name, message string) error {
	if message != "" {
		_, err := g.runGit(ctx, "tag", "-a", name, "-m", message)
		return err
	}
	_, err := g.runGit(ctx, "tag", name)
	return err
}

// GetTags 获取标签列表
func (g *GitClient) GetTags(ctx context.Context) ([]string, error) {
	output, err := g.runGit(ctx, "tag", "-l")
	if err != nil {
		return nil, err
	}

	var tags []string
	for _, line := range strings.Split(output, "\n") {
		if line != "" {
			tags = append(tags, line)
		}
	}

	return tags, nil
}

// GetRemoteURL 获取远程 URL
func (g *GitClient) GetRemoteURL(ctx context.Context, remote string) (string, error) {
	if remote == "" {
		remote = "origin"
	}
	return g.runGit(ctx, "remote", "get-url", remote)
}

// SetRemoteURL 设置远程 URL
func (g *GitClient) SetRemoteURL(ctx context.Context, remote, url string) error {
	if remote == "" {
		remote = "origin"
	}
	_, err := g.runGit(ctx, "remote", "set-url", remote, url)
	return err
}

// AddRemote 添加远程
func (g *GitClient) AddRemote(ctx context.Context, name, url string) error {
	_, err := g.runGit(ctx, "remote", "add", name, url)
	return err
}

// IsRepo 检查是否为 Git 仓库
func (g *GitClient) IsRepo(ctx context.Context) bool {
	_, err := g.runGit(ctx, "rev-parse", "--git-dir")
	return err == nil
}

// GetRoot 获取仓库根目录
func (g *GitClient) GetRoot(ctx context.Context) (string, error) {
	return g.runGit(ctx, "rev-parse", "--show-toplevel")
}

// GetConfig 获取 Git 配置
func (g *GitClient) GetConfig(ctx context.Context, key string) (string, error) {
	return g.runGit(ctx, "config", "--get", key)
}

// SetConfig 设置 Git 配置
func (g *GitClient) SetConfig(ctx context.Context, key, value string) error {
	_, err := g.runGit(ctx, "config", key, value)
	return err
}

// 辅助函数

// parseChangeType 解析变更类型
func parseChangeType(status byte) ChangeType {
	switch status {
	case 'A':
		return ChangeAdded
	case 'M':
		return ChangeModified
	case 'D':
		return ChangeDeleted
	case 'R':
		return ChangeRenamed
	default:
		return ChangeModified
	}
}
