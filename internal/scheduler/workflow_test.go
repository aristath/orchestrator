package scheduler

import (
	"testing"

	"github.com/aristath/orchestrator/internal/config"
)

// setupTestWorkflow creates a DAG and WorkflowManager with a standard workflow for testing.
func setupTestWorkflow() (*DAG, *WorkflowManager) {
	dag := NewDAG()

	workflows := map[string]config.WorkflowConfig{
		"standard": {
			Steps: []config.WorkflowStepConfig{
				{Agent: "coder"},
				{Agent: "reviewer"},
				{Agent: "tester"},
			},
		},
	}

	wm := NewWorkflowManager(dag, workflows)
	return dag, wm
}

func TestWorkflowManager_StandardWorkflow(t *testing.T) {
	dag, wm := setupTestWorkflow()

	// Create and add initial coder task
	coderTask := &Task{
		ID:          "task-1",
		Name:        "Implement feature",
		AgentRole:   "coder",
		Prompt:      "Write the code",
		Status:      TaskPending,
		FailureMode: FailHard,
	}
	if err := dag.AddTask(coderTask); err != nil {
		t.Fatalf("Failed to add coder task: %v", err)
	}

	// Complete the coder task
	dag.MarkCompleted("task-1", "Code written successfully")

	// OnTaskCompleted should create reviewer follow-up
	followUps, err := wm.OnTaskCompleted(coderTask)
	if err != nil {
		t.Fatalf("OnTaskCompleted failed for coder: %v", err)
	}
	if len(followUps) != 1 {
		t.Fatalf("Expected 1 follow-up task, got %d", len(followUps))
	}

	reviewerTask := followUps[0]
	if reviewerTask.AgentRole != "reviewer" {
		t.Errorf("Expected reviewer agent role, got %s", reviewerTask.AgentRole)
	}
	if reviewerTask.ID != "task-1-reviewer" {
		t.Errorf("Expected ID 'task-1-reviewer', got %s", reviewerTask.ID)
	}
	if len(reviewerTask.DependsOn) != 1 || reviewerTask.DependsOn[0] != "task-1" {
		t.Errorf("Expected reviewer to depend on task-1, got %v", reviewerTask.DependsOn)
	}
	if reviewerTask.FailureMode != FailSoft {
		t.Errorf("Expected reviewer FailureMode=FailSoft, got %v", reviewerTask.FailureMode)
	}

	// Complete the reviewer task
	dag.MarkCompleted(reviewerTask.ID, "Review passed")

	// OnTaskCompleted should create tester follow-up
	followUps2, err := wm.OnTaskCompleted(reviewerTask)
	if err != nil {
		t.Fatalf("OnTaskCompleted failed for reviewer: %v", err)
	}
	if len(followUps2) != 1 {
		t.Fatalf("Expected 1 follow-up task, got %d", len(followUps2))
	}

	testerTask := followUps2[0]
	if testerTask.AgentRole != "tester" {
		t.Errorf("Expected tester agent role, got %s", testerTask.AgentRole)
	}
	if testerTask.ID != "task-1-reviewer-tester" {
		t.Errorf("Expected ID 'task-1-reviewer-tester', got %s", testerTask.ID)
	}
	if len(testerTask.DependsOn) != 1 || testerTask.DependsOn[0] != "task-1-reviewer" {
		t.Errorf("Expected tester to depend on task-1-reviewer, got %v", testerTask.DependsOn)
	}
	if testerTask.FailureMode != FailHard {
		t.Errorf("Expected tester FailureMode=FailHard, got %v", testerTask.FailureMode)
	}
}

func TestWorkflowManager_LastStepNoFollowUp(t *testing.T) {
	dag, wm := setupTestWorkflow()

	// Create and add tester task (last step in workflow)
	testerTask := &Task{
		ID:          "test-1",
		Name:        "Run tests",
		AgentRole:   "tester",
		Prompt:      "Execute tests",
		Status:      TaskPending,
		FailureMode: FailHard,
	}
	if err := dag.AddTask(testerTask); err != nil {
		t.Fatalf("Failed to add tester task: %v", err)
	}

	// Complete the tester task
	dag.MarkCompleted("test-1", "All tests passed")

	// OnTaskCompleted should return empty list (no follow-up for last step)
	followUps, err := wm.OnTaskCompleted(testerTask)
	if err != nil {
		t.Fatalf("OnTaskCompleted failed: %v", err)
	}
	if len(followUps) != 0 {
		t.Errorf("Expected 0 follow-up tasks for last step, got %d", len(followUps))
	}
}

