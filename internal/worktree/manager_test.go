package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestRepo creates a temporary git repository for testing
func setupTestRepo(t *testing.T) string {
	t.Helper()

	repoPath := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v (output: %s)", err, string(output))
	}

	// Configure git user for commits
	configName := exec.Command("git", "config", "user.name", "Test User")
	configName.Dir = repoPath
	if output, err := configName.CombinedOutput(); err != nil {
		t.Fatalf("git config user.name failed: %v (output: %s)", err, string(output))
	}

	configEmail := exec.Command("git", "config", "user.email", "test@example.com")
	configEmail.Dir = repoPath
	if output, err := configEmail.CombinedOutput(); err != nil {
		t.Fatalf("git config user.email failed: %v (output: %s)", err, string(output))
	}

	// Create initial branch (main)
	checkout := exec.Command("git", "checkout", "-b", "main")
	checkout.Dir = repoPath
	if output, err := checkout.CombinedOutput(); err != nil {
		t.Fatalf("git checkout -b main failed: %v (output: %s)", err, string(output))
	}

	// Create initial file
	initialFile := filepath.Join(repoPath, "README.md")
	if err := os.WriteFile(initialFile, []byte("# Test Repo\n"), 0644); err != nil {
		t.Fatalf("failed to write initial file: %v", err)
	}

	// Add and commit
	add := exec.Command("git", "add", ".")
	add.Dir = repoPath
	if output, err := add.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v (output: %s)", err, string(output))
	}

	commit := exec.Command("git", "commit", "-m", "initial commit")
	commit.Dir = repoPath
	if output, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v (output: %s)", err, string(output))
	}

	return repoPath
}

func TestCreate(t *testing.T) {
	repoPath := setupTestRepo(t)

	manager := NewWorktreeManager(WorktreeManagerConfig{
		RepoPath:   repoPath,
		BaseBranch: "main",
	})

	info, err := manager.Create("test-task-1")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify worktree directory exists
	if _, err := os.Stat(info.Path); os.IsNotExist(err) {
		t.Errorf("worktree directory does not exist: %s", info.Path)
	}

	// Verify .git file exists (worktrees use gitfile, not directory)
	gitFile := filepath.Join(info.Path, ".git")
	if stat, err := os.Stat(gitFile); err != nil {
		t.Errorf(".git file does not exist: %v", err)
	} else if stat.IsDir() {
		t.Errorf(".git is a directory, expected file (gitfile)")
	}

	// Verify branch exists
	branchCmd := exec.Command("git", "branch", "--list", info.Branch)
	branchCmd.Dir = repoPath
	output, err := branchCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git branch --list failed: %v", err)
	}
	if !strings.Contains(string(output), info.Branch) {
		t.Errorf("branch %s not found in git branch output", info.Branch)
	}

	// Verify WorktreeInfo fields
	if info.TaskID != "test-task-1" {
		t.Errorf("expected TaskID 'test-task-1', got '%s'", info.TaskID)
	}
	if info.Branch != "task/test-task-1" {
		t.Errorf("expected Branch 'task/test-task-1', got '%s'", info.Branch)
	}
	if info.Head == "" {
		t.Errorf("Head commit should not be empty")
	}
}

func TestCreateDuplicateID(t *testing.T) {
	repoPath := setupTestRepo(t)

	manager := NewWorktreeManager(WorktreeManagerConfig{
		RepoPath:   repoPath,
		BaseBranch: "main",
	})

	// Create first worktree
	_, err := manager.Create("duplicate-task")
	if err != nil {
		t.Fatalf("First Create failed: %v", err)
	}

	// Attempt to create second worktree with same ID
	_, err = manager.Create("duplicate-task")
	if err == nil {
		t.Errorf("expected error when creating duplicate worktree, got nil")
	}
}

func TestMergeClean(t *testing.T) {
	repoPath := setupTestRepo(t)

	manager := NewWorktreeManager(WorktreeManagerConfig{
		RepoPath:        repoPath,
		BaseBranch:      "main",
		DefaultStrategy: MergeOrt,
	})

	// Create worktree
	info, err := manager.Create("merge-clean-task")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Add a new file in the worktree
	newFile := filepath.Join(info.Path, "feature.txt")
	if err := os.WriteFile(newFile, []byte("new feature\n"), 0644); err != nil {
		t.Fatalf("failed to write new file: %v", err)
	}

	// Commit in worktree
	addCmd := exec.Command("git", "add", "feature.txt")
	addCmd.Dir = info.Path
	if output, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git add in worktree failed: %v (output: %s)", err, string(output))
	}

	commitCmd := exec.Command("git", "commit", "-m", "add feature")
	commitCmd.Dir = info.Path
	if output, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit in worktree failed: %v (output: %s)", err, string(output))
	}

	// Merge back to main
	result, err := manager.Merge(info, MergeOrt)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	if !result.Merged {
		t.Errorf("expected clean merge, got Merged=false with error: %v", result.Error)
	}

	// Verify file exists in main worktree
	mainFeatureFile := filepath.Join(repoPath, "feature.txt")
	if _, err := os.Stat(mainFeatureFile); os.IsNotExist(err) {
		t.Errorf("feature.txt not found in main worktree after merge")
	}
}

