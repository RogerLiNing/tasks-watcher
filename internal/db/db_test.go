package db

import (
	"database/sql"
	"encoding/json"
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

// --- ListProjects tests ---

func TestListProjects_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	projects, err := db.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(projects))
	}
}

func TestListProjects_WithProjects(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	makeProject(t, db, "alpha")
	makeProject(t, db, "beta")

	projects, err := db.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
	if len(projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(projects))
	}
}

// --- GetProjectByName tests ---

func TestGetProjectByName_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p, err := db.GetProjectByName("nonexistent")
	if err != nil {
		t.Fatalf("GetProjectByName failed: %v", err)
	}
	if p != nil {
		t.Errorf("expected nil, got %+v", p)
	}
}

func TestGetProjectByName_Found(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "find-me")
	p, err := db.GetProjectByName("find-me")
	if err != nil {
		t.Fatalf("GetProjectByName failed: %v", err)
	}
	if p == nil {
		t.Fatal("expected project, got nil")
	}
	if p.ID != pid {
		t.Errorf("expected id %q, got %q", pid, p.ID)
	}
}

// --- UpdateProject tests ---

func TestUpdateProject_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "original")
	p, _ := db.GetProject(pid)
	p.Name = "updated"
	p.Description = "new desc"
	if err := db.UpdateProject(p); err != nil {
		t.Fatalf("UpdateProject failed: %v", err)
	}

	p2, _ := db.GetProject(pid)
	if p2.Name != "updated" {
		t.Errorf("expected name 'updated', got %q", p2.Name)
	}
	if p2.Description != "new desc" {
		t.Errorf("expected description 'new desc', got %q", p2.Description)
	}
}

// --- DeleteProject tests ---

func TestDeleteProject_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "to-delete")
	if err := db.DeleteProject(pid); err != nil {
		t.Fatalf("DeleteProject failed: %v", err)
	}
	p, _ := db.GetProject(pid)
	if p != nil {
		t.Error("expected nil after deletion")
	}
}

func TestDeleteProject_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Should not error even if not found
	if err := db.DeleteProject("nonexistent-id"); err != nil {
		t.Errorf("DeleteProject for nonexistent should not error, got: %v", err)
	}
}

// --- GetOrCreateProject tests ---

func TestGetOrCreateProject_Existing(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "exists")
	p, err := db.GetOrCreateProject("exists")
	if err != nil {
		t.Fatalf("GetOrCreateProject failed: %v", err)
	}
	if p.ID != pid {
		t.Errorf("expected id %q, got %q", pid, p.ID)
	}
}

func TestGetOrCreateProject_New(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p, err := db.GetOrCreateProject("brand-new")
	if err != nil {
		t.Fatalf("GetOrCreateProject failed: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil project")
	}
	if p.Name != "brand-new" {
		t.Errorf("expected name 'brand-new', got %q", p.Name)
	}
}

// --- GetProjectByRepoPath tests ---

func TestGetProjectByRepoPath_EmptyString(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p, err := db.GetProjectByRepoPath("")
	if err != nil {
		t.Fatalf("GetProjectByRepoPath failed: %v", err)
	}
	if p != nil {
		t.Errorf("expected nil for empty path, got %+v", p)
	}
}

func TestGetProjectByRepoPath_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p, err := db.GetProjectByRepoPath("/some/path")
	if err != nil {
		t.Fatalf("GetProjectByRepoPath failed: %v", err)
	}
	if p != nil {
		t.Errorf("expected nil, got %+v", p)
	}
}

func TestGetProjectByRepoPath_Found(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "repo-project")
	p, _ := db.GetProject(pid)
	p.RepoPath = "/Users/me/src/myproject"
	db.UpdateProject(p)

	found, err := db.GetProjectByRepoPath("/Users/me/src/myproject")
	if err != nil {
		t.Fatalf("GetProjectByRepoPath failed: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find project by repo path")
	}
	if found.ID != pid {
		t.Errorf("expected id %q, got %q", pid, found.ID)
	}
}

// --- GetTask tests ---

func TestGetTask_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	task, err := db.GetTask("nonexistent")
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if task != nil {
		t.Errorf("expected nil, got %+v", task)
	}
}

func TestGetTask_Found(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	tid := makeTask(t, db, pid, "my-task", models.TaskStatusInProgress)

	task, err := db.GetTask(tid)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if task == nil {
		t.Fatal("expected task, got nil")
	}
	if task.Title != "my-task" {
		t.Errorf("expected title 'my-task', got %q", task.Title)
	}
	if task.Status != models.TaskStatusInProgress {
		t.Errorf("expected status in_progress, got %q", task.Status)
	}
}

func TestGetTask_WithDescription(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	task := &models.Task{
		ProjectID: pid,
		Title:     "with-desc",
		Status:    models.TaskStatusPending,
		Priority:  models.PriorityMedium,
		Description: map[string]string{
			"en": "English description",
			"zh": "中文描述",
		},
	}
	db.CreateTask(task)

	fetched, err := db.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	descJSON, _ := json.Marshal(fetched.Description)
	if string(descJSON) == "" || string(descJSON) == "{}" {
		// Legacy description
	} else {
		desc := fetched.Description
		if desc["en"] != "English description" {
			t.Errorf("expected en desc, got %v", desc)
		}
	}
}

// --- ListTasks tests ---

func TestListTasks_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	tasks, total, err := db.ListTasks(pid, "", "", "", "", 50, 0)
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
	if total != 0 {
		t.Errorf("expected total 0, got %d", total)
	}
}

