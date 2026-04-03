package db

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rogerrlee/tasks-watcher/internal/models"
)

// setupTestDB creates an in-memory DB for testing, resolving the project root
// from this source file's location so migrations are found correctly.
func setupTestDB(t *testing.T) *DB {
	origDir, _ := os.Getwd()
	_, thisFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("failed to chdir to project root %s: %v", projectRoot, err)
	}
	defer func() { os.Chdir(origDir) }()

	database, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	return database
}

// makeProject is a test helper that creates a project and returns its ID.
func makeProject(t *testing.T, db *DB, name string) string {
	p := &models.Project{Name: name}
	if err := db.CreateProject(p); err != nil {
		t.Fatalf("CreateProject(%q) failed: %v", name, err)
	}
	return p.ID
}

// makeTask is a test helper that creates a task in the given project.
func makeTask(t *testing.T, db *DB, projectID, title string, status models.TaskStatus) string {
	task := &models.Task{
		ProjectID: projectID,
		Title:     title,
		Status:    status,
		Priority:  models.PriorityMedium,
	}
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask(%q) failed: %v", title, err)
	}
	return task.ID
}

// --- ComputeParentStatus tests ---

func TestComputeParentStatus(t *testing.T) {
	cases := []struct {
		name     string
		children []models.TaskStatus
		want     models.TaskStatus
	}{
		{"empty", []models.TaskStatus{}, models.TaskStatusPending},
		{"single pending", []models.TaskStatus{models.TaskStatusPending}, models.TaskStatusPending},
		{"single in_progress", []models.TaskStatus{models.TaskStatusInProgress}, models.TaskStatusInProgress},
		{"single completed", []models.TaskStatus{models.TaskStatusCompleted}, models.TaskStatusCompleted},
		{"single failed", []models.TaskStatus{models.TaskStatusFailed}, models.TaskStatusCompleted},
		{"single cancelled", []models.TaskStatus{models.TaskStatusCancelled}, models.TaskStatusCompleted},
		{"all completed", []models.TaskStatus{models.TaskStatusCompleted, models.TaskStatusCompleted}, models.TaskStatusCompleted},
		{"all failed", []models.TaskStatus{models.TaskStatusFailed, models.TaskStatusFailed}, models.TaskStatusCompleted},
		{"all cancelled", []models.TaskStatus{models.TaskStatusCancelled, models.TaskStatusCancelled}, models.TaskStatusCompleted},
		{"mixed terminal", []models.TaskStatus{models.TaskStatusCompleted, models.TaskStatusFailed, models.TaskStatusCancelled}, models.TaskStatusCompleted},
		{"in_progress among completed", []models.TaskStatus{models.TaskStatusCompleted, models.TaskStatusInProgress, models.TaskStatusCompleted}, models.TaskStatusInProgress},
		{"pending among completed", []models.TaskStatus{models.TaskStatusCompleted, models.TaskStatusPending, models.TaskStatusCompleted}, models.TaskStatusPending},
		{"failed among pending", []models.TaskStatus{models.TaskStatusPending, models.TaskStatusFailed, models.TaskStatusPending}, models.TaskStatusPending},
		{"failed alone among pending", []models.TaskStatus{models.TaskStatusPending, models.TaskStatusFailed}, models.TaskStatusPending},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeParentStatus(tc.children)
			if got != tc.want {
				t.Errorf("ComputeParentStatus(%v) = %q, want %q", tc.children, got, tc.want)
			}
		})
	}
}

// --- isTerminalStatus tests ---

func TestIsTerminalStatus(t *testing.T) {
	cases := []struct {
		status   models.TaskStatus
		terminal bool
	}{
		{models.TaskStatusPending, false},
		{models.TaskStatusInProgress, false},
		{models.TaskStatusCompleted, true},
		{models.TaskStatusFailed, true},
		{models.TaskStatusCancelled, true},
	}
	for _, tc := range cases {
		t.Run(string(tc.status), func(t *testing.T) {
			got := isTerminalStatus(tc.status)
			if got != tc.terminal {
				t.Errorf("isTerminalStatus(%q) = %v, want %v", tc.status, got, tc.terminal)
			}
		})
	}
}

// --- AddDependency tests ---

func TestAddDependency_Self(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	tid := makeTask(t, db, pid, "task", models.TaskStatusPending)

	_, err := db.AddDependency(tid, tid)
	if err == nil {
		t.Error("expected error for self-dependency, got nil")
	}
}