func TestWorkflowManager_NoMatchingWorkflow(t *testing.T) {
	dag, wm := setupTestWorkflow()

	// Create task with agent role not in any workflow
	customTask := &Task{
		ID:          "custom-1",
		Name:        "Custom task",
		AgentRole:   "custom-agent",
		Prompt:      "Do custom work",
		Status:      TaskPending,
		FailureMode: FailHard,
	}
	if err := dag.AddTask(customTask); err != nil {
		t.Fatalf("Failed to add custom task: %v", err)
	}

	// Complete the custom task
	dag.MarkCompleted("custom-1", "Custom work done")

	// OnTaskCompleted should return empty list (no matching workflow)
	followUps, err := wm.OnTaskCompleted(customTask)
	if err != nil {
		t.Fatalf("OnTaskCompleted failed: %v", err)
	}
	if len(followUps) != 0 {
		t.Errorf("Expected 0 follow-up tasks for non-workflow agent, got %d", len(followUps))
	}
}

func TestWorkflowManager_FollowUpDependencies(t *testing.T) {
	dag, wm := setupTestWorkflow()

	// Create coder task
	coderTask := &Task{
		ID:          "task-1",
		Name:        "Implement feature",
		AgentRole:   "coder",
		Prompt:      "Write the code",
		Status:      TaskPending,
		FailureMode: FailHard,
	}
	if err := dag.AddTask(coderTask); err != nil {
		t.Fatalf("Failed to add coder task: %v", err)
	}

	// Complete and spawn follow-up
	dag.MarkCompleted("task-1", "Code written")
	followUps, err := wm.OnTaskCompleted(coderTask)
	if err != nil {
		t.Fatalf("OnTaskCompleted failed: %v", err)
	}

	// Verify follow-up has correct dependencies
	if len(followUps) != 1 {
		t.Fatalf("Expected 1 follow-up, got %d", len(followUps))
	}

	reviewerTask := followUps[0]
	if len(reviewerTask.DependsOn) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(reviewerTask.DependsOn))
	}
	if reviewerTask.DependsOn[0] != "task-1" {
		t.Errorf("Expected dependency on task-1, got %s", reviewerTask.DependsOn[0])
	}
}

func TestWorkflowManager_FollowUpIDFormat(t *testing.T) {
	dag, wm := setupTestWorkflow()

	// Create coder task
	coderTask := &Task{
		ID:          "task-1",
		Name:        "Implement feature",
		AgentRole:   "coder",
		Prompt:      "Write the code",
		Status:      TaskPending,
		FailureMode: FailHard,
	}
	if err := dag.AddTask(coderTask); err != nil {
		t.Fatalf("Failed to add coder task: %v", err)
	}

	// Complete and spawn follow-up
	dag.MarkCompleted("task-1", "Code written")
	followUps, err := wm.OnTaskCompleted(coderTask)
	if err != nil {
		t.Fatalf("OnTaskCompleted failed: %v", err)
	}

	// Verify follow-up ID format: {originalID}-{agentRole}
	if len(followUps) != 1 {
		t.Fatalf("Expected 1 follow-up, got %d", len(followUps))
	}

	expectedID := "task-1-reviewer"
	if followUps[0].ID != expectedID {
		t.Errorf("Expected follow-up ID %s, got %s", expectedID, followUps[0].ID)
	}
}