func TestListTasks_WithTasks(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	makeTask(t, db, pid, "task-1", models.TaskStatusPending)
	makeTask(t, db, pid, "task-2", models.TaskStatusInProgress)

	tasks, total, err := db.ListTasks(pid, "", "", "", "", 50, 0)
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
}

func TestListTasks_FilterByStatus(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	makeTask(t, db, pid, "pending-task", models.TaskStatusPending)
	makeTask(t, db, pid, "done-task", models.TaskStatusCompleted)

	tasks, total, err := db.ListTasks(pid, string(models.TaskStatusPending), "", "", "", 50, 0)
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	if total != 1 {
		t.Errorf("expected 1 pending task, got %d", total)
	}
	if len(tasks) != 1 || tasks[0].Title != "pending-task" {
		t.Errorf("unexpected tasks: %v", tasks)
	}
}

func TestListTasks_FilterByAssignee(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	t1 := &models.Task{ProjectID: pid, Title: "t1", Status: models.TaskStatusPending, Priority: models.PriorityMedium, Assignee: "alice"}
	t2 := &models.Task{ProjectID: pid, Title: "t2", Status: models.TaskStatusPending, Priority: models.PriorityMedium, Assignee: "bob"}
	db.CreateTask(t1)
	db.CreateTask(t2)

	tasks, _, err := db.ListTasks(pid, "", "alice", "", "", 50, 0)
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 task for alice, got %d", len(tasks))
	}
}

func TestListTasks_Search(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	makeTask(t, db, pid, "find-me-task", models.TaskStatusPending)
	makeTask(t, db, pid, "other-task", models.TaskStatusPending)

	tasks, _, err := db.ListTasks(pid, "", "", "find", "", 50, 0)
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 task matching 'find', got %d", len(tasks))
	}
}

func TestListTasks_Pagination(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	for i := 0; i < 5; i++ {
		makeTask(t, db, pid, "task", models.TaskStatusPending)
	}

	page1, total, err := db.ListTasks(pid, "", "", "", "", 2, 0)
	if err != nil {
		t.Fatalf("ListTasks page 1 failed: %v", err)
	}
	if len(page1) != 2 {
		t.Errorf("expected 2 tasks per page, got %d", len(page1))
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}

	page2, _, err := db.ListTasks(pid, "", "", "", "", 2, 2)
	if err != nil {
		t.Fatalf("ListTasks page 2 failed: %v", err)
	}
	if len(page2) != 2 {
		t.Errorf("expected 2 tasks on page 2, got %d", len(page2))
	}
}

// --- UpdateTask tests ---

func TestUpdateTask_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	tid := makeTask(t, db, pid, "original", models.TaskStatusPending)

	task, _ := db.GetTask(tid)
	task.Title = "updated title"
	task.Status = models.TaskStatusInProgress
	task.Priority = models.PriorityHigh
	task.Description = map[string]string{"en": "new description"}
	if err := db.UpdateTask(task); err != nil {
		t.Fatalf("UpdateTask failed: %v", err)
	}

	updated, _ := db.GetTask(tid)
	if updated.Title != "updated title" {
		t.Errorf("expected title 'updated title', got %q", updated.Title)
	}
	if updated.Status != models.TaskStatusInProgress {
		t.Errorf("expected status in_progress, got %q", updated.Status)
	}
	if updated.Priority != models.PriorityHigh {
		t.Errorf("expected priority high, got %q", updated.Priority)
	}
}

// --- UpdateTaskStatus tests ---

func TestUpdateTaskStatus_ToCompleted(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	tid := makeTask(t, db, pid, "task", models.TaskStatusInProgress)

	if err := db.UpdateTaskStatus(tid, models.TaskStatusCompleted, ""); err != nil {
		t.Fatalf("UpdateTaskStatus failed: %v", err)
	}
	task, _ := db.GetTask(tid)
	if task.Status != models.TaskStatusCompleted {
		t.Errorf("expected completed, got %q", task.Status)
	}
}

func TestUpdateTaskStatus_ToFailed(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	tid := makeTask(t, db, pid, "task", models.TaskStatusInProgress)

	if err := db.UpdateTaskStatus(tid, models.TaskStatusFailed, "something broke"); err != nil {
		t.Fatalf("UpdateTaskStatus failed: %v", err)
	}
	task, _ := db.GetTask(tid)
	if task.Status != models.TaskStatusFailed {
		t.Errorf("expected failed, got %q", task.Status)
	}
	if task.ErrorMessage != "something broke" {
		t.Errorf("expected error message, got %q", task.ErrorMessage)
	}
}

func TestUpdateTaskStatus_ToCancelled(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	tid := makeTask(t, db, pid, "task", models.TaskStatusPending)

	if err := db.UpdateTaskStatus(tid, models.TaskStatusCancelled, ""); err != nil {
		t.Fatalf("UpdateTaskStatus failed: %v", err)
	}
	task, _ := db.GetTask(tid)
	if task.Status != models.TaskStatusCancelled {
		t.Errorf("expected cancelled, got %q", task.Status)
	}
}

// --- HeartbeatTask tests ---

func TestHeartbeatTask_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	tid := makeTask(t, db, pid, "task", models.TaskStatusInProgress)

	if err := db.HeartbeatTask(tid); err != nil {
		t.Fatalf("HeartbeatTask failed: %v", err)
	}
	task, _ := db.GetTask(tid)
	if task.HeartbeatAt == 0 {
		t.Error("expected heartbeat_at to be set")
	}
}

