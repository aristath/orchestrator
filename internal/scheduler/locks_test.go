package scheduler

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestResourceLockManager_BasicLockUnlock verifies basic lock/unlock operations.
func TestResourceLockManager_BasicLockUnlock(t *testing.T) {
	mgr := NewResourceLockManager()

	// Lock and unlock should not panic
	mgr.Lock("main.go")
	mgr.Unlock("main.go")

	// Should be able to lock again after unlock
	mgr.Lock("main.go")
	mgr.Unlock("main.go")
}

// TestResourceLockManager_SameFileBlocks verifies that locking the same file blocks concurrent access.
func TestResourceLockManager_SameFileBlocks(t *testing.T) {
	mgr := NewResourceLockManager()
	orderChan := make(chan int, 2)

	// Goroutine A locks "main.go" first
	go func() {
		mgr.Lock("main.go")
		orderChan <- 1
		time.Sleep(50 * time.Millisecond) // Hold the lock briefly
		mgr.Unlock("main.go")
	}()

	// Give goroutine A time to acquire the lock
	time.Sleep(10 * time.Millisecond)

	// Goroutine B tries to lock "main.go" - should block
	go func() {
		mgr.Lock("main.go")
		orderChan <- 2
		mgr.Unlock("main.go")
	}()

	// Verify ordering: A acquired first, then B
	first := <-orderChan
	second := <-orderChan

	if first != 1 || second != 2 {
		t.Errorf("Expected order [1, 2], got [%d, %d]", first, second)
	}
}

// TestResourceLockManager_DifferentFilesConcurrent verifies that locking different files doesn't block.
func TestResourceLockManager_DifferentFilesConcurrent(t *testing.T) {
	mgr := NewResourceLockManager()
	var wg sync.WaitGroup
	var aLocked, bLocked atomic.Bool

	wg.Add(2)

	// Goroutine A locks "a.go"
	go func() {
		defer wg.Done()
		mgr.Lock("a.go")
		aLocked.Store(true)
		time.Sleep(20 * time.Millisecond)
		mgr.Unlock("a.go")
	}()

	// Goroutine B locks "b.go"
	go func() {
		defer wg.Done()
		mgr.Lock("b.go")
		bLocked.Store(true)
		time.Sleep(20 * time.Millisecond)
		mgr.Unlock("b.go")
	}()

	// Give both goroutines time to acquire their locks
	time.Sleep(10 * time.Millisecond)

	// Both should have acquired locks (no blocking)
	if !aLocked.Load() || !bLocked.Load() {
		t.Error("Both goroutines should have acquired their locks concurrently")
	}

	wg.Wait()
}

// TestResourceLockManager_LockAllOrdering verifies that LockAll sorts and prevents deadlocks.
func TestResourceLockManager_LockAllOrdering(t *testing.T) {
	mgr := NewResourceLockManager()
	var wg sync.WaitGroup

	// Both goroutines try to lock the same files in different orders
	// If LockAll doesn't sort, this could deadlock
	wg.Add(2)

	// Goroutine A: locks ["b.go", "a.go"]
	go func() {
		defer wg.Done()
		mgr.LockAll([]string{"b.go", "a.go"})
		time.Sleep(10 * time.Millisecond)
		mgr.UnlockAll([]string{"b.go", "a.go"})
	}()

	// Goroutine B: locks ["a.go", "b.go"]
	go func() {
		defer wg.Done()
		time.Sleep(5 * time.Millisecond) // Slight delay to ensure A acquires first
		mgr.LockAll([]string{"a.go", "b.go"})
		time.Sleep(10 * time.Millisecond)
		mgr.UnlockAll([]string{"a.go", "b.go"})
	}()

	// Wait with timeout to catch deadlocks
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - no deadlock
	case <-time.After(2 * time.Second):
		t.Fatal("Deadlock detected: LockAll did not prevent deadlock through ordering")
	}
}

// TestResourceLockManager_UnlockAllReleasesAll verifies that UnlockAll releases all locks.
func TestResourceLockManager_UnlockAllReleasesAll(t *testing.T) {
	mgr := NewResourceLockManager()

	// Lock multiple files
	files := []string{"a.go", "b.go", "c.go"}
	mgr.LockAll(files)

	// Unlock all
	mgr.UnlockAll(files)

	// Another goroutine should be able to acquire all locks
	acquired := make(chan bool, 1)
	go func() {
		mgr.LockAll(files)
		acquired <- true
		mgr.UnlockAll(files)
	}()

	select {
	case <-acquired:
		// Success - locks were released
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Locks were not fully released by UnlockAll")
	}
}

// TestResourceLockManager_EmptyFilepaths verifies that LockAll/UnlockAll handle empty slices.
func TestResourceLockManager_EmptyFilepaths(t *testing.T) {
	mgr := NewResourceLockManager()

	// Should not panic
	mgr.LockAll([]string{})
	mgr.UnlockAll([]string{})
}
