package worktree

// MergeStrategy defines how to merge a worktree branch back to the base branch
type MergeStrategy int

const (
	// MergeOrt uses the default ort strategy (Ostensibly Recursive's Twin)
	MergeOrt MergeStrategy = iota
	// MergeOurs uses the ours strategy (always favor our changes)
	MergeOurs
	// MergeTheirs uses the theirs strategy (always favor their changes)
	MergeTheirs
)

// String returns the git merge strategy name
func (s MergeStrategy) String() string {
	switch s {
	case MergeOrt:
		return "ort"
	case MergeOurs:
		return "ours"
	case MergeTheirs:
		return "theirs"
	default:
		return "ort"
	}
}

// WorktreeInfo holds information about a created worktree
type WorktreeInfo struct {
	Path   string // Absolute path to the worktree directory
	Branch string // Branch name (e.g., "task/task-123")
	TaskID string // Original task ID
	Head   string // Current HEAD commit hash
}

// MergeResult represents the outcome of a merge operation
type MergeResult struct {
	Merged        bool     // True if merge succeeded
	ConflictFiles []string // List of files with conflicts (if any)
	Error         error    // Error if merge failed
}

// WorktreeManagerConfig configures the worktree manager
type WorktreeManagerConfig struct {
	RepoPath        string        // Absolute path to the git repository
	BaseBranch      string        // Base branch to branch from (e.g., "main")
	WorktreeDir     string        // Directory under repo for worktrees (default ".worktrees")
	DefaultStrategy MergeStrategy // Default merge strategy
}