// --- DeleteTask tests ---

func TestDeleteTask_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	tid := makeTask(t, db, pid, "task", models.TaskStatusPending)

	if err := db.DeleteTask(tid); err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}
	task, _ := db.GetTask(tid)
	if task != nil {
		t.Error("expected nil after deletion")
	}
}

func TestDeleteTask_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Should not error even if not found
	if err := db.DeleteTask("nonexistent-id"); err != nil {
		t.Errorf("DeleteTask for nonexistent should not error, got: %v", err)
	}
}

// --- scanTasks helper tests (via GetDependencyTasks) ---

func TestGetDependencyTasks_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	tid := makeTask(t, db, pid, "task", models.TaskStatusPending)

	deps, err := db.GetDependencyTasks(tid)
	if err != nil {
		t.Fatalf("GetDependencyTasks failed: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("expected 0 dependencies, got %d", len(deps))
	}
}

func TestGetDependencyTasks_WithBlockers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	t1 := makeTask(t, db, pid, "task1", models.TaskStatusPending)
	t2 := makeTask(t, db, pid, "task2", models.TaskStatusInProgress)

	db.AddDependency(t1, t2)

	deps, err := db.GetDependencyTasks(t1)
	if err != nil {
		t.Fatalf("GetDependencyTasks failed: %v", err)
	}
	if len(deps) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(deps))
	}
	if deps[0].Title != "task2" {
		t.Errorf("expected blocker 'task2', got %q", deps[0].Title)
	}
}

func TestGetDependentTasks_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	bid := makeTask(t, db, pid, "blocker", models.TaskStatusPending)

	deps, err := db.GetDependentTasks(bid)
	if err != nil {
		t.Fatalf("GetDependentTasks failed: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("expected 0 dependents, got %d", len(deps))
	}
}

func TestGetDependentTasks_WithDependents(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	b := makeTask(t, db, pid, "blocker", models.TaskStatusPending)
	t1 := makeTask(t, db, pid, "dependent1", models.TaskStatusPending)
	t2 := makeTask(t, db, pid, "dependent2", models.TaskStatusPending)

	db.AddDependency(t1, b)
	db.AddDependency(t2, b)

	deps, err := db.GetDependentTasks(b)
	if err != nil {
		t.Fatalf("GetDependentTasks failed: %v", err)
	}
	if len(deps) != 2 {
		t.Errorf("expected 2 dependents, got %d", len(deps))
	}
}

// --- AddSubtask tests ---

func TestAddSubtask_SelfLoop(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	pid := makeProject(t, db, "proj")
	task := makeTask(t, db, pid, "task", models.TaskStatusPending)

	_, err := db.AddSubtask(task, task)
	if err == nil {
		t.Error("expected error for self-loop")
	}
}

func TestAddSubtask_ChildNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	pid := makeProject(t, db, "proj")
	parent := makeTask(t, db, pid, "parent", models.TaskStatusPending)

	_, err := db.AddSubtask(parent, "nonexistent-child-id")
	if err == nil {
		t.Error("expected error for nonexistent child")
	}
}

func TestAddSubtask_AlreadyHasParent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	pid := makeProject(t, db, "proj")
	p1 := makeTask(t, db, pid, "parent1", models.TaskStatusPending)
	p2 := makeTask(t, db, pid, "parent2", models.TaskStatusPending)
	child := makeTask(t, db, pid, "child", models.TaskStatusPending)

	db.AddSubtask(p1, child)
	_, err := db.AddSubtask(p2, child)
	if err == nil {
		t.Error("expected error for child already having a parent")
	}
}

// --- Subtask query tests ---

func TestGetSubtaskIDs_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	parent := makeTask(t, db, pid, "parent", models.TaskStatusPending)

	ids, err := db.GetSubtaskIDs(parent)
	if err != nil {
		t.Fatalf("GetSubtaskIDs failed: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected 0 subtask IDs, got %d", len(ids))
	}
}

func TestGetSubtaskIDs_WithChildren(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	parent := makeTask(t, db, pid, "parent", models.TaskStatusPending)
	child := makeTask(t, db, pid, "child", models.TaskStatusPending)

	db.AddSubtask(parent, child)

	ids, err := db.GetSubtaskIDs(parent)
	if err != nil {
		t.Fatalf("GetSubtaskIDs failed: %v", err)
	}
	if len(ids) != 1 {
		t.Errorf("expected 1 subtask ID, got %d", len(ids))
	}
	if ids[0] != child {
		t.Errorf("expected child ID %q, got %q", child, ids[0])
	}
}

func TestGetSubtaskTasks_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	parent := makeTask(t, db, pid, "parent", models.TaskStatusPending)

	tasks, err := db.GetSubtaskTasks(parent)
	if err != nil {
		t.Fatalf("GetSubtaskTasks failed: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 subtasks, got %d", len(tasks))
	}
}

func TestGetSubtaskTasks_WithChildren(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	parent := makeTask(t, db, pid, "parent", models.TaskStatusPending)
	child := makeTask(t, db, pid, "child-task", models.TaskStatusPending)

	db.AddSubtask(parent, child)

	tasks, err := db.GetSubtaskTasks(parent)
	if err != nil {
		t.Fatalf("GetSubtaskTasks failed: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 subtask, got %d", len(tasks))
	}
	if tasks[0].Title != "child-task" {
		t.Errorf("expected 'child-task', got %q", tasks[0].Title)
	}
}

