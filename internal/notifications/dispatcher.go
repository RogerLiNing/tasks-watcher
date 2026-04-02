package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/rogerrlee/tasks-watcher/internal/db"
	"github.com/rogerrlee/tasks-watcher/internal/models"
)

type Dispatcher struct {
	db  *db.DB
	sse interface{ Broadcast(models.SSEEvent) }
}

func NewDispatcher(database *db.DB, sseBroadcaster interface{ Broadcast(models.SSEEvent) }) *Dispatcher {
	return &Dispatcher{db: database, sse: sseBroadcaster}
}

func (d *Dispatcher) Notify(eventType string, task *models.Task) {
	// 1. Save to DB as notification
	msg := buildMessage(eventType, task)
	n := &models.Notification{
		TaskID:    task.ID,
		Type:      eventType,
		Message:   msg,
		Read:      false,
	}
	if err := d.db.CreateNotification(n); err != nil {
		log.Printf("[notify] failed to save notification: %v", err)
	}

	// 2. Send via configured channels
	go d.sendChannels(eventType, task, msg)
}

func (d *Dispatcher) sendChannels(eventType string, task *models.Task, msg string) {
	if shouldNotifyOS(eventType) {
		macosCfg, _ := d.db.GetNotificationConfig("macos")
		if macosCfg != nil && macosCfg.Enabled {
			d.macosNotification(msg, task.Title)
		}

		emailCfg, _ := d.db.GetNotificationConfig("email")
		if emailCfg != nil && emailCfg.Enabled {
			d.sendEmail(task, msg)
		}
	}

	d.sendWebhooks(eventType, task)
}

func buildMessage(eventType string, task *models.Task) string {
	switch eventType {
	case models.EventTaskCreated:
		return fmt.Sprintf("New task created: %s", task.Title)
	case models.EventTaskStarted:
		return fmt.Sprintf("Task started: %s", task.Title)
	case models.EventTaskCompleted:
		return fmt.Sprintf("Task completed: %s", task.Title)
	case models.EventTaskFailed:
		return fmt.Sprintf("Task failed: %s — %s", task.Title, task.ErrorMessage)
	case models.EventTaskCancelled:
		return fmt.Sprintf("Task cancelled: %s", task.Title)
	default:
		return fmt.Sprintf("Task updated: %s", task.Title)
	}
}

func shouldNotifyOS(eventType string) bool {
	switch eventType {
	case models.EventTaskStarted, models.EventTaskCompleted, models.EventTaskFailed:
		return true
	default:
		return false
	}
}

func (d *Dispatcher) macosNotification(body, title string) {
	if runtime.GOOS != "darwin" {
		return
	}
	script := fmt.Sprintf(`display notification "%s" with title "Tasks Watcher: %s"`, body, title)
	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		log.Printf("[notify] macOS notification failed: %v", err)
	}
}

func (d *Dispatcher) sendEmail(task *models.Task, msg string) {
	cfg, err := d.db.GetNotificationConfig("email")
	if err != nil || cfg == nil || !cfg.Enabled {
		return
	}

	emailCfg := parseEmailConfig(cfg.Config)
	if emailCfg.SMTPHost == "" || emailCfg.SMTPUsername == "" || len(emailCfg.ToAddresses) == 0 {
		return
	}

	subject := fmt.Sprintf("[Tasks Watcher] %s", msg)
	body := fmt.Sprintf(
		"Task: %s\nStatus: %s\nPriority: %s\nAssignee: %s\n\n%s\n\nView at: http://localhost:4242",
		task.Title, task.Status, task.Priority, task.Assignee, msg,
	)

	auth := smtp.PlainAuth("", emailCfg.SMTPUsername, emailCfg.SMTPPassword, emailCfg.SMTPHost)
	addr := fmt.Sprintf("%s:%d", emailCfg.SMTPHost, emailCfg.SMTPPort)

	for _, to := range emailCfg.ToAddresses {
		go func(toAddr string) {
			msgBody := buildEmailMsg(emailCfg.FromAddress, toAddr, subject, body)
			err := smtp.SendMail(addr, auth, emailCfg.FromAddress, []string{toAddr}, []byte(msgBody))
			if err != nil {
				log.Printf("[notify] email to %s failed: %v", toAddr, err)
			} else {
				log.Printf("[notify] email sent to %s", toAddr)
			}
		}(to)
	}
}

func parseEmailConfig(config map[string]interface{}) models.EmailConfig {
	cfg := models.EmailConfig{SMTPPort: 587}
	if v, ok := config["smtp_host"].(string); ok {
		cfg.SMTPHost = v
	}
	if v, ok := config["smtp_port"].(float64); ok {
		cfg.SMTPPort = int(v)
	}
	if v, ok := config["smtp_username"].(string); ok {
		cfg.SMTPUsername = v
	}
	if v, ok := config["smtp_password"].(string); ok {
		cfg.SMTPPassword = v
	}
	if v, ok := config["from_address"].(string); ok {
		cfg.FromAddress = v
	}
	if v, ok := config["to_addresses"].([]interface{}); ok {
		for _, a := range v {
			if s, ok := a.(string); ok {
				cfg.ToAddresses = append(cfg.ToAddresses, s)
			}
		}
	}
	return cfg
}

func buildEmailMsg(from, to, subject, body string) string {
	now := time.Now().Format(time.RFC1123Z)
	return fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, to, subject, now, body,
	)
}

func (d *Dispatcher) sendWebhooks(eventType string, task *models.Task) {
	webhooks, err := d.db.ListWebhooks()
	if err != nil {
		return
	}

	payload := map[string]interface{}{
		"event": eventType,
		"task":  task,
		"time":  models.Now(),
	}
	body, _ := json.Marshal(payload)

	for _, wh := range webhooks {
		if !wh.Active {
			continue
		}
		if !matchesEvent(eventType, wh.Events) {
			continue
		}

		go func(url string) {
			req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Tasks-Watcher-Event", eventType)

			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				log.Printf("[notify] webhook failed for %s: %v", url, err)
				return
			}
			resp.Body.Close()
			if resp.StatusCode >= 400 {
				log.Printf("[notify] webhook error %d from %s", resp.StatusCode, url)
			}
		}(wh.URL)
	}
}

func matchesEvent(eventType, filter string) bool {
	if filter == "" || filter == "*" || filter == "task.*" {
		return true
	}
	parts := strings.Split(filter, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == eventType || p == "task.*" {
			return true
		}
	}
	return false
}
