package db

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/rogerrlee/tasks-watcher/internal/models"
)

// --- Dependencies ---

// AddDependency adds a blocking relationship: task depends on blocker.
// Returns an error if the dependency already exists or would create a cycle.
func (db *DB) AddDependency(taskID, blockerID string) (*models.TaskDependency, error) {
	if taskID == blockerID {
		return nil, fmt.Errorf("a task cannot depend on itself")
	}

	// Check both tasks exist
	t, err := db.GetTask(taskID)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, fmt.Errorf("task %q not found", taskID)
	}
	b, err := db.GetTask(blockerID)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, fmt.Errorf("blocker task %q not found", blockerID)
	}

	// Check for circular dependency
	circular, err := db.checkCircularDependency(taskID, blockerID)
	if err != nil {
		return nil, err
	}
	if circular {
		return nil, fmt.Errorf("adding %q as a blocker for %q would create a circular dependency", b.Title, t.Title)
	}

	dep := &models.TaskDependency{
		ID:        uuid.New().String(),
		TaskID:    taskID,
		BlockerID: blockerID,
		CreatedAt: models.Now(),
	}
	_, err = db.conn.Exec(
		`INSERT OR IGNORE INTO task_dependencies (id, task_id, blocker_id, created_at) VALUES (?, ?, ?, ?)`,
		dep.ID, dep.TaskID, dep.BlockerID, dep.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return dep, nil
}

// RemoveDependency removes a blocking relationship.
func (db *DB) RemoveDependency(taskID, blockerID string) error {
	_, err := db.conn.Exec(
		`DELETE FROM task_dependencies WHERE task_id = ? AND blocker_id = ?`,
		taskID, blockerID,
	)
	return err
}

