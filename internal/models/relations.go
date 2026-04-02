package models

// TaskDependency represents a blocking relationship.
// Task depends on Blocker — task_id cannot start until blocker_id completes.
type TaskDependency struct {
	ID        string `json:"id"`
	TaskID    string `json:"task_id"`
	BlockerID string `json:"blocker_id"`
	CreatedAt int64  `json:"created_at"`
}

// TaskRelations holds all relation metadata for a single task.
type TaskRelations struct {
	TaskID     string   `json:"task_id"`
	BlockerIDs []string `json:"blocker_ids"` // tasks this task depends on
	Dependents []string `json:"dependents"`   // tasks that depend on this one
	SubtaskIDs []string `json:"subtask_ids"` // direct children
	ParentID   string   `json:"parent_id"`   // "" if top-level
}

// TaskWithRelations embeds a Task with its relation metadata.
type TaskWithRelations struct {
	Task
	Relations TaskRelations `json:"relations"`
}

// CanStartResult is returned by the can-start check.
type CanStartResult struct {
	CanStart  bool     `json:"can_start"`
	Blockers  []string `json:"blockers"` // titles of incomplete blockers
	HasChildren bool   `json:"has_children"`
	ChildTitles []string `json:"child_titles,omitempty"`
}

// Event types for dependencies and subtasks
const (
	EventDependencyAdded      = "task.dependency.added"
	EventDependencyRemoved    = "task.dependency.removed"
	EventSubtaskAdded        = "task.subtask.added"
	EventSubtaskRemoved      = "task.subtask.removed"
	EventSubtaskStatusChanged = "task.subtask.status_changed"
)
