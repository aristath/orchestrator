package worktree

import (
	"bufio"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// WorktreeManager manages git worktrees for parallel task execution
type WorktreeManager struct {
	config   WorktreeManagerConfig
	mergeMu  sync.Mutex // Serializes merge operations to prevent git lock conflicts
}

// NewWorktreeManager creates a new worktree manager
func NewWorktreeManager(cfg WorktreeManagerConfig) *WorktreeManager {
	if cfg.WorktreeDir == "" {
		cfg.WorktreeDir = ".worktrees"
	}
	return &WorktreeManager{config: cfg}
}

// Create creates a new worktree for the given task ID
func (m *WorktreeManager) Create(taskID string) (*WorktreeInfo, error) {
	branch := fmt.Sprintf("task/%s", taskID)
	wtPath := filepath.Join(m.config.RepoPath, m.config.WorktreeDir, taskID)

	// Create worktree with new branch based on baseBranch
	cmd := exec.Command("git", "worktree", "add", "-b", branch, wtPath, m.config.BaseBranch)
	cmd.Dir = m.config.RepoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to create worktree: %w (output: %s)", err, string(output))
	}

	// Get HEAD commit
	headCmd := exec.Command("git", "rev-parse", "HEAD")
	headCmd.Dir = wtPath
	headOutput, err := headCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD commit: %w (output: %s)", err, string(headOutput))
	}

	info := &WorktreeInfo{
		Path:   wtPath,
		Branch: branch,
		TaskID: taskID,
		Head:   strings.TrimSpace(string(headOutput)),
	}

	return info, nil
}

// Merge merges the worktree branch back to the base branch
func (m *WorktreeManager) Merge(info *WorktreeInfo, strategy MergeStrategy) (*MergeResult, error) {
	// Serialize merge operations to prevent concurrent git operations on the main repo
	m.mergeMu.Lock()
	defer m.mergeMu.Unlock()

	// First, checkout base branch to ensure we're merging into the right place
	checkoutCmd := exec.Command("git", "checkout", m.config.BaseBranch)
	checkoutCmd.Dir = m.config.RepoPath
	if checkoutOutput, err := checkoutCmd.CombinedOutput(); err != nil {
		return &MergeResult{
			Merged: false,
			Error:  fmt.Errorf("failed to checkout base branch: %w (output: %s)", err, string(checkoutOutput)),
		}, nil
	}

	// Detect conflicts using merge-tree (dry-run merge)
	detectCmd := exec.Command("git", "merge-tree", "--write-tree", m.config.BaseBranch, info.Branch)
	detectCmd.Dir = m.config.RepoPath
	detectOutput, err := detectCmd.CombinedOutput()
	if err != nil {
		// Non-zero exit indicates conflicts
		result := &MergeResult{
			Merged: false,
			Error:  fmt.Errorf("merge conflict detected: %s", string(detectOutput)),
		}
		// Try to parse conflict files from output
		result.ConflictFiles = parseConflictFiles(string(detectOutput))
		return result, nil
	}

	// Check if output contains conflict markers (git merge-tree may exit 0 but still have conflicts)
	outputStr := string(detectOutput)
	if strings.Contains(outputStr, "CONFLICT") {
		result := &MergeResult{
			Merged: false,
			Error:  fmt.Errorf("merge conflict detected: %s", outputStr),
		}
		result.ConflictFiles = parseConflictFiles(outputStr)
		return result, nil
	}

	// No conflicts, perform actual merge
	// Map strategy to git merge strategy names
	strategyArg := "recursive" // default
	if strategy == MergeOurs {
		strategyArg = "ours"
	} else if strategy == MergeTheirs {
		strategyArg = "theirs"
	}

	mergeCmd := exec.Command("git", "merge", "--no-ff", "-s", strategyArg, info.Branch)
	mergeCmd.Dir = m.config.RepoPath
	mergeOutput, err := mergeCmd.CombinedOutput()
	if err != nil {
		return &MergeResult{
			Merged: false,
			Error:  fmt.Errorf("merge failed: %w (output: %s)", err, string(mergeOutput)),
		}, nil
	}

	return &MergeResult{Merged: true}, nil
}

