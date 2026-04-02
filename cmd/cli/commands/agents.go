package commands

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

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
			json.Unmarshal(resp, &result)

			agents, ok := result["agents"].([]interface{})
			if !ok || len(agents) == 0 {
				fmt.Println("No agents found.")
				return nil
			}

			// Get active tasks for each agent
			taskResp, _ := apiRequest("GET", "/api/tasks?status=in_progress", nil)
			var taskResult map[string]interface{}
			json.Unmarshal(taskResp, &taskResult)
			tasks := []map[string]interface{}{}
			if t, ok := taskResult["tasks"].([]interface{}); ok {
				for _, item := range t {
					if m, ok := item.(map[string]interface{}); ok {
						tasks = append(tasks, m)
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
				agent, _ := a.(map[string]interface{})
				name := fmt.Sprintf("%s", agent["name"])
				active := int(agent["active_tasks"].(float64))
				pending := int(agent["pending_tasks"].(float64))
				completed := int(agent["completed_tasks"].(float64))
				failed := int(agent["failed_tasks"].(float64))
				total := int(agent["total_tasks"].(float64))

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