func TestMergeConflict(t *testing.T) {
	repoPath := setupTestRepo(t)

	manager := NewWorktreeManager(WorktreeManagerConfig{
		RepoPath:   repoPath,
		BaseBranch: "main",
	})

	// Create worktree first (before modifying main)
	info, err := manager.Create("conflict-task")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Modify README.md in main worktree
	mainReadme := filepath.Join(repoPath, "README.md")
	if err := os.WriteFile(mainReadme, []byte("# Test Repo\nMain branch content\n"), 0644); err != nil {
		t.Fatalf("failed to modify README in main: %v", err)
	}

	addMain := exec.Command("git", "add", "README.md")
	addMain.Dir = repoPath
	if output, err := addMain.CombinedOutput(); err != nil {
		t.Fatalf("git add in main failed: %v (output: %s)", err, string(output))
	}

	commitMain := exec.Command("git", "commit", "-m", "update README in main")
	commitMain.Dir = repoPath
	if output, err := commitMain.CombinedOutput(); err != nil {
		t.Fatalf("git commit in main failed: %v (output: %s)", err, string(output))
	}

	// Modify same file differently in worktree
	wtReadme := filepath.Join(info.Path, "README.md")
	if err := os.WriteFile(wtReadme, []byte("# Test Repo\nWorktree branch content\n"), 0644); err != nil {
		t.Fatalf("failed to modify README in worktree: %v", err)
	}

	addWT := exec.Command("git", "add", "README.md")
	addWT.Dir = info.Path
	if output, err := addWT.CombinedOutput(); err != nil {
		t.Fatalf("git add in worktree failed: %v (output: %s)", err, string(output))
	}

	commitWT := exec.Command("git", "commit", "-m", "update README in worktree")
	commitWT.Dir = info.Path
	if output, err := commitWT.CombinedOutput(); err != nil {
		t.Fatalf("git commit in worktree failed: %v (output: %s)", err, string(output))
	}

	// Attempt merge - should detect conflict
	result, err := manager.Merge(info, MergeOrt)
	if err != nil {
		t.Fatalf("Merge returned error: %v", err)
	}

	if result.Merged {
		t.Errorf("expected conflict detection, got Merged=true")
	}

	if result.Error == nil {
		t.Errorf("expected conflict error, got nil")
	}

	// Verify git state is clean (no merge in progress)
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = repoPath
	statusOutput, err := statusCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git status failed: %v", err)
	}
	if strings.Contains(string(statusOutput), "UU") || strings.Contains(string(statusOutput), "AA") {
		t.Errorf("git state is not clean after conflict detection: %s", string(statusOutput))
	}
}

func TestCleanup(t *testing.T) {
	repoPath := setupTestRepo(t)

	manager := NewWorktreeManager(WorktreeManagerConfig{
		RepoPath:   repoPath,
		BaseBranch: "main",
	})

	info, err := manager.Create("cleanup-task")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify worktree exists
	if _, err := os.Stat(info.Path); os.IsNotExist(err) {
		t.Fatalf("worktree should exist before cleanup")
	}

	// Cleanup
	if err := manager.Cleanup(info); err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Verify worktree directory removed
	if _, err := os.Stat(info.Path); !os.IsNotExist(err) {
		t.Errorf("worktree directory still exists after cleanup")
	}

	// Verify branch deleted
	branchCmd := exec.Command("git", "branch", "--list", info.Branch)
	branchCmd.Dir = repoPath
	output, err := branchCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git branch --list failed: %v", err)
	}
	if strings.Contains(string(output), info.Branch) {
		t.Errorf("branch %s still exists after cleanup", info.Branch)
	}

	// Verify not in worktree list
	worktrees, err := manager.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	for _, wt := range worktrees {
		if wt.Branch == info.Branch {
			t.Errorf("worktree %s still in list after cleanup", info.Branch)
		}
	}
}