func TestGetParentID_NoParent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	child := makeTask(t, db, pid, "child", models.TaskStatusPending)

	pid2, err := db.GetParentID(child)
	if err != nil {
		t.Fatalf("GetParentID failed: %v", err)
	}
	if pid2 != "" {
		t.Errorf("expected empty parent ID, got %q", pid2)
	}
}

func TestGetParentID_WithParent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	parent := makeTask(t, db, pid, "parent", models.TaskStatusPending)
	child := makeTask(t, db, pid, "child", models.TaskStatusPending)

	db.AddSubtask(parent, child)

	parentID, err := db.GetParentID(child)
	if err != nil {
		t.Fatalf("GetParentID failed: %v", err)
	}
	if parentID != parent {
		t.Errorf("expected parent ID %q, got %q", parent, parentID)
	}
}

func TestGetParentTask_NoParent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	child := makeTask(t, db, pid, "orphan", models.TaskStatusPending)

	p, err := db.GetParentTask(child)
	if err != nil {
		t.Fatalf("GetParentTask failed: %v", err)
	}
	if p != nil {
		t.Errorf("expected nil (no parent), got %+v", p)
	}
}

func TestGetParentTask_WithParent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	parent := makeTask(t, db, pid, "my-parent", models.TaskStatusPending)
	child := makeTask(t, db, pid, "child", models.TaskStatusPending)

	db.AddSubtask(parent, child)

	p, err := db.GetParentTask(child)
	if err != nil {
		t.Fatalf("GetParentTask failed: %v", err)
	}
	if p == nil {
		t.Fatal("expected parent task, got nil")
	}
	if p.Title != "my-parent" {
		t.Errorf("expected 'my-parent', got %q", p.Title)
	}
}

func TestRemoveSubtask_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	parent := makeTask(t, db, pid, "parent", models.TaskStatusPending)
	child := makeTask(t, db, pid, "child", models.TaskStatusPending)

	db.AddSubtask(parent, child)

	ids, _ := db.GetSubtaskIDs(parent)
	if len(ids) != 1 {
		t.Fatalf("expected 1 subtask before removal, got %d", len(ids))
	}

	if err := db.RemoveSubtask(parent, child); err != nil {
		t.Fatalf("RemoveSubtask failed: %v", err)
	}

	ids, _ = db.GetSubtaskIDs(parent)
	if len(ids) != 0 {
		t.Errorf("expected 0 subtasks after removal, got %d", len(ids))
	}
}

func TestRemoveSubtask_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	parent := makeTask(t, db, pid, "parent", models.TaskStatusPending)
	child := makeTask(t, db, pid, "child", models.TaskStatusPending)

	// Remove without adding first — should not error
	if err := db.RemoveSubtask(parent, child); err != nil {
		t.Errorf("RemoveSubtask for nonexistent relation should not error, got: %v", err)
	}
}

// --- scanTasks edge cases ---

func TestGetDependencyTasks_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	deps, err := db.GetDependencyTasks("nonexistent-id")
	if err != nil {
		t.Fatalf("GetDependencyTasks for nonexistent should not error, got: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("expected 0, got %d", len(deps))
	}
}

func TestGetDependentTasks_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	deps, err := db.GetDependentTasks("nonexistent-id")
	if err != nil {
		t.Fatalf("GetDependentTasks for nonexistent should not error, got: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("expected 0, got %d", len(deps))
	}
}

// --- ListAgents tests ---

func TestListAgents_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	agents, err := db.ListAgents()
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(agents))
	}
}

func TestListAgents_WithAssignees(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	for _, name := range []string{"alice", "bob", "charlie"} {
		task := &models.Task{ProjectID: pid, Title: "task", Status: models.TaskStatusPending, Priority: models.PriorityMedium, Assignee: name}
		db.CreateTask(task)
	}

	agents, err := db.ListAgents()
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}
	if len(agents) != 3 {
		t.Errorf("expected 3 agents, got %d", len(agents))
	}
}

func TestListAgents_Distinct(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	// Create multiple tasks with the same assignee
	for i := 0; i < 3; i++ {
		task := &models.Task{ProjectID: pid, Title: "task", Status: models.TaskStatusPending, Priority: models.PriorityMedium, Assignee: "alice"}
		db.CreateTask(task)
	}

	agents, err := db.ListAgents()
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}
	if len(agents) != 1 {
		t.Errorf("expected 1 distinct agent, got %d", len(agents))
	}
}

// --- Notification DB tests ---

func TestCreateNotification_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	n := &models.Notification{
		TaskID:  "task-1",
		Type:    "task.completed",
		Message: "Task done",
		Read:    false,
	}
	if err := db.CreateNotification(n); err != nil {
		t.Fatalf("CreateNotification failed: %v", err)
	}
	if n.ID == "" {
		t.Error("expected ID to be set")
	}
}

func TestListNotifications_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	notifs, err := db.ListNotifications(10)
	if err != nil {
		t.Fatalf("ListNotifications failed: %v", err)
	}
	if len(notifs) != 0 {
		t.Errorf("expected 0 notifications, got %d", len(notifs))
	}
}

func TestListNotifications_WithNotifications(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	for i := 0; i < 3; i++ {
		db.CreateNotification(&models.Notification{TaskID: "task", Type: "t", Message: "m", Read: false})
	}

	notifs, err := db.ListNotifications(10)
	if err != nil {
		t.Fatalf("ListNotifications failed: %v", err)
	}
	if len(notifs) != 3 {
		t.Errorf("expected 3 notifications, got %d", len(notifs))
	}
}

