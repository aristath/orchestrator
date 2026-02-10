package scheduler

import (
	"sort"
	"sync"
)

// ResourceLockManager provides per-file mutual exclusion for concurrent task execution.
// Uses a keyed mutex pattern: each file path gets its own mutex, allowing concurrent
// writes to different files while blocking concurrent writes to the same file.
type ResourceLockManager struct {
	mu    sync.Mutex           // Guards the locks map itself
	locks map[string]*sync.Mutex // Per-file mutexes
}

// NewResourceLockManager creates a new ResourceLockManager.
func NewResourceLockManager() *ResourceLockManager {
	return &ResourceLockManager{
		locks: make(map[string]*sync.Mutex),
	}
}

// Lock acquires the per-file mutex for the given filepath.
// Creates the mutex on first access if it doesn't exist.
func (r *ResourceLockManager) Lock(filepath string) {
	r.mu.Lock()
	// Get or create the per-file mutex
	fileLock, exists := r.locks[filepath]
	if !exists {
		fileLock = &sync.Mutex{}
		r.locks[filepath] = fileLock
	}
	r.mu.Unlock()

	// Acquire the per-file lock (outside the manager lock to avoid contention)
	fileLock.Lock()
}

// Unlock releases the per-file mutex for the given filepath.
func (r *ResourceLockManager) Unlock(filepath string) {
	r.mu.Lock()
	fileLock, exists := r.locks[filepath]
	r.mu.Unlock()

	if exists {
		fileLock.Unlock()
	}
}

// LockAll acquires locks for ALL given filepaths.
// CRITICAL: sorts filepaths lexicographically BEFORE acquiring to prevent deadlocks.
// Acquires locks in sorted order.
func (r *ResourceLockManager) LockAll(filepaths []string) {
	if len(filepaths) == 0 {
		return
	}

	// Create a sorted copy to avoid modifying the original slice
	sorted := make([]string, len(filepaths))
	copy(sorted, filepaths)
	sort.Strings(sorted)

	// Acquire locks in sorted order
	for _, filepath := range sorted {
		r.Lock(filepath)
	}
}

// UnlockAll releases locks for all given filepaths.
// Releases in reverse sorted order for symmetry with LockAll.
func (r *ResourceLockManager) UnlockAll(filepaths []string) {
	if len(filepaths) == 0 {
		return
	}

	// Create a sorted copy
	sorted := make([]string, len(filepaths))
	copy(sorted, filepaths)
	sort.Strings(sorted)

	// Release locks in reverse sorted order
	for i := len(sorted) - 1; i >= 0; i-- {
		r.Unlock(sorted[i])
	}
}
