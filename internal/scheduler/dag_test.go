package scheduler

import (
	"errors"
	"strings"
	"testing"
)

// TestDAGValidate tests DAG validation with various graph structures.
func TestDAGValidate(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() *DAG
		wantErr     bool
		errContains string
	}{
		{
			name: "valid linear chain",
			setup: func() *DAG {
				dag := NewDAG()
				dag.AddTask(&Task{ID: "A", DependsOn: []string{}})
				dag.AddTask(&Task{ID: "B", DependsOn: []string{"A"}})
				dag.AddTask(&Task{ID: "C", DependsOn: []string{"B"}})
				return dag
			},
			wantErr: false,
		},
		{
			name: "valid parallel tasks",
			setup: func() *DAG {
				dag := NewDAG()
				dag.AddTask(&Task{ID: "A", DependsOn: []string{}})
				dag.AddTask(&Task{ID: "B", DependsOn: []string{}})
				dag.AddTask(&Task{ID: "C", DependsOn: []string{"A", "B"}})
				return dag
			},
			wantErr: false,
		},
		{
			name: "single task no deps",
			setup: func() *DAG {
				dag := NewDAG()
				dag.AddTask(&Task{ID: "A", DependsOn: []string{}})
				return dag
			},
			wantErr: false,
		},
		{
			name: "direct cycle",
			setup: func() *DAG {
				dag := NewDAG()
				dag.AddTask(&Task{ID: "A", DependsOn: []string{"B"}})
				dag.AddTask(&Task{ID: "B", DependsOn: []string{"A"}})
				return dag
			},
			wantErr:     true,
			errContains: "cycle",
		},
		{
			name: "transitive cycle",
			setup: func() *DAG {
				dag := NewDAG()
				dag.AddTask(&Task{ID: "A", DependsOn: []string{"B"}})
				dag.AddTask(&Task{ID: "B", DependsOn: []string{"C"}})
				dag.AddTask(&Task{ID: "C", DependsOn: []string{"A"}})
				return dag
			},
			wantErr:     true,
			errContains: "cycle",
		},
		{
			name: "self-loop",
			setup: func() *DAG {
				dag := NewDAG()
				dag.AddTask(&Task{ID: "A", DependsOn: []string{"A"}})
				return dag
			},
			wantErr:     true,
			errContains: "cycle",
		},
		{
			name: "missing dependency",
			setup: func() *DAG {
				dag := NewDAG()
				dag.AddTask(&Task{ID: "A", DependsOn: []string{"nonexistent"}})
				return dag
			},
			wantErr:     true,
			errContains: "nonexistent",
		},
		{
			name: "duplicate task ID",
			setup: func() *DAG {
				dag := NewDAG()
				dag.AddTask(&Task{ID: "A", DependsOn: []string{}})
				// Attempting to add the same ID again should fail at AddTask
				err := dag.AddTask(&Task{ID: "A", DependsOn: []string{}})
				if err == nil {
					t.Fatal("Expected error when adding duplicate task ID")
				}
				return dag
			},
			wantErr: false, // Validate should succeed since duplicate was rejected
		},
		{
			name: "disconnected components",
			setup: func() *DAG {
				dag := NewDAG()
				// Component 1: A -> B
				dag.AddTask(&Task{ID: "A", DependsOn: []string{}})
				dag.AddTask(&Task{ID: "B", DependsOn: []string{"A"}})
				// Component 2: C -> D
				dag.AddTask(&Task{ID: "C", DependsOn: []string{}})
				dag.AddTask(&Task{ID: "D", DependsOn: []string{"C"}})
				return dag
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dag := tt.setup()
			order, err := dag.Validate()

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Error message %q doesn't contain %q", err.Error(), tt.errContains)
				}
			}

			// For successful validation, verify order contains expected tasks
			if err == nil && tt.name == "disconnected components" {
				if len(order) != 4 {
					t.Errorf("Expected 4 tasks in order, got %d: %v", len(order), order)
				}
			}
		})
	}
}