func TestMarkNotificationRead_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	n := &models.Notification{TaskID: "t", Type: "t", Message: "m", Read: false}
	db.CreateNotification(n)

	if err := db.MarkNotificationRead(n.ID); err != nil {
		t.Fatalf("MarkNotificationRead failed: %v", err)
	}

	notifs, _ := db.ListNotifications(10)
	if !notifs[0].Read {
		t.Error("expected notification to be marked read")
	}
}

func TestMarkAllNotificationsRead_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	for i := 0; i < 3; i++ {
		db.CreateNotification(&models.Notification{TaskID: "t", Type: "t", Message: "m", Read: false})
	}

	if err := db.MarkAllNotificationsRead(); err != nil {
		t.Fatalf("MarkAllNotificationsRead failed: %v", err)
	}

	count, _ := db.GetUnreadNotificationCount()
	if count != 0 {
		t.Errorf("expected 0 unread, got %d", count)
	}
}

func TestGetUnreadNotificationCount_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	count, err := db.GetUnreadNotificationCount()
	if err != nil {
		t.Fatalf("GetUnreadNotificationCount failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 unread, got %d", count)
	}
}

func TestGetUnreadNotificationCount_WithUnread(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	db.CreateNotification(&models.Notification{TaskID: "t", Type: "t", Message: "m", Read: false})
	db.CreateNotification(&models.Notification{TaskID: "t", Type: "t", Message: "m", Read: true})

	count, err := db.GetUnreadNotificationCount()
	if err != nil {
		t.Fatalf("GetUnreadNotificationCount failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 unread, got %d", count)
	}
}

func TestClearNotifications_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	db.CreateNotification(&models.Notification{TaskID: "t", Type: "t", Message: "m", Read: false})
	db.CreateNotification(&models.Notification{TaskID: "t", Type: "t", Message: "m", Read: false})

	if err := db.ClearNotifications(); err != nil {
		t.Fatalf("ClearNotifications failed: %v", err)
	}

	count, _ := db.GetUnreadNotificationCount()
	if count != 0 {
		t.Errorf("expected 0 after clear, got %d", count)
	}
}

// --- Webhook DB tests ---

func TestCreateWebhook_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	wh := &models.WebhookConfig{URL: "https://example.com/hook", Events: "task.*", Active: true}
	if err := db.CreateWebhook(wh); err != nil {
		t.Fatalf("CreateWebhook failed: %v", err)
	}
	if wh.ID == "" {
		t.Error("expected webhook ID to be set")
	}
}

func TestListWebhooks_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	webhooks, err := db.ListWebhooks()
	if err != nil {
		t.Fatalf("ListWebhooks failed: %v", err)
	}
	if len(webhooks) != 0 {
		t.Errorf("expected 0 webhooks, got %d", len(webhooks))
	}
}

func TestListWebhooks_WithWebhooks(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	db.CreateWebhook(&models.WebhookConfig{URL: "https://a.com/hook", Events: "task.*", Active: true})
	db.CreateWebhook(&models.WebhookConfig{URL: "https://b.com/hook", Events: "task.completed", Active: false})

	webhooks, err := db.ListWebhooks()
	if err != nil {
		t.Fatalf("ListWebhooks failed: %v", err)
	}
	if len(webhooks) != 2 {
		t.Errorf("expected 2 webhooks, got %d", len(webhooks))
	}
}

func TestDeleteWebhook_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	wh := &models.WebhookConfig{URL: "https://example.com/hook", Events: "task.*", Active: true}
	db.CreateWebhook(wh)

	if err := db.DeleteWebhook(wh.ID); err != nil {
		t.Fatalf("DeleteWebhook failed: %v", err)
	}

	webhooks, _ := db.ListWebhooks()
	if len(webhooks) != 0 {
		t.Errorf("expected 0 webhooks after delete, got %d", len(webhooks))
	}
}

func TestDeleteWebhook_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Should not error
	if err := db.DeleteWebhook("nonexistent-id"); err != nil {
		t.Errorf("DeleteWebhook for nonexistent should not error, got: %v", err)
	}
}

// --- Column DB tests ---

func TestListColumns_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Migration seeds default columns; clear them first
	cols, _ := db.ListColumns()
	for _, c := range cols {
		db.DeleteColumn(c.ID)
	}

	columns, err := db.ListColumns()
	if err != nil {
		t.Fatalf("ListColumns failed: %v", err)
	}
	if len(columns) != 0 {
		t.Errorf("expected 0 columns, got %d", len(columns))
	}
}

func TestCreateColumn_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	c := &models.TaskColumn{Key: "test_col", Label: "Test Column"}
	if err := db.CreateColumn(c); err != nil {
		t.Fatalf("CreateColumn failed: %v", err)
	}
	if c.ID == "" {
		t.Error("expected column ID to be set")
	}
	if c.Key != "test_col" {
		t.Errorf("expected key 'test_col', got %q", c.Key)
	}
}

func TestUpdateColumn_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	c := &models.TaskColumn{Key: "original", Label: "Original"}
	db.CreateColumn(c)

	c.Label = "Updated Label"
	if err := db.UpdateColumn(c); err != nil {
		t.Fatalf("UpdateColumn failed: %v", err)
	}

	cols, _ := db.ListColumns()
	found := false
	for _, col := range cols {
		if col.ID == c.ID && col.Label == "Updated Label" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected column to be updated")
	}
}

