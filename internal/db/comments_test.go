package db

import (
	"testing"

	"github.com/rogerrlee/tasks-watcher/internal/models"
)

func TestCreateComment(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := &models.Project{Name: "proj"}
	db.CreateProject(p)

	task := &models.Task{ProjectID: p.ID, Title: "Test task", Status: models.TaskStatusPending, Priority: models.PriorityMedium}
	db.CreateTask(task)

	c := &models.TaskComment{TaskID: task.ID, Author: "alice", Content: "Looks good!"}
	if err := db.CreateComment(c); err != nil {
		t.Fatalf("CreateComment failed: %v", err)
	}
	if c.ID == "" {
		t.Fatal("comment ID should be set")
	}
	if c.CreatedAt == 0 {
		t.Fatal("comment CreatedAt should be set")
	}

	// Get the comment
	fetched, err := db.GetComment(c.ID)
	if err != nil {
		t.Fatalf("GetComment failed: %v", err)
	}
	if fetched == nil {
		t.Fatal("expected comment, got nil")
	}
	if fetched.Author != "alice" {
		t.Errorf("expected author 'alice', got %q", fetched.Author)
	}
	if fetched.Content != "Looks good!" {
		t.Errorf("expected content 'Looks good!', got %q", fetched.Content)
	}
}

func TestListComments(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := &models.Project{Name: "proj"}
	db.CreateProject(p)

	task := &models.Task{ProjectID: p.ID, Title: "Test task", Status: models.TaskStatusPending, Priority: models.PriorityMedium}
	db.CreateTask(task)

	comments, err := db.ListComments(task.ID)
	if err != nil {
		t.Fatalf("ListComments failed: %v", err)
	}
	if len(comments) != 0 {
		t.Errorf("expected 0 comments, got %d", len(comments))
	}

	// Add two comments
	db.CreateComment(&models.TaskComment{TaskID: task.ID, Author: "alice", Content: "First"})
	db.CreateComment(&models.TaskComment{TaskID: task.ID, Author: "bob", Content: "Second"})

	comments, err = db.ListComments(task.ID)
	if err != nil {
		t.Fatalf("ListComments failed: %v", err)
	}
	if len(comments) != 2 {
		t.Errorf("expected 2 comments, got %d", len(comments))
	}
	if comments[0].Content != "First" {
		t.Errorf("expected first comment 'First', got %q", comments[0].Content)
	}
	if comments[1].Content != "Second" {
		t.Errorf("expected second comment 'Second', got %q", comments[1].Content)
	}

	// No comments for non-existent task
	other, err := db.ListComments("nonexistent")
	if err != nil {
		t.Fatalf("ListComments for nonexistent task failed: %v", err)
	}
	if len(other) != 0 {
		t.Errorf("expected 0 comments for nonexistent task, got %d", len(other))
	}
}

func TestUpdateComment(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := &models.Project{Name: "proj"}
	db.CreateProject(p)

	task := &models.Task{ProjectID: p.ID, Title: "Test task", Status: models.TaskStatusPending, Priority: models.PriorityMedium}
	db.CreateTask(task)

	c := &models.TaskComment{TaskID: task.ID, Author: "alice", Content: "Original"}
	db.CreateComment(c)

	c.Content = "Updated content"
	c.Author = "bob"
	if err := db.UpdateComment(c); err != nil {
		t.Fatalf("UpdateComment failed: %v", err)
	}

	fetched, err := db.GetComment(c.ID)
	if err != nil {
		t.Fatalf("GetComment failed: %v", err)
	}
	if fetched.Content != "Updated content" {
		t.Errorf("expected 'Updated content', got %q", fetched.Content)
	}
	if fetched.Author != "bob" {
		t.Errorf("expected author 'bob', got %q", fetched.Author)
	}
	if fetched.UpdatedAt == 0 {
		t.Error("UpdatedAt should be set after update")
	}
}

func TestDeleteComment(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p := &models.Project{Name: "proj"}
	db.CreateProject(p)

	task := &models.Task{ProjectID: p.ID, Title: "Test task", Status: models.TaskStatusPending, Priority: models.PriorityMedium}
	db.CreateTask(task)

	c := &models.TaskComment{TaskID: task.ID, Author: "alice", Content: "To be deleted"}
	db.CreateComment(c)

	if err := db.DeleteComment(c.ID); err != nil {
		t.Fatalf("DeleteComment failed: %v", err)
	}

	fetched, err := db.GetComment(c.ID)
	if err != nil {
		t.Fatalf("GetComment after delete failed: %v", err)
	}
	if fetched != nil {
		t.Error("expected nil after delete")
	}
}