func TestAddDependency_TaskNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	tid := makeTask(t, db, pid, "task", models.TaskStatusPending)

	_, err := db.AddDependency(tid, "nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent blocker, got nil")
	}
}

func TestAddDependency_BlockerNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	tid := makeTask(t, db, pid, "task", models.TaskStatusPending)

	_, err := db.AddDependency("nonexistent-id", tid)
	if err == nil {
		t.Error("expected error for nonexistent task, got nil")
	}
}

func TestAddDependency_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	tid := makeTask(t, db, pid, "task", models.TaskStatusPending)
	bid := makeTask(t, db, pid, "blocker", models.TaskStatusInProgress)

	dep, err := db.AddDependency(tid, bid)
	if err != nil {
		t.Fatalf("AddDependency failed: %v", err)
	}
	if dep.TaskID != tid {
		t.Errorf("expected TaskID %q, got %q", tid, dep.TaskID)
	}
	if dep.BlockerID != bid {
		t.Errorf("expected BlockerID %q, got %q", bid, dep.BlockerID)
	}
}

func TestAddDependency_Circular(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	t1 := makeTask(t, db, pid, "task1", models.TaskStatusPending)
	t2 := makeTask(t, db, pid, "task2", models.TaskStatusPending)
	t3 := makeTask(t, db, pid, "task3", models.TaskStatusPending)

	// t1 depends on t2
	db.AddDependency(t1, t2)
	// t2 depends on t3
	db.AddDependency(t2, t3)
	// t3 depends on t1 — should be circular
	_, err := db.AddDependency(t3, t1)
	if err == nil {
		t.Error("expected circular dependency error, got nil")
	}
}

func TestAddDependency_LinearChain(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	t1 := makeTask(t, db, pid, "task1", models.TaskStatusPending)
	t2 := makeTask(t, db, pid, "task2", models.TaskStatusPending)
	t3 := makeTask(t, db, pid, "task3", models.TaskStatusPending)

	// t1 depends on t2, t2 depends on t3 — no cycle
	db.AddDependency(t1, t2)
	_, err := db.AddDependency(t2, t3)
	if err != nil {
		t.Errorf("expected no error for linear chain, got: %v", err)
	}
}

// --- GetBlockerIDs / GetDependentIDs tests ---

func TestGetBlockerIDs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	t1 := makeTask(t, db, pid, "task1", models.TaskStatusPending)
	b1 := makeTask(t, db, pid, "blocker1", models.TaskStatusInProgress)
	b2 := makeTask(t, db, pid, "blocker2", models.TaskStatusCompleted)

	db.AddDependency(t1, b1)
	db.AddDependency(t1, b2)

	ids, err := db.GetBlockerIDs(t1)
	if err != nil {
		t.Fatalf("GetBlockerIDs failed: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 blockers, got %d", len(ids))
	}
}

func TestGetDependentIDs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	b := makeTask(t, db, pid, "blocker", models.TaskStatusInProgress)
	t1 := makeTask(t, db, pid, "task1", models.TaskStatusPending)
	t2 := makeTask(t, db, pid, "task2", models.TaskStatusPending)

	db.AddDependency(t1, b)
	db.AddDependency(t2, b)

	ids, err := db.GetDependentIDs(b)
	if err != nil {
		t.Fatalf("GetDependentIDs failed: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 dependents, got %d", len(ids))
	}
}

// --- CanStartTask tests ---

func TestCanStartTask_NoBlockers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	tid := makeTask(t, db, pid, "task", models.TaskStatusPending)

	result, err := db.CanStartTask(tid)
	if err != nil {
		t.Fatalf("CanStartTask failed: %v", err)
	}
	if !result.CanStart {
		t.Error("expected CanStart=true with no blockers")
	}
}

func TestCanStartTask_BlockedByIncomplete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	tid := makeTask(t, db, pid, "task", models.TaskStatusPending)
	bid := makeTask(t, db, pid, "blocker", models.TaskStatusInProgress)

	db.AddDependency(tid, bid)

	result, err := db.CanStartTask(tid)
	if err != nil {
		t.Fatalf("CanStartTask failed: %v", err)
	}
	if result.CanStart {
		t.Error("expected CanStart=false when blocked by in_progress task")
	}
	if len(result.Blockers) != 1 {
		t.Errorf("expected 1 blocker, got %d", len(result.Blockers))
	}
}