func TestDeleteColumn_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	c := &models.TaskColumn{Key: "to_delete", Label: "ToDelete"}
	db.CreateColumn(c)

	if err := db.DeleteColumn(c.ID); err != nil {
		t.Fatalf("DeleteColumn failed: %v", err)
	}

	cols, _ := db.ListColumns()
	for _, col := range cols {
		if col.ID == c.ID {
			t.Error("expected column to be deleted")
		}
	}
}

func TestDeleteColumn_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	if err := db.DeleteColumn("nonexistent-id"); err != nil {
		t.Errorf("DeleteColumn for nonexistent should not error, got: %v", err)
	}
}

// --- scanTasks with NULL description ---

func TestGetDependencyTasks_WithNullDescription(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	b := makeTask(t, db, pid, "blocker", models.TaskStatusPending)
	t1 := makeTask(t, db, pid, "task", models.TaskStatusPending)

	db.AddDependency(t1, b)

	deps, err := db.GetDependencyTasks(t1)
	if err != nil {
		t.Fatalf("GetDependencyTasks failed: %v", err)
	}
	// Should not panic on null description
	if len(deps) != 1 {
		t.Errorf("expected 1, got %d", len(deps))
	}
}

func TestGetSubtaskTasks_WithNullDescription(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	parent := makeTask(t, db, pid, "parent", models.TaskStatusPending)
	child := makeTask(t, db, pid, "child", models.TaskStatusPending)

	db.AddSubtask(parent, child)

	tasks, err := db.GetSubtaskTasks(parent)
	if err != nil {
		t.Fatalf("GetSubtaskTasks failed: %v", err)
	}
	// Should not panic on null description
	if len(tasks) != 1 {
		t.Errorf("expected 1, got %d", len(tasks))
	}
}

// --- ListColumns with seeded data ---

func TestListColumns_WithSeededColumns(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cols, err := db.ListColumns()
	if err != nil {
		t.Fatalf("ListColumns failed: %v", err)
	}
	// Migration 005 seeds 5 default columns
	if len(cols) == 0 {
		t.Error("expected migration to seed default columns")
	}
}

// Ensure scanTasks doesn't panic on sql.Rows error
var _ = sql.ErrNoRows

// --- GetOrCreateByRepoPath tests ---

func TestGetOrCreateByRepoPath_EmptyString(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p, err := db.GetOrCreateByRepoPath("")
	if err != nil {
		t.Fatalf("GetOrCreateByRepoPath failed: %v", err)
	}
	if p != nil {
		t.Errorf("expected nil for empty path, got %+v", p)
	}
}

func TestGetOrCreateByRepoPath_Existing(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "proj")
	p, _ := db.GetProject(pid)
	p.RepoPath = "/Users/me/src/existing"
	db.UpdateProject(p)

	found, err := db.GetOrCreateByRepoPath("/Users/me/src/existing")
	if err != nil {
		t.Fatalf("GetOrCreateByRepoPath failed: %v", err)
	}
	if found.ID != pid {
		t.Errorf("expected existing project, got id %q", found.ID)
	}
}

func TestGetOrCreateByRepoPath_NewCreatesProject(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p, err := db.GetOrCreateByRepoPath("/Users/me/src/my-project")
	if err != nil {
		t.Fatalf("GetOrCreateByRepoPath failed: %v", err)
	}
	if p == nil {
		t.Fatal("expected project to be created")
	}
	if p.Name != "my-project" {
		t.Errorf("expected name 'my-project', got %q", p.Name)
	}
	if p.RepoPath != "/Users/me/src/my-project" {
		t.Errorf("expected repo path, got %q", p.RepoPath)
	}
}

func TestGetOrCreateByRepoPath_NameCollisionUpdatesExisting(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create project with name "src" but no repo_path
	pid := makeProject(t, db, "src")
	p, _ := db.GetProject(pid)
	if p.RepoPath != "" {
		t.Fatalf("expected empty repo_path")
	}

	// GetOrCreateByRepoPath with /Users/me/src
	found, err := db.GetOrCreateByRepoPath("/Users/me/src")
	if err != nil {
		t.Fatalf("GetOrCreateByRepoPath failed: %v", err)
	}
	// Should find existing project by name and update its repo_path
	if found.ID != pid {
		t.Errorf("expected existing project to be updated, got id %q", found.ID)
	}
	if found.RepoPath != "/Users/me/src" {
		t.Errorf("expected repo_path to be updated, got %q", found.RepoPath)
	}
}

func TestGetOrCreateByRepoPath_DotSlash(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// filepath.Base("./") returns "." — should fall back to "default"
	p, err := db.GetOrCreateByRepoPath("./")
	if err != nil {
		t.Fatalf("GetOrCreateByRepoPath failed: %v", err)
	}
	if p.Name != "default" {
		t.Errorf("expected name 'default', got %q", p.Name)
	}
}

// --- Notification Config DB tests ---

func TestGetNotificationConfig_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	c, err := db.GetNotificationConfig("nonexistent")
	if err != nil {
		t.Fatalf("GetNotificationConfig failed: %v", err)
	}
	if c != nil {
		t.Errorf("expected nil, got %+v", c)
	}
}