// parseConflictFiles attempts to extract conflicting file paths from merge-tree output
func parseConflictFiles(output string) []string {
	var conflicts []string
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		// merge-tree output includes lines like "CONFLICT (content): Merge conflict in <file>"
		if strings.Contains(line, "CONFLICT") && strings.Contains(line, "in ") {
			parts := strings.Split(line, "in ")
			if len(parts) > 1 {
				conflicts = append(conflicts, strings.TrimSpace(parts[len(parts)-1]))
			}
		}
	}
	return conflicts
}

// Cleanup removes the worktree and deletes the branch
func (m *WorktreeManager) Cleanup(info *WorktreeInfo) error {
	var errors []string

	// Remove worktree
	removeCmd := exec.Command("git", "worktree", "remove", info.Path)
	removeCmd.Dir = m.config.RepoPath
	if output, err := removeCmd.CombinedOutput(); err != nil {
		// Retry with --force
		forceCmd := exec.Command("git", "worktree", "remove", "--force", info.Path)
		forceCmd.Dir = m.config.RepoPath
		if forceOutput, forceErr := forceCmd.CombinedOutput(); forceErr != nil {
			errors = append(errors, fmt.Sprintf("worktree remove failed: %v (output: %s, force output: %s)", err, string(output), string(forceOutput)))
		}
	}

	// Delete branch
	branchCmd := exec.Command("git", "branch", "-d", info.Branch)
	branchCmd.Dir = m.config.RepoPath
	if output, err := branchCmd.CombinedOutput(); err != nil {
		// Retry with -D (force delete)
		forceCmd := exec.Command("git", "branch", "-D", info.Branch)
		forceCmd.Dir = m.config.RepoPath
		if forceOutput, forceErr := forceCmd.CombinedOutput(); forceErr != nil {
			errors = append(errors, fmt.Sprintf("branch delete failed: %v (output: %s, force output: %s)", err, string(output), string(forceOutput)))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %s", strings.Join(errors, "; "))
	}
	return nil
}

// ForceCleanup removes the worktree and branch using force flags
func (m *WorktreeManager) ForceCleanup(info *WorktreeInfo) error {
	var errors []string

	// Force remove worktree
	removeCmd := exec.Command("git", "worktree", "remove", "--force", info.Path)
	removeCmd.Dir = m.config.RepoPath
	if output, err := removeCmd.CombinedOutput(); err != nil {
		errors = append(errors, fmt.Sprintf("force worktree remove failed: %v (output: %s)", err, string(output)))
	}

	// Force delete branch
	branchCmd := exec.Command("git", "branch", "-D", info.Branch)
	branchCmd.Dir = m.config.RepoPath
	if output, err := branchCmd.CombinedOutput(); err != nil {
		errors = append(errors, fmt.Sprintf("force branch delete failed: %v (output: %s)", err, string(output)))
	}

	if len(errors) > 0 {
		return fmt.Errorf("force cleanup errors: %s", strings.Join(errors, "; "))
	}
	return nil
}

// List returns all worktrees in the repository
func (m *WorktreeManager) List() ([]WorktreeInfo, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = m.config.RepoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w (output: %s)", err, string(output))
	}

	var worktrees []WorktreeInfo
	var current WorktreeInfo

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			// Empty line signals end of a worktree entry
			if current.Path != "" {
				worktrees = append(worktrees, current)
				current = WorktreeInfo{}
			}
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			current.Path = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "HEAD ") {
			current.Head = strings.TrimPrefix(line, "HEAD ")
		} else if strings.HasPrefix(line, "branch ") {
			branch := strings.TrimPrefix(line, "branch ")
			current.Branch = strings.TrimPrefix(branch, "refs/heads/")
			// Extract task ID from branch name (format: task/{taskID})
			if strings.HasPrefix(current.Branch, "task/") {
				current.TaskID = strings.TrimPrefix(current.Branch, "task/")
			}
		}
	}

	// Add last entry if not followed by empty line
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees, nil
}

// Prune cleans up stale worktree metadata
func (m *WorktreeManager) Prune() error {
	cmd := exec.Command("git", "worktree", "prune")
	cmd.Dir = m.config.RepoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to prune worktrees: %w (output: %s)", err, string(output))
	}
	return nil
}