// TestDAGEligible tests dependency resolution and task eligibility.
func TestDAGEligible(t *testing.T) {
	tests := []struct {
		name          string
		setup         func() *DAG
		expectedCount int
		expectedIDs   []string
	}{
		{
			name: "initial eligible",
			setup: func() *DAG {
				dag := NewDAG()
				dag.AddTask(&Task{ID: "A", DependsOn: []string{}, Status: TaskPending})
				dag.AddTask(&Task{ID: "B", DependsOn: []string{}, Status: TaskPending})
				dag.AddTask(&Task{ID: "C", DependsOn: []string{"A"}, Status: TaskPending})
				dag.Validate()
				return dag
			},
			expectedCount: 2,
			expectedIDs:   []string{"A", "B"},
		},
		{
			name: "completion unlocks dependents",
			setup: func() *DAG {
				dag := NewDAG()
				taskA := &Task{ID: "A", DependsOn: []string{}, Status: TaskCompleted}
				taskB := &Task{ID: "B", DependsOn: []string{"A"}, Status: TaskPending}
				dag.AddTask(taskA)
				dag.AddTask(taskB)
				dag.Validate()
				return dag
			},
			expectedCount: 1,
			expectedIDs:   []string{"B"},
		},
		{
			name: "partial completion",
			setup: func() *DAG {
				dag := NewDAG()
				taskA := &Task{ID: "A", DependsOn: []string{}, Status: TaskCompleted}
				taskB := &Task{ID: "B", DependsOn: []string{}, Status: TaskPending}
				taskC := &Task{ID: "C", DependsOn: []string{"A", "B"}, Status: TaskPending}
				dag.AddTask(taskA)
				dag.AddTask(taskB)
				dag.AddTask(taskC)
				dag.Validate()
				return dag
			},
			expectedCount: 1,
			expectedIDs:   []string{"B"}, // C is not eligible yet
		},
		{
			name: "hard failure blocks",
			setup: func() *DAG {
				dag := NewDAG()
				taskA := &Task{ID: "A", DependsOn: []string{}, Status: TaskFailed, FailureMode: FailHard}
				taskB := &Task{ID: "B", DependsOn: []string{"A"}, Status: TaskPending}
				dag.AddTask(taskA)
				dag.AddTask(taskB)
				dag.Validate()
				return dag
			},
			expectedCount: 0,
			expectedIDs:   []string{},
		},
		{
			name: "soft failure allows",
			setup: func() *DAG {
				dag := NewDAG()
				taskA := &Task{ID: "A", DependsOn: []string{}, Status: TaskFailed, FailureMode: FailSoft}
				taskB := &Task{ID: "B", DependsOn: []string{"A"}, Status: TaskPending}
				dag.AddTask(taskA)
				dag.AddTask(taskB)
				dag.Validate()
				return dag
			},
			expectedCount: 1,
			expectedIDs:   []string{"B"},
		},
		{
			name: "skip treated as success",
			setup: func() *DAG {
				dag := NewDAG()
				taskA := &Task{ID: "A", DependsOn: []string{}, Status: TaskSkipped}
				taskB := &Task{ID: "B", DependsOn: []string{"A"}, Status: TaskPending}
				dag.AddTask(taskA)
				dag.AddTask(taskB)
				dag.Validate()
				return dag
			},
			expectedCount: 1,
			expectedIDs:   []string{"B"},
		},
		{
			name: "fail skip treated as success",
			setup: func() *DAG {
				dag := NewDAG()
				taskA := &Task{ID: "A", DependsOn: []string{}, Status: TaskFailed, FailureMode: FailSkip}
				taskB := &Task{ID: "B", DependsOn: []string{"A"}, Status: TaskPending}
				dag.AddTask(taskA)
				dag.AddTask(taskB)
				dag.Validate()
				return dag
			},
			expectedCount: 1,
			expectedIDs:   []string{"B"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dag := tt.setup()
			eligible := dag.Eligible()

			if len(eligible) != tt.expectedCount {
				t.Errorf("Eligible() returned %d tasks, expected %d", len(eligible), tt.expectedCount)
			}

			// Verify expected IDs are present
			foundIDs := make(map[string]bool)
			for _, task := range eligible {
				foundIDs[task.ID] = true
			}

			for _, expectedID := range tt.expectedIDs {
				if !foundIDs[expectedID] {
					t.Errorf("Expected task %q to be eligible, but it wasn't", expectedID)
				}
			}
		})
	}
}