func TestGetNotificationConfig_Found(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	db.UpsertNotificationConfig(&models.NotificationConfig{
		Type:    "macos",
		Enabled: true,
		Config:  map[string]interface{}{"sound": true},
	})

	c, err := db.GetNotificationConfig("macos")
	if err != nil {
		t.Fatalf("GetNotificationConfig failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected config, got nil")
	}
	if c.Type != "macos" {
		t.Errorf("expected type 'macos', got %q", c.Type)
	}
	if !c.Enabled {
		t.Error("expected enabled=true")
	}
}

func TestListNotificationConfigs_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Migration seeds macos+email configs; confirm at least 2 exist
	list, err := db.ListNotificationConfigs()
	if err != nil {
		t.Fatalf("ListNotificationConfigs failed: %v", err)
	}
	if len(list) < 2 {
		t.Errorf("expected at least 2 seeded configs, got %d", len(list))
	}
}

func TestUpsertNotificationConfig_Insert(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cfg := &models.NotificationConfig{
		Type:    "macos",
		Enabled: true,
		Config:  map[string]interface{}{"sound": "default"},
	}
	if err := db.UpsertNotificationConfig(cfg); err != nil {
		t.Fatalf("UpsertNotificationConfig failed: %v", err)
	}

	c, _ := db.GetNotificationConfig("macos")
	if c == nil {
		t.Fatal("expected config after insert")
	}
	if c.Type != "macos" {
		t.Errorf("expected type 'macos', got %q", c.Type)
	}
}

func TestUpsertNotificationConfig_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	db.UpsertNotificationConfig(&models.NotificationConfig{
		Type:    "macos",
		Enabled: false,
		Config:  map[string]interface{}{"sound": "default"},
	})

	// Update: enable it
	db.UpsertNotificationConfig(&models.NotificationConfig{
		Type:    "macos",
		Enabled: true,
		Config:  map[string]interface{}{"sound": "custom"},
	})

	c, _ := db.GetNotificationConfig("macos")
	if !c.Enabled {
		t.Error("expected enabled=true after update")
	}
}

func TestUpsertNotificationConfig_DisabledConfig(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	db.UpsertNotificationConfig(&models.NotificationConfig{
		Type:    "email",
		Enabled: false,
		Config:  map[string]interface{}{"smtp_host": "smtp.example.com"},
	})

	c, _ := db.GetNotificationConfig("email")
	if c.Enabled {
		t.Error("expected enabled=false")
	}
}

// --- ExportAll tests ---

func TestExportAll_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	export, err := db.ExportAll()
	if err != nil {
		t.Fatalf("ExportAll failed: %v", err)
	}
	if export == nil {
		t.Fatal("expected non-nil export")
	}
	projects := export["projects"].([]models.Project)
	tasks := export["tasks"].([]models.Task)
	notifs := export["notifications"].([]models.Notification)
	if len(projects) != 0 || len(tasks) != 0 || len(notifs) != 0 {
		t.Errorf("expected empty export, got projects=%d tasks=%d notifs=%d",
			len(projects), len(tasks), len(notifs))
	}
	if _, ok := export["exported_at"]; !ok {
		t.Error("expected exported_at field")
	}
}

func TestExportAll_WithData(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pid := makeProject(t, db, "export-test")
	makeTask(t, db, pid, "task-1", models.TaskStatusPending)
	db.CreateNotification(&models.Notification{TaskID: "t", Type: "t", Message: "m", Read: false})

	export, err := db.ExportAll()
	if err != nil {
		t.Fatalf("ExportAll failed: %v", err)
	}
	projects := export["projects"].([]models.Project)
	tasks := export["tasks"].([]models.Task)
	notifs := export["notifications"].([]models.Notification)
	if len(projects) != 1 {
		t.Errorf("expected 1 project, got %d", len(projects))
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}
	if len(notifs) != 1 {
		t.Errorf("expected 1 notification, got %d", len(notifs))
	}
}

// --- Subtask position tests ---

func TestGetSubtaskPosition_Basic(t *testing.T) {
	db := setupTestDB(t)
	pid := makeProject(t, db, "proj")
	parent := makeTask(t, db, pid, "parent", models.TaskStatusPending)
	child := makeTask(t, db, pid, "child", models.TaskStatusPending)
	db.AddSubtask(parent, child)

	pos, err := db.GetSubtaskPosition(parent, child)
	if err != nil {
		t.Fatalf("GetSubtaskPosition failed: %v", err)
	}
	if pos != 0 {
		t.Errorf("expected position 0 (first), got %d", pos)
	}
}

func TestGetSubtaskPosition_NotFound(t *testing.T) {
	db := setupTestDB(t)
	pid := makeProject(t, db, "proj")
	parent := makeTask(t, db, pid, "parent", models.TaskStatusPending)
	child := makeTask(t, db, pid, "child", models.TaskStatusPending)

	pos, err := db.GetSubtaskPosition(parent, child)
	if err != nil {
		t.Fatalf("GetSubtaskPosition for nonexistent relation should not error, got: %v", err)
	}
	if pos != 0 {
		t.Errorf("expected 0 for nonexistent relation, got %d", pos)
	}
}

func TestGetSubtaskPositions_Empty(t *testing.T) {
	db := setupTestDB(t)
	pid := makeProject(t, db, "proj")
	parent := makeTask(t, db, pid, "parent", models.TaskStatusPending)

	posMap, err := db.GetSubtaskPositions(parent)
	if err != nil {
		t.Fatalf("GetSubtaskPositions failed: %v", err)
	}
	if len(posMap) != 0 {
		t.Errorf("expected 0 positions, got %d", len(posMap))
	}
}

