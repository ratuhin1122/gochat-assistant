package nlu_test

import (
	"strings"
	"testing"

	"github.com/ratuhin1122/gochat-assistant/internal/nlu"
)

func TestLoadDefault(t *testing.T) {
	entries, err := nlu.LoadDefault()
	if err != nil {
		t.Fatalf("failed to load default embedded FAQ: %v", err)
	}

	if len(entries) != 15 {
		t.Fatalf("expected 15 entries, got %d", len(entries))
	}

	expectedIDs := map[string]bool{
		"greeting":        false,
		"how_are_you":     false,
		"what_can_you_do": false,
		"who_built_you":   false,
		"help":            false,
		"thanks":          false,
		"goodbye":         false,
		"hours":           false,
		"weather":         false,
		"pricing":         false,
		"languages":       false,
		"privacy":         false,
		"human_agent":     false,
		"feedback":        false,
		"joke":            false,
	}

	for i, entry := range entries {
		if entry.ID == "" {
			t.Errorf("entry %d has empty ID", i)
		}
		if _, ok := expectedIDs[entry.ID]; ok {
			expectedIDs[entry.ID] = true
		} else {
			t.Errorf("unexpected entry ID: %q", entry.ID)
		}

		if len(entry.Questions) < 3 || len(entry.Questions) > 5 {
			t.Errorf("entry %q: expected 3-5 questions, got %d", entry.ID, len(entry.Questions))
		}

		for qIdx, q := range entry.Questions {
			if strings.TrimSpace(q) == "" {
				t.Errorf("entry %q question %d is empty", entry.ID, qIdx)
			}
		}

		if strings.TrimSpace(entry.Answer) == "" {
			t.Errorf("entry %q has empty answer", entry.ID)
		}
	}

	for id, found := range expectedIDs {
		if !found {
			t.Errorf("expected entry ID %q was not found in FAQ", id)
		}
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		entries []nlu.FAQEntry
		wantErr bool
	}{
		{
			name:    "empty entries slice",
			entries: []nlu.FAQEntry{},
			wantErr: true,
		},
		{
			name: "empty ID",
			entries: []nlu.FAQEntry{
				{ID: "", Questions: []string{"q1", "q2", "q3"}, Answer: "a"},
			},
			wantErr: true,
		},
		{
			name: "whitespace ID",
			entries: []nlu.FAQEntry{
				{ID: "   ", Questions: []string{"q1", "q2", "q3"}, Answer: "a"},
			},
			wantErr: true,
		},
		{
			name: "empty questions list",
			entries: []nlu.FAQEntry{
				{ID: "test", Questions: []string{}, Answer: "a"},
			},
			wantErr: true,
		},
		{
			name: "empty question in questions list",
			entries: []nlu.FAQEntry{
				{ID: "test", Questions: []string{"valid", "  "}, Answer: "a"},
			},
			wantErr: true,
		},
		{
			name: "empty answer",
			entries: []nlu.FAQEntry{
				{ID: "test", Questions: []string{"q1", "q2", "q3"}, Answer: ""},
			},
			wantErr: true,
		},
		{
			name: "whitespace answer",
			entries: []nlu.FAQEntry{
				{ID: "test", Questions: []string{"q1", "q2", "q3"}, Answer: "   \t\n"},
			},
			wantErr: true,
		},
		{
			name: "valid entry",
			entries: []nlu.FAQEntry{
				{ID: "test", Questions: []string{"q1", "q2", "q3"}, Answer: "answer"},
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := nlu.Validate(tc.entries)
			if tc.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestLoad_Reader(t *testing.T) {
	yamlContent := `
- id: custom
  questions:
    - "custom q1"
    - "custom q2"
    - "custom q3"
  answer: "custom answer"
`
	entries, err := nlu.Load(strings.NewReader(yamlContent))
	if err != nil {
		t.Fatalf("unexpected error loading from reader: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if entries[0].ID != "custom" || entries[0].Answer != "custom answer" {
		t.Errorf("unexpected entry content: %+v", entries[0])
	}

	// Test default load via Load() with no args
	defaultEntries, err := nlu.Load()
	if err != nil {
		t.Fatalf("unexpected error loading via Load(): %v", err)
	}
	if len(defaultEntries) != 15 {
		t.Fatalf("expected 15 default entries, got %d", len(defaultEntries))
	}
}

func TestParse_InvalidYAML(t *testing.T) {
	_, err := nlu.Parse([]byte("invalid: yaml: [syntax"))
	if err == nil {
		t.Errorf("expected error on invalid YAML syntax, got nil")
	}

	_, err = nlu.Parse([]byte("   \n"))
	if err == nil {
		t.Errorf("expected error on empty YAML, got nil")
	}
}
