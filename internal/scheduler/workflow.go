package scheduler

import (
	"fmt"

	"github.com/aristath/orchestrator/internal/config"
)

// WorkflowManager handles spawning follow-up tasks based on workflow configuration.
// When a task completes, it checks if the task's agent role is a step in any
// configured workflow, and if so, creates the next step's task in the DAG.
type WorkflowManager struct {
	dag       *DAG
	workflows map[string]config.WorkflowConfig // workflow name -> config
}

// NewWorkflowManager creates a new WorkflowManager.
func NewWorkflowManager(dag *DAG, workflows map[string]config.WorkflowConfig) *WorkflowManager {
	return &WorkflowManager{
		dag:       dag,
		workflows: workflows,
	}
}

// OnTaskCompleted is the main hook called after a task completes.
// It finds which workflow(s) the completed task's AgentRole participates in,
// creates follow-up tasks for the next step(s), adds them to the DAG,
// and re-validates to catch cycles.
func (wm *WorkflowManager) OnTaskCompleted(completedTask *Task) ([]*Task, error) {
	var newTasks []*Task

	// Find all workflows that contain the completed task's agent role
	for workflowName, workflow := range wm.workflows {
		stepIndex := wm.findAgentStepIndex(workflow, completedTask.AgentRole)
		if stepIndex == -1 {
			// This workflow doesn't contain the completed task's agent role
			continue
		}

		// Check if there's a next step
		if stepIndex >= len(workflow.Steps)-1 {
			// This is the last step in the workflow, no follow-up needed
			continue
		}

		// Get the next step
		nextStep := workflow.Steps[stepIndex+1]
		nextAgent := nextStep.Agent

		// Create follow-up task
		followUpTask := &Task{
			ID:          fmt.Sprintf("%s-%s", completedTask.ID, nextAgent),
			Name:        fmt.Sprintf("Follow-up: %s after %s", nextAgent, completedTask.ID),
			AgentRole:   nextAgent,
			Prompt:      fmt.Sprintf("Review the output of task %s: %s", completedTask.ID, completedTask.Result),
			DependsOn:   []string{completedTask.ID},
			WritesFiles: completedTask.WritesFiles, // Follow-up may write to same files
			Status:      TaskPending,
			FailureMode: wm.determineFailureMode(nextAgent),
		}

		// Try to add the task to the DAG
		if err := wm.dag.AddTask(followUpTask); err != nil {
			return newTasks, fmt.Errorf("failed to add follow-up task for workflow %q: %w", workflowName, err)
		}

		// Re-validate the DAG to catch any cycles
		if _, err := wm.dag.Validate(); err != nil {
			// Validation failed, remove the task we just added
			// Since we can't easily remove from the DAG, we'll return an error
			// The DAG is now in an invalid state and should be considered corrupted
			return newTasks, fmt.Errorf("adding follow-up task for workflow %q would create cycle: %w", workflowName, err)
		}

		newTasks = append(newTasks, followUpTask)
	}

	return newTasks, nil
}

// FindWorkflow returns workflow name, config, and step index for the given agent role.
// Returns empty string if not found.
func (wm *WorkflowManager) FindWorkflow(agentRole string) (string, *config.WorkflowConfig, int) {
	for name, workflow := range wm.workflows {
		stepIndex := wm.findAgentStepIndex(workflow, agentRole)
		if stepIndex != -1 {
			return name, &workflow, stepIndex
		}
	}
	return "", nil, -1
}

// findAgentStepIndex finds the index of a step with the given agent role in a workflow.
// Returns -1 if not found.
func (wm *WorkflowManager) findAgentStepIndex(workflow config.WorkflowConfig, agentRole string) int {
	for i, step := range workflow.Steps {
		if step.Agent == agentRole {
			return i
		}
	}
	return -1
}

// determineFailureMode determines the failure mode based on the agent role.
// Review tasks use FailSoft (code can proceed), test tasks use FailHard.
func (wm *WorkflowManager) determineFailureMode(agentRole string) FailureMode {
	// Simple heuristic: tester tasks are critical, reviewer tasks are not
	if agentRole == "tester" || agentRole == "test" {
		return FailHard
	}
	return FailSoft
}