func TestForceCleanup(t *testing.T) {
	repoPath := setupTestRepo(t)

	manager := NewWorktreeManager(WorktreeManagerConfig{
		RepoPath:   repoPath,
		BaseBranch: "main",
	})

	info, err := manager.Create("force-cleanup-task")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Make uncommitted changes in worktree
	dirtyFile := filepath.Join(info.Path, "dirty.txt")
	if err := os.WriteFile(dirtyFile, []byte("uncommitted\n"), 0644); err != nil {
		t.Fatalf("failed to create dirty file: %v", err)
	}

	// Force cleanup should succeed despite dirty state
	if err := manager.ForceCleanup(info); err != nil {
		t.Fatalf("ForceCleanup failed: %v", err)
	}

	// Verify worktree directory removed
	if _, err := os.Stat(info.Path); !os.IsNotExist(err) {
		t.Errorf("worktree directory still exists after force cleanup")
	}

	// Verify branch deleted
	branchCmd := exec.Command("git", "branch", "--list", info.Branch)
	branchCmd.Dir = repoPath
	output, err := branchCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git branch --list failed: %v", err)
	}
	if strings.Contains(string(output), info.Branch) {
		t.Errorf("branch %s still exists after force cleanup", info.Branch)
	}
}

func TestPrune(t *testing.T) {
	repoPath := setupTestRepo(t)

	manager := NewWorktreeManager(WorktreeManagerConfig{
		RepoPath:   repoPath,
		BaseBranch: "main",
	})

	info, err := manager.Create("prune-task")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Manually remove worktree directory (simulating crash)
	if err := os.RemoveAll(info.Path); err != nil {
		t.Fatalf("failed to remove worktree directory: %v", err)
	}

	// Prune should clean up stale metadata
	if err := manager.Prune(); err != nil {
		t.Fatalf("Prune failed: %v", err)
	}

	// Verify worktree no longer in list
	worktrees, err := manager.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	for _, wt := range worktrees {
		if wt.Branch == info.Branch {
			t.Errorf("stale worktree %s still in list after prune", info.Branch)
		}
	}
}

func TestList(t *testing.T) {
	repoPath := setupTestRepo(t)

	manager := NewWorktreeManager(WorktreeManagerConfig{
		RepoPath:   repoPath,
		BaseBranch: "main",
	})

	// Create two worktrees
	info1, err := manager.Create("list-task-1")
	if err != nil {
		t.Fatalf("Create task 1 failed: %v", err)
	}

	info2, err := manager.Create("list-task-2")
	if err != nil {
		t.Fatalf("Create task 2 failed: %v", err)
	}

	// List all worktrees
	worktrees, err := manager.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Should have 3 total: main worktree + 2 task worktrees
	if len(worktrees) != 3 {
		t.Errorf("expected 3 worktrees, got %d", len(worktrees))
	}

	// Verify both task worktrees are in the list
	found1 := false
	found2 := false
	for _, wt := range worktrees {
		if wt.Branch == info1.Branch {
			found1 = true
			// Resolve symlinks for path comparison (macOS /private prefix)
			expectedPath, _ := filepath.EvalSymlinks(info1.Path)
			actualPath, _ := filepath.EvalSymlinks(wt.Path)
			if expectedPath == "" {
				expectedPath = info1.Path
			}
			if actualPath == "" {
				actualPath = wt.Path
			}
			if actualPath != expectedPath {
				t.Errorf("task 1 path mismatch: expected %s, got %s", expectedPath, actualPath)
			}
			if wt.TaskID != info1.TaskID {
				t.Errorf("task 1 ID mismatch: expected %s, got %s", info1.TaskID, wt.TaskID)
			}
		}
		if wt.Branch == info2.Branch {
			found2 = true
			// Resolve symlinks for path comparison (macOS /private prefix)
			expectedPath, _ := filepath.EvalSymlinks(info2.Path)
			actualPath, _ := filepath.EvalSymlinks(wt.Path)
			if expectedPath == "" {
				expectedPath = info2.Path
			}
			if actualPath == "" {
				actualPath = wt.Path
			}
			if actualPath != expectedPath {
				t.Errorf("task 2 path mismatch: expected %s, got %s", expectedPath, actualPath)
			}
			if wt.TaskID != info2.TaskID {
				t.Errorf("task 2 ID mismatch: expected %s, got %s", info2.TaskID, wt.TaskID)
			}
		}
	}

	if !found1 {
		t.Errorf("task 1 worktree not found in list")
	}
	if !found2 {
		t.Errorf("task 2 worktree not found in list")
	}
}
