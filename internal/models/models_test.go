package models

import (
	"testing"
	"time"
)

func TestValidTaskStatus(t *testing.T) {
	tests := []struct {
		status  string
		wantOk  bool
	}{
		{"pending", true},
		{"in_progress", true},
		{"completed", true},
		{"failed", true},
		{"cancelled", true},
		{"invalid", false},
		{"", false},
		{"PENDING", false}, // case-sensitive
	}
	for _, tt := range tests {
		got := ValidTaskStatus(tt.status)
		if got != tt.wantOk {
			t.Errorf("ValidTaskStatus(%q) = %v, want %v", tt.status, got, tt.wantOk)
		}
	}
}

func TestValidPriority(t *testing.T) {
	tests := []struct {
		priority string
		wantOk   bool
	}{
		{"low", true},
		{"medium", true},
		{"high", true},
		{"urgent", true},
		{"invalid", false},
		{"", false},
	}
	for _, tt := range tests {
		got := ValidPriority(tt.priority)
		if got != tt.wantOk {
			t.Errorf("ValidPriority(%q) = %v, want %v", tt.priority, got, tt.wantOk)
		}
	}
}

func TestValidTaskMode(t *testing.T) {
	tests := []struct {
		mode   string
		wantOk bool
	}{
		{"", true},
		{"sequential", true},
		{"parallel", true},
		{"invalid", false},
		{"SEQUENTIAL", false},
	}
	for _, tt := range tests {
		got := ValidTaskMode(tt.mode)
		if got != tt.wantOk {
			t.Errorf("ValidTaskMode(%q) = %v, want %v", tt.mode, got, tt.wantOk)
		}
	}
}

func TestSerializeDescription(t *testing.T) {
	tests := []struct {
		name string
		desc map[string]string
		want string
	}{
		{"nil", nil, ""},
		{"empty", map[string]string{}, ""},
		{"single en", map[string]string{"en": "hello"}, `{"en":"hello"}`},
		{"multi locale", map[string]string{"en": "hello", "zh": "你好"}, `{"en":"hello","zh":"你好"}`},
		{"empty value", map[string]string{"en": ""}, `{"en":""}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SerializeDescription(tt.desc)
			if got != tt.want {
				t.Errorf("SerializeDescription(%v) = %q, want %q", tt.desc, got, tt.want)
			}
		})
	}
}

func TestMergeDescription(t *testing.T) {
	tests := []struct {
		name string
		prev any
		val  any
		want map[string]string
	}{
		{
			name: "nil prev nil val",
			prev: nil,
			val:  nil,
			want: map[string]string{},
		},
		{
			name: "nil prev string val",
			prev: nil,
			val:  "plain text",
			want: map[string]string{"en": "plain text"},
		},
		{
			name: "nil prev map[string]string val",
			prev: nil,
			val:  map[string]string{"zh": "你好"},
			want: map[string]string{"zh": "你好"},
		},
		{
			name: "nil prev map[string]interface{} val",
			prev: nil,
			val:  map[string]interface{}{"en": "hello", "fr": "bonjour"},
			want: map[string]string{"en": "hello", "fr": "bonjour"},
		},
		{
			name: "nil prev json string val",
			prev: nil,
			val:  `{"en":"hello","zh":"你好"}`,
			want: map[string]string{"en": "hello", "zh": "你好"},
		},
		{
			name: "prev overridden",
			prev: map[string]string{"en": "old"},
			val:  map[string]string{"en": "new"},
			want: map[string]string{"en": "new"},
		},
		{
			name: "prev retained and new added",
			prev: map[string]string{"zh": "你好"},
			val:  map[string]string{"en": "hello"},
			want: map[string]string{"zh": "你好", "en": "hello"},
		},
		{
			name: "nil prev unsupported type (number)",
			prev: nil,
			val:  42,
			want: map[string]string{},
		},
		{
			name: "nil prev empty string",
			prev: nil,
			val:  "",
			want: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var prev map[string]string
			if tt.prev != nil {
				prev = tt.prev.(map[string]string)
			}
			got := MergeDescription(prev, tt.val)
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("MergeDescription[%q] = %q, want %q", k, got[k], v)
				}
			}
			for k, v := range got {
				if _, ok := tt.want[k]; !ok {
					t.Errorf("MergeDescription unexpected key %q = %q", k, v)
				}
			}
		})
	}
}

func TestNow_ReturnsUnixTimestamp(t *testing.T) {
	before := time.Now().Unix()
	got := Now()
	after := time.Now().Unix()
	if got < before || got > after {
		t.Errorf("Now() = %d, want between %d and %d", got, before, after)
	}
}

func TestTaskIsTerminal(t *testing.T) {
	tests := []struct {
		status   TaskStatus
		terminal bool
	}{
		{TaskStatusPending, false},
		{TaskStatusInProgress, false},
		{TaskStatusCompleted, true},
		{TaskStatusFailed, true},
		{TaskStatusCancelled, true},
	}
	for _, tt := range tests {
		task := &Task{Status: tt.status}
		got := task.IsTerminal()
		if got != tt.terminal {
			t.Errorf("Task{Status:%q}.IsTerminal() = %v, want %v", tt.status, got, tt.terminal)
		}
	}
}