func TestCanStartTask_BlockedByPending(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	tid := makeTask(t, db, pid, "task", models.TaskStatusPending)
	bid := makeTask(t, db, pid, "blocker", models.TaskStatusPending)

	db.AddDependency(tid, bid)

	result, err := db.CanStartTask(tid)
	if err != nil {
		t.Fatalf("CanStartTask failed: %v", err)
	}
	if result.CanStart {
		t.Error("expected CanStart=false when blocked by pending task")
	}
}

func TestCanStartTask_NotBlockedByCompleted(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	tid := makeTask(t, db, pid, "task", models.TaskStatusPending)
	bid := makeTask(t, db, pid, "blocker", models.TaskStatusCompleted)

	db.AddDependency(tid, bid)

	result, err := db.CanStartTask(tid)
	if err != nil {
		t.Fatalf("CanStartTask failed: %v", err)
	}
	if !result.CanStart {
		t.Error("expected CanStart=true; completed task should not block")
	}
}

func TestCanStartTask_NotBlockedByFailed(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	tid := makeTask(t, db, pid, "task", models.TaskStatusPending)
	bid := makeTask(t, db, pid, "blocker", models.TaskStatusFailed)

	db.AddDependency(tid, bid)

	result, err := db.CanStartTask(tid)
	if err != nil {
		t.Fatalf("CanStartTask failed: %v", err)
	}
	if !result.CanStart {
		t.Error("expected CanStart=true; failed task should not block")
	}
}

func TestCanStartTask_WithSubtasks(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	parent := makeTask(t, db, pid, "parent", models.TaskStatusPending)
	child := makeTask(t, db, pid, "child", models.TaskStatusInProgress)

	db.AddSubtask(parent, child)

	result, err := db.CanStartTask(parent)
	if err != nil {
		t.Fatalf("CanStartTask failed: %v", err)
	}
	if result.HasChildren && result.CanStart {
		t.Error("expected CanStart=false when parent has non-terminal children")
	}
}

// --- GetChildStatuses tests ---

func TestGetChildStatuses(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	parent := makeTask(t, db, pid, "parent", models.TaskStatusPending)
	c1 := makeTask(t, db, pid, "child1", models.TaskStatusCompleted)
	c2 := makeTask(t, db, pid, "child2", models.TaskStatusInProgress)
	c3 := makeTask(t, db, pid, "child3", models.TaskStatusPending)

	db.AddSubtask(parent, c1)
	db.AddSubtask(parent, c2)
	db.AddSubtask(parent, c3)

	statuses, err := db.GetChildStatuses(parent)
	if err != nil {
		t.Fatalf("GetChildStatuses failed: %v", err)
	}
	if len(statuses) != 3 {
		t.Errorf("expected 3 child statuses, got %d", len(statuses))
	}
}

func TestGetChildStatuses_NoChildren(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	tid := makeTask(t, db, pid, "task", models.TaskStatusPending)

	statuses, err := db.GetChildStatuses(tid)
	if err != nil {
		t.Fatalf("GetChildStatuses failed: %v", err)
	}
	if len(statuses) != 0 {
		t.Errorf("expected 0 child statuses, got %d", len(statuses))
	}
}

// --- RemoveDependency tests ---

func TestRemoveDependency(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	tid := makeTask(t, db, pid, "task", models.TaskStatusPending)
	bid := makeTask(t, db, pid, "blocker", models.TaskStatusInProgress)

	db.AddDependency(tid, bid)

	ids, _ := db.GetBlockerIDs(tid)
	if len(ids) != 1 {
		t.Fatalf("expected 1 blocker before removal, got %d", len(ids))
	}

	err := db.RemoveDependency(tid, bid)
	if err != nil {
		t.Fatalf("RemoveDependency failed: %v", err)
	}

	ids, _ = db.GetBlockerIDs(tid)
	if len(ids) != 0 {
		t.Errorf("expected 0 blockers after removal, got %d", len(ids))
	}
}

// --- Open / Close tests ---

func TestDB_BasicCRUD(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Verify we can do basic operations
	pid := makeProject(t, db, "test-project")
	p, err := db.GetProject(pid)
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}
	if p.Name != "test-project" {
		t.Errorf("expected name 'test-project', got %q", p.Name)
	}
}