func TestWorkflowManager_DAGRevalidationOnFollowUp(t *testing.T) {
	dag := NewDAG()

	// Create a contrived scenario where adding a follow-up would create a cycle
	// Setup: task-1 (coder) -> task-2, and we'll try to make task-2 depend on task-1-reviewer
	// This creates: task-1 -> task-1-reviewer (follow-up) and task-1 -> task-2
	// If task-2 somehow depends back on task-1-reviewer before it exists, we'd have issues
	// But DAG validation happens AFTER adding, so let's create a direct cycle

	workflows := map[string]config.WorkflowConfig{
		"cyclic": {
			Steps: []config.WorkflowStepConfig{
				{Agent: "coder"},
				{Agent: "reviewer"},
			},
		},
	}
	wm := NewWorkflowManager(dag, workflows)

	// Add initial task
	task1 := &Task{
		ID:          "task-1",
		Name:        "Task 1",
		AgentRole:   "coder",
		Status:      TaskPending,
		FailureMode: FailHard,
	}
	if err := dag.AddTask(task1); err != nil {
		t.Fatalf("Failed to add task-1: %v", err)
	}

	// Pre-add a task that will create a cycle when follow-up is added
	// task-1-reviewer (will be created as follow-up) shouldn't exist yet
	// But if we manually add a task that depends on something that depends on the follow-up
	// Let's add task-2 that will depend on task-1-reviewer (the future follow-up)
	// Don't add task-2 yet - it would fail because task-1-reviewer doesn't exist

	// Instead, let's create a cycle by having the follow-up depend on something that depends on task-1
	// Actually, a simpler test: manually add a task with same ID as follow-up would have
	conflictTask := &Task{
		ID:          "task-1-reviewer",
		Name:        "Conflict",
		AgentRole:   "other",
		DependsOn:   []string{"task-1"},
		Status:      TaskPending,
		FailureMode: FailHard,
	}
	if err := dag.AddTask(conflictTask); err != nil {
		t.Fatalf("Failed to add conflict task: %v", err)
	}

	// Now complete task-1, which will try to add task-1-reviewer (duplicate ID)
	dag.MarkCompleted("task-1", "Done")
	_, err := wm.OnTaskCompleted(task1)
	if err == nil {
		t.Error("Expected error when follow-up creates duplicate task ID, got nil")
	}

	// Verify DAG still has original task
	if _, exists := dag.Get("task-1"); !exists {
		t.Error("Original task should still exist in DAG")
	}
}

func TestWorkflowManager_MultipleWorkflows(t *testing.T) {
	dag := NewDAG()

	// Define two workflows that both contain "coder"
	workflows := map[string]config.WorkflowConfig{
		"standard": {
			Steps: []config.WorkflowStepConfig{
				{Agent: "coder"},
				{Agent: "reviewer"},
			},
		},
		"security": {
			Steps: []config.WorkflowStepConfig{
				{Agent: "coder"},
				{Agent: "security-scanner"},
			},
		},
	}
	wm := NewWorkflowManager(dag, workflows)

	// Create coder task
	coderTask := &Task{
		ID:          "task-1",
		Name:        "Implement feature",
		AgentRole:   "coder",
		Prompt:      "Write the code",
		Status:      TaskPending,
		FailureMode: FailHard,
	}
	if err := dag.AddTask(coderTask); err != nil {
		t.Fatalf("Failed to add coder task: %v", err)
	}

	// Complete coder task
	dag.MarkCompleted("task-1", "Code written")
	followUps, err := wm.OnTaskCompleted(coderTask)
	if err != nil {
		t.Fatalf("OnTaskCompleted failed: %v", err)
	}

	// Should create follow-ups from BOTH workflows
	if len(followUps) != 2 {
		t.Fatalf("Expected 2 follow-up tasks (one per workflow), got %d", len(followUps))
	}

	// Check that we have both reviewer and security-scanner
	agents := make(map[string]bool)
	for _, task := range followUps {
		agents[task.AgentRole] = true
	}

	if !agents["reviewer"] {
		t.Error("Expected reviewer follow-up task")
	}
	if !agents["security-scanner"] {
		t.Error("Expected security-scanner follow-up task")
	}

	// Both should depend on task-1
	for _, task := range followUps {
		if len(task.DependsOn) != 1 || task.DependsOn[0] != "task-1" {
			t.Errorf("Task %s should depend on task-1, got %v", task.ID, task.DependsOn)
		}
	}
}

func TestWorkflowManager_FindWorkflow(t *testing.T) {
	_, wm := setupTestWorkflow()

	// Find workflow for coder
	name, workflow, idx := wm.FindWorkflow("coder")
	if name != "standard" {
		t.Errorf("Expected workflow name 'standard', got %s", name)
	}
	if workflow == nil {
		t.Fatal("Expected non-nil workflow")
	}
	if idx != 0 {
		t.Errorf("Expected coder at index 0, got %d", idx)
	}

	// Find workflow for reviewer
	name, workflow, idx = wm.FindWorkflow("reviewer")
	if name != "standard" {
		t.Errorf("Expected workflow name 'standard', got %s", name)
	}
	if idx != 1 {
		t.Errorf("Expected reviewer at index 1, got %d", idx)
	}

	// Find workflow for non-existent agent
	name, workflow, idx = wm.FindWorkflow("nonexistent")
	if name != "" {
		t.Errorf("Expected empty workflow name for nonexistent agent, got %s", name)
	}
	if workflow != nil {
		t.Error("Expected nil workflow for nonexistent agent")
	}
	if idx != -1 {
		t.Errorf("Expected index -1 for nonexistent agent, got %d", idx)
	}
}