// TestDAGMarkTransitions tests state transition methods.
func TestDAGMarkTransitions(t *testing.T) {
	t.Run("MarkRunning on eligible task succeeds", func(t *testing.T) {
		dag := NewDAG()
		dag.AddTask(&Task{ID: "A", DependsOn: []string{}, Status: TaskPending})
		dag.Validate()

		err := dag.MarkRunning("A")
		if err != nil {
			t.Errorf("MarkRunning() error = %v, want nil", err)
		}

		task, _ := dag.Get("A")
		if task.Status != TaskRunning {
			t.Errorf("Task status = %v, want TaskRunning", task.Status)
		}
	})

	t.Run("MarkCompleted stores result", func(t *testing.T) {
		dag := NewDAG()
		dag.AddTask(&Task{ID: "A", DependsOn: []string{}, Status: TaskRunning})

		result := "task completed successfully"
		err := dag.MarkCompleted("A", result)
		if err != nil {
			t.Errorf("MarkCompleted() error = %v, want nil", err)
		}

		task, _ := dag.Get("A")
		if task.Status != TaskCompleted {
			t.Errorf("Task status = %v, want TaskCompleted", task.Status)
		}
		if task.Result != result {
			t.Errorf("Task result = %q, want %q", task.Result, result)
		}
	})

	t.Run("MarkFailed stores error", func(t *testing.T) {
		dag := NewDAG()
		dag.AddTask(&Task{ID: "A", DependsOn: []string{}, Status: TaskRunning, FailureMode: FailHard})

		testErr := errors.New("task failed")
		err := dag.MarkFailed("A", testErr)
		if err != nil {
			t.Errorf("MarkFailed() error = %v, want nil", err)
		}

		task, _ := dag.Get("A")
		if task.Status != TaskFailed {
			t.Errorf("Task status = %v, want TaskFailed", task.Status)
		}
		if task.Error != testErr {
			t.Errorf("Task error = %v, want %v", task.Error, testErr)
		}
	})

	t.Run("MarkRunning on non-existent task returns error", func(t *testing.T) {
		dag := NewDAG()

		err := dag.MarkRunning("nonexistent")
		if err == nil {
			t.Error("MarkRunning() error = nil, want error")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Error message %q doesn't contain 'not found'", err.Error())
		}
	})

	t.Run("Get returns task and exists flag", func(t *testing.T) {
		dag := NewDAG()
		dag.AddTask(&Task{ID: "A", Name: "Task A"})

		task, exists := dag.Get("A")
		if !exists {
			t.Error("Get() exists = false, want true")
		}
		if task.Name != "Task A" {
			t.Errorf("Task name = %q, want %q", task.Name, "Task A")
		}

		_, exists = dag.Get("nonexistent")
		if exists {
			t.Error("Get() exists = true for nonexistent task, want false")
		}
	})

	t.Run("Tasks returns all tasks", func(t *testing.T) {
		dag := NewDAG()
		dag.AddTask(&Task{ID: "A"})
		dag.AddTask(&Task{ID: "B"})
		dag.AddTask(&Task{ID: "C"})

		tasks := dag.Tasks()
		if len(tasks) != 3 {
			t.Errorf("Tasks() returned %d tasks, want 3", len(tasks))
		}
	})

	t.Run("Order returns same as Validate", func(t *testing.T) {
		dag := NewDAG()
		dag.AddTask(&Task{ID: "A", DependsOn: []string{}})
		dag.AddTask(&Task{ID: "B", DependsOn: []string{"A"}})

		order1, err1 := dag.Order()
		order2, err2 := dag.Validate()

		if err1 != nil || err2 != nil {
			t.Errorf("Order() or Validate() returned error")
		}

		if len(order1) != len(order2) {
			t.Errorf("Order length mismatch: %d vs %d", len(order1), len(order2))
		}

		for i := range order1 {
			if order1[i] != order2[i] {
				t.Errorf("Order mismatch at index %d: %q vs %q", i, order1[i], order2[i])
			}
		}
	})
}