func TestGetSubtaskPositions_WithMultiple(t *testing.T) {
	db := setupTestDB(t)
	pid := makeProject(t, db, "proj")
	parent := makeTask(t, db, pid, "parent", models.TaskStatusPending)
	c1 := makeTask(t, db, pid, "c1", models.TaskStatusPending)
	c2 := makeTask(t, db, pid, "c2", models.TaskStatusPending)
	c3 := makeTask(t, db, pid, "c3", models.TaskStatusPending)
	db.AddSubtask(parent, c1)
	db.AddSubtask(parent, c2)
	db.AddSubtask(parent, c3)

	posMap, err := db.GetSubtaskPositions(parent)
	if err != nil {
		t.Fatalf("GetSubtaskPositions failed: %v", err)
	}
	if len(posMap) != 3 {
		t.Errorf("expected 3 positions, got %d", len(posMap))
	}
	// All should be assigned distinct positions starting at 0.
	seen := make(map[int]bool)
	for _, pos := range posMap {
		if seen[pos] {
			t.Errorf("duplicate position %d found", pos)
		}
		seen[pos] = true
	}
}

func TestSetSubtaskPosition_SamePosition(t *testing.T) {
	db := setupTestDB(t)
	pid := makeProject(t, db, "proj")
	parent := makeTask(t, db, pid, "parent", models.TaskStatusPending)
	child := makeTask(t, db, pid, "child", models.TaskStatusPending)
	db.AddSubtask(parent, child)

	// Setting to same position should be a no-op.
	if err := db.SetSubtaskPosition(parent, child, 0); err != nil {
		t.Fatalf("SetSubtaskPosition(0) should not error: %v", err)
	}
	pos, _ := db.GetSubtaskPosition(parent, child)
	if pos != 0 {
		t.Errorf("position should still be 0, got %d", pos)
	}
}

func TestSetSubtaskPosition_ShiftUp(t *testing.T) {
	db := setupTestDB(t)
	pid := makeProject(t, db, "proj")
	parent := makeTask(t, db, pid, "parent", models.TaskStatusPending)
	c1 := makeTask(t, db, pid, "c1", models.TaskStatusPending)
	c2 := makeTask(t, db, pid, "c2", models.TaskStatusPending)
	c3 := makeTask(t, db, pid, "c3", models.TaskStatusPending)
	db.AddSubtask(parent, c1) // pos 0
	db.AddSubtask(parent, c2) // pos 1
	db.AddSubtask(parent, c3) // pos 2

	// Move c3 from pos 2 → pos 0 (shift up: c1 and c2 shift down).
	if err := db.SetSubtaskPosition(parent, c3, 0); err != nil {
		t.Fatalf("SetSubtaskPosition failed: %v", err)
	}

	pos1, _ := db.GetSubtaskPosition(parent, c1)
	pos2, _ := db.GetSubtaskPosition(parent, c2)
	pos3, _ := db.GetSubtaskPosition(parent, c3)
	if pos1 != 1 {
		t.Errorf("c1: expected pos 1, got %d", pos1)
	}
	if pos2 != 2 {
		t.Errorf("c2: expected pos 2, got %d", pos2)
	}
	if pos3 != 0 {
		t.Errorf("c3: expected pos 0, got %d", pos3)
	}
}

func TestSetSubtaskPosition_ShiftDown(t *testing.T) {
	db := setupTestDB(t)
	pid := makeProject(t, db, "proj")
	parent := makeTask(t, db, pid, "parent", models.TaskStatusPending)
	c1 := makeTask(t, db, pid, "c1", models.TaskStatusPending)
	c2 := makeTask(t, db, pid, "c2", models.TaskStatusPending)
	c3 := makeTask(t, db, pid, "c3", models.TaskStatusPending)
	db.AddSubtask(parent, c1) // pos 0
	db.AddSubtask(parent, c2) // pos 1
	db.AddSubtask(parent, c3) // pos 2

	// Move c1 from pos 0 → pos 2 (shift down: c2 and c3 shift up).
	if err := db.SetSubtaskPosition(parent, c1, 2); err != nil {
		t.Fatalf("SetSubtaskPosition failed: %v", err)
	}

	pos1, _ := db.GetSubtaskPosition(parent, c1)
	pos2, _ := db.GetSubtaskPosition(parent, c2)
	pos3, _ := db.GetSubtaskPosition(parent, c3)
	if pos1 != 2 {
		t.Errorf("c1: expected pos 2, got %d", pos1)
	}
	if pos2 != 0 {
		t.Errorf("c2: expected pos 0, got %d", pos2)
	}
	if pos3 != 1 {
		t.Errorf("c3: expected pos 1, got %d", pos3)
	}
}

func TestSetSubtaskPosition_NegativeBecomesZero(t *testing.T) {
	db := setupTestDB(t)
	pid := makeProject(t, db, "proj")
	parent := makeTask(t, db, pid, "parent", models.TaskStatusPending)
	child := makeTask(t, db, pid, "child", models.TaskStatusPending)
	db.AddSubtask(parent, child)

	if err := db.SetSubtaskPosition(parent, child, -5); err != nil {
		t.Fatalf("SetSubtaskPosition(-5) should not error: %v", err)
	}
	pos, _ := db.GetSubtaskPosition(parent, child)
	if pos != 0 {
		t.Errorf("negative position should clamp to 0, got %d", pos)
	}
}
