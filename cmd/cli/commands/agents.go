package commands

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// toInt safely converts an interface{} to int, returning 0 on failure.
func toInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return 0
	}
}

func AgentsCommand() *cobra.Command {
	agentsCmd := &cobra.Command{
		Use:   "agents",
		Short: "Show agent overview",
		Long:  "See what each agent/tool is working on. Groups tasks by assignee (claude-code, cursor, cli, manual) and shows per-agent stats: active, pending, completed, failed.",
		Example: `  tasks-watcher agents overview`,
	}
	agentsCmd.AddCommand(
		agentsOverviewCmd(),
	)
	return agentsCmd
}

func agentsOverviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "overview",
		Short: "Show what each agent is working on",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiRequest("GET", "/api/agents/overview", nil)
			if err != nil {
				return err
			}
			var result map[string]interface{}
			if err := json.Unmarshal(resp, &result); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			agents, ok := result["agents"].([]interface{})
			if !ok || len(agents) == 0 {
				fmt.Println("No agents found.")
				return nil
			}

			// Get active tasks for each agent
			taskResp, err := apiRequest("GET", "/api/tasks?status=in_progress", nil)
			tasks := []map[string]interface{}{}
			if err == nil {
				var taskResult map[string]interface{}
				if err2 := json.Unmarshal(taskResp, &taskResult); err2 == nil {
					if t, ok := taskResult["tasks"].([]interface{}); ok {
						for _, item := range t {
							if m, ok := item.(map[string]interface{}); ok {
								tasks = append(tasks, m)
							}
						}
					}
				}
			}

			// Group active tasks by assignee
			activeByAgent := make(map[string][]map[string]interface{})
			for _, t := range tasks {
				if asgn, ok := t["assignee"].(string); ok && asgn != "" {
					activeByAgent[asgn] = append(activeByAgent[asgn], t)
				}
			}

			fmt.Println()
			fmt.Println("  Agent Overview")
			fmt.Println("  " + strings.Repeat("─", 60))

			for _, a := range agents {
				agent, ok := a.(map[string]interface{})
				if !ok {
					continue
				}
				name := fmt.Sprintf("%s", agent["name"])
				active := toInt(agent["active_tasks"])
				pending := toInt(agent["pending_tasks"])
				completed := toInt(agent["completed_tasks"])
				failed := toInt(agent["failed_tasks"])
				total := toInt(agent["total_tasks"])

				icon := "🤖"
				if strings.Contains(name, "cursor") || strings.Contains(name, "Cursor") {
					icon = "📎"
				} else if name == "manual" || name == "" {
					icon = "👤"
				}

				fmt.Printf("  %s %s\n", icon, name)
				if activeTasks, ok := activeByAgent[name]; ok && len(activeTasks) > 0 {
					for _, t := range activeTasks {
						title := fmt.Sprintf("%s", t["title"])
						if len(title) > 45 {
							title = title[:42] + "..."
						}
						updated := int64(0)
						if u, ok := t["updated_at"].(float64); ok {
							updated = int64(u)
						}
						rel := relativeTime(updated)
						fmt.Printf("    ▶  %s  (%s)\n", title, rel)
					}
				} else if active > 0 {
					fmt.Printf("    ▶  %d task(s) active\n", active)
				} else {
					fmt.Printf("    ○  No active tasks\n")
				}
				fmt.Printf("    ↳ %d total | %d done | %d pending | %d failed\n",
					total, completed, pending, failed)
				fmt.Println()
			}

			return nil
		},
	}
	return cmd
}

func relativeTime(ts int64) string {
	if ts == 0 {
		return "just now"
	}
	d := time.Since(time.Unix(ts, 0))
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		m := int(d.Minutes())
		return fmt.Sprintf("%dm ago", m)
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		return fmt.Sprintf("%dh ago", h)
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}