// TestDAGComplexScenarios tests more complex real-world scenarios.
func TestDAGComplexScenarios(t *testing.T) {
	t.Run("diamond dependency pattern", func(t *testing.T) {
		// A -> B -> D
		// A -> C -> D
		// D depends on both B and C, which both depend on A
		dag := NewDAG()
		dag.AddTask(&Task{ID: "A", DependsOn: []string{}, Status: TaskPending})
		dag.AddTask(&Task{ID: "B", DependsOn: []string{"A"}, Status: TaskPending})
		dag.AddTask(&Task{ID: "C", DependsOn: []string{"A"}, Status: TaskPending})
		dag.AddTask(&Task{ID: "D", DependsOn: []string{"B", "C"}, Status: TaskPending})

		order, err := dag.Validate()
		if err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}

		// A should come first, D should come last, B and C in between
		if order[0] != "A" {
			t.Errorf("First task should be A, got %s", order[0])
		}
		if order[len(order)-1] != "D" {
			t.Errorf("Last task should be D, got %s", order[len(order)-1])
		}

		// Initially only A is eligible
		eligible := dag.Eligible()
		if len(eligible) != 1 || eligible[0].ID != "A" {
			t.Errorf("Initially only A should be eligible")
		}

		// Complete A, then B and C become eligible
		dag.MarkCompleted("A", "done")
		eligible = dag.Eligible()
		if len(eligible) != 2 {
			t.Errorf("After A completes, B and C should be eligible, got %d tasks", len(eligible))
		}

		// Complete B and C, then D becomes eligible
		dag.MarkCompleted("B", "done")
		dag.MarkCompleted("C", "done")
		eligible = dag.Eligible()
		if len(eligible) != 1 || eligible[0].ID != "D" {
			t.Errorf("After B and C complete, D should be eligible")
		}
	})

	t.Run("mixed failure modes", func(t *testing.T) {
		// A (will fail hard) -> B (should block)
		// C (will fail soft) -> D (should allow)
		// E (will fail skip) -> F (should allow)
		dag := NewDAG()
		dag.AddTask(&Task{ID: "A", DependsOn: []string{}, Status: TaskFailed, FailureMode: FailHard})
		dag.AddTask(&Task{ID: "B", DependsOn: []string{"A"}, Status: TaskPending})
		dag.AddTask(&Task{ID: "C", DependsOn: []string{}, Status: TaskFailed, FailureMode: FailSoft})
		dag.AddTask(&Task{ID: "D", DependsOn: []string{"C"}, Status: TaskPending})
		dag.AddTask(&Task{ID: "E", DependsOn: []string{}, Status: TaskFailed, FailureMode: FailSkip})
		dag.AddTask(&Task{ID: "F", DependsOn: []string{"E"}, Status: TaskPending})

		eligible := dag.Eligible()

		// B should NOT be eligible (A failed hard)
		// D should be eligible (C failed soft)
		// F should be eligible (E failed skip)
		if len(eligible) != 2 {
			t.Errorf("Expected 2 eligible tasks (D and F), got %d", len(eligible))
		}

		eligibleIDs := make(map[string]bool)
		for _, task := range eligible {
			eligibleIDs[task.ID] = true
		}

		if eligibleIDs["B"] {
			t.Error("Task B should NOT be eligible (dependency failed hard)")
		}
		if !eligibleIDs["D"] {
			t.Error("Task D should be eligible (dependency failed soft)")
		}
		if !eligibleIDs["F"] {
			t.Error("Task F should be eligible (dependency failed skip)")
		}
	})
}