// GetBlockerIDs returns all task IDs that block the given task.
func (db *DB) GetBlockerIDs(taskID string) ([]string, error) {
	rows, err := db.conn.Query(
		`SELECT blocker_id FROM task_dependencies WHERE task_id = ?`, taskID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetDependentIDs returns all task IDs that depend on (are blocked by) the given task.
func (db *DB) GetDependentIDs(taskID string) ([]string, error) {
	rows, err := db.conn.Query(
		`SELECT task_id FROM task_dependencies WHERE blocker_id = ?`, taskID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetDependencyTasks returns the full blocker Task objects for a given task.
func (db *DB) GetDependencyTasks(taskID string) ([]models.Task, error) {
	rows, err := db.conn.Query(`
		SELECT t.id, t.project_id, t.title, t.description, t.status, t.priority,
		       t.assignee, t.source, t.error_message, t.heartbeat_at,
		       t.created_at, t.updated_at, t.completed_at
		FROM tasks t
		JOIN task_dependencies d ON t.id = d.blocker_id
		WHERE d.task_id = ?
		ORDER BY t.created_at
	`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTasks(rows)
}

// GetDependentTasks returns the full Task objects that depend on the given task.
func (db *DB) GetDependentTasks(taskID string) ([]models.Task, error) {
	rows, err := db.conn.Query(`
		SELECT t.id, t.project_id, t.title, t.description, t.status, t.priority,
		       t.assignee, t.source, t.error_message, t.heartbeat_at,
		       t.created_at, t.updated_at, t.completed_at
		FROM tasks t
		JOIN task_dependencies d ON t.id = d.task_id
		WHERE d.blocker_id = ?
		ORDER BY t.created_at
	`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTasks(rows)
}

// checkCircularDependency uses DFS to detect if adding blockerID as a blocker
// for taskID would create a cycle in the dependency graph.
func (db *DB) checkCircularDependency(taskID, blockerID string) (bool, error) {
	visited := make(map[string]bool)
	stack := []string{blockerID}

	for len(stack) > 0 {
		curr := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if curr == taskID {
			return true, nil
		}
		if visited[curr] {
			continue
		}
		visited[curr] = true

		blockers, err := db.GetBlockerIDs(curr)
		if err != nil {
			return false, err
		}
		stack = append(stack, blockers...)
	}
	return false, nil
}

// CanStartTask checks whether a task can transition to in_progress.
func (db *DB) CanStartTask(taskID string) (*models.CanStartResult, error) {
	result := &models.CanStartResult{CanStart: true}

	// Check incomplete blockers
	blockerIDs, err := db.GetBlockerIDs(taskID)
	if err != nil {
		return nil, err
	}
	for _, bid := range blockerIDs {
		t, err := db.GetTask(bid)
		if err != nil {
			return nil, err
		}
		if t != nil && !t.IsTerminal() {
			result.CanStart = false
			result.Blockers = append(result.Blockers, t.Title)
		}
	}

	// Check non-terminal children
	childIDs, err := db.GetSubtaskIDs(taskID)
	if err != nil {
		return nil, err
	}
	if len(childIDs) > 0 {
		result.HasChildren = true
		for _, cid := range childIDs {
			t, err := db.GetTask(cid)
			if err != nil {
				return nil, err
			}
			if t != nil && !t.IsTerminal() {
				result.CanStart = false
				result.ChildTitles = append(result.ChildTitles, t.Title)
			}
		}
	}

	return result, nil
}

// --- Subtasks ---

// AddSubtask assigns childID as a subtask of parentID.
func (db *DB) AddSubtask(parentID, childID string) (*models.Task, error) {
	if parentID == childID {
		return nil, fmt.Errorf("a task cannot be a subtask of itself")
	}

	// Check both tasks exist
	parent, err := db.GetTask(parentID)
	if err != nil {
		return nil, err
	}
	if parent == nil {
		return nil, fmt.Errorf("parent task %q not found", parentID)
	}
	child, err := db.GetTask(childID)
	if err != nil {
		return nil, err
	}
	if child == nil {
		return nil, fmt.Errorf("child task %q not found", childID)
	}

	// Check child doesn't already have a parent
	existingParent, err := db.GetParentID(childID)
	if err != nil {
		return nil, err
	}
	if existingParent != "" {
		return nil, fmt.Errorf("task %q is already a subtask of %q", child.Title, existingParent)
	}

	_, err = db.conn.Exec(
		`INSERT OR IGNORE INTO task_subtasks (id, parent_id, child_id, created_at) VALUES (?, ?, ?, ?)`,
		uuid.New().String(), parentID, childID, models.Now(),
	)
	if err != nil {
		return nil, err
	}
	return child, nil
}

// RemoveSubtask removes the parent-child relationship.
func (db *DB) RemoveSubtask(parentID, childID string) error {
	_, err := db.conn.Exec(
		`DELETE FROM task_subtasks WHERE parent_id = ? AND child_id = ?`,
		parentID, childID,
	)
	return err
}

// GetSubtaskIDs returns all child task IDs of the given parent.
func (db *DB) GetSubtaskIDs(parentID string) ([]string, error) {
	rows, err := db.conn.Query(
		`SELECT child_id FROM task_subtasks WHERE parent_id = ?`, parentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetSubtaskTasks returns full Task objects for all subtasks of the given parent.
func (db *DB) GetSubtaskTasks(parentID string) ([]models.Task, error) {
	rows, err := db.conn.Query(`
		SELECT t.id, t.project_id, t.title, t.description, t.status, t.priority,
		       t.assignee, t.source, t.error_message, t.heartbeat_at,
		       t.created_at, t.updated_at, t.completed_at
		FROM tasks t
		JOIN task_subtasks s ON t.id = s.child_id
		WHERE s.parent_id = ?
		ORDER BY t.created_at
	`, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTasks(rows)
}

// GetParentID returns the parent task ID of childID, or "" if top-level.
func (db *DB) GetParentID(childID string) (string, error) {
	var parentID sql.NullString
	err := db.conn.QueryRow(
		`SELECT parent_id FROM task_subtasks WHERE child_id = ?`, childID,
	).Scan(&parentID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if parentID.Valid {
		return parentID.String, nil
	}
	return "", nil
}

// GetParentTask returns the parent Task of childID, or nil if top-level.
func (db *DB) GetParentTask(childID string) (*models.Task, error) {
	parentID, err := db.GetParentID(childID)
	if err != nil {
		return nil, err
	}
	if parentID == "" {
		return nil, nil
	}
	return db.GetTask(parentID)
}

// GetChildStatuses returns the statuses of all direct children of a parent task.
func (db *DB) GetChildStatuses(parentID string) ([]models.TaskStatus, error) {
	rows, err := db.conn.Query(
		`SELECT t.status FROM tasks t JOIN task_subtasks s ON t.id = s.child_id WHERE s.parent_id = ?`, parentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var statuses []models.TaskStatus
	for rows.Next() {
		var s models.TaskStatus
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		statuses = append(statuses, s)
	}
	return statuses, rows.Err()
}

// ComputeParentStatus computes the parent's status from child statuses.
// If any child is in_progress → parent is in_progress.
// If all children are terminal → parent is completed.
// Otherwise → parent stays pending.
func ComputeParentStatus(childStatuses []models.TaskStatus) models.TaskStatus {
	if len(childStatuses) == 0 {
		return models.TaskStatusPending
	}
	allTerminal := true
	for _, s := range childStatuses {
		if !isTerminalStatus(s) {
			allTerminal = false
			break
		}
	}
	if allTerminal {
		return models.TaskStatusCompleted
	}
	// Any child in_progress?
	for _, s := range childStatuses {
		if s == models.TaskStatusInProgress {
			return models.TaskStatusInProgress
		}
	}
	return models.TaskStatusPending
}

func isTerminalStatus(s models.TaskStatus) bool {
	return s == models.TaskStatusCompleted || s == models.TaskStatusFailed || s == models.TaskStatusCancelled
}

// scanTasks is a helper to scan task rows into a slice.
func scanTasks(rows *sql.Rows) ([]models.Task, error) {
	var tasks []models.Task
	for rows.Next() {
		var t models.Task
		var completedAt, heartbeatAt sql.NullInt64
		var errorMsg sql.NullString
		if err := rows.Scan(
			&t.ID, &t.ProjectID, &t.Title, &t.Description, &t.Status, &t.Priority,
			&t.Assignee, &t.Source, &errorMsg, &heartbeatAt,
			&t.CreatedAt, &t.UpdatedAt, &completedAt,
		); err != nil {
			return nil, err
		}
		if errorMsg.Valid {
			t.ErrorMessage = errorMsg.String
		}
		if heartbeatAt.Valid {
			t.HeartbeatAt = heartbeatAt.Int64
		}
		if completedAt.Valid {
			t.CompletedAt = completedAt.Int64
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}
