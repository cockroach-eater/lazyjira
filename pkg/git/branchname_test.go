package git

import (
	"strings"
	"testing"
)

func TestGenerateBranchName(t *testing.T) {
	tests := []struct {
		name      string
		data      BranchTemplateData
		tmpl      string
		asciiOnly bool
		want      string
	}{
		{
			name: "default template",
			data: BranchTemplateData{
				Key:     "PROJ-123",
				Summary: "fix-login",
			},
			want: "PROJ-123-fix-login",
		},
		{
			name: "with parent key",
			data: BranchTemplateData{
				Key:       "PROJ-142",
				ParentKey: "PROJ-100",
				Summary:   "fix-login-validation",
			},
			tmpl: "{{.ParentKey}}/{{.Key}}_{{.Summary}}",
			want: "PROJ-100/PROJ-142_fix-login-validation",
		},
		{
			name: "empty parent key strips leading slash",
			data: BranchTemplateData{
				Key:     "PROJ-142",
				Summary: "fix-login",
			},
			tmpl: "{{.ParentKey}}/{{.Key}}_{{.Summary}}",
			want: "PROJ-142_fix-login",
		},
		{
			name: "all fields",
			data: BranchTemplateData{
				Key:        "PROJ-42",
				ProjectKey: "PROJ",
				Number:     "42",
				Summary:    "add-feature",
				Type:       "Story",
				ParentKey:  "PROJ-10",
			},
			tmpl: "{{.Type}}/{{.ParentKey}}/{{.Key}}-{{.Summary}}",
			want: "Story/PROJ-10/PROJ-42-add-feature",
		},
		{
			name: "ascii only strips unicode from summary",
			data: BranchTemplateData{
				Key:       "PROJ-1",
				ParentKey: "PROJ-2",
				Summary:   "fix-\u00e4\u00f6\u00fc-bug",
			},
			tmpl:      "{{.ParentKey}}/{{.Key}}_{{.Summary}}",
			asciiOnly: true,
			want:      "PROJ-2/PROJ-1_fix-bug",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateBranchName(tt.data, tt.tmpl, tt.asciiOnly)
			if got != tt.want {
				t.Errorf("GenerateBranchName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		asciiOnly bool
		want      string
	}{
		{"spaces to hyphens", "hello world", false, "hello-world"},
		{"multiple hyphens", "a---b", false, "a-b"},
		{"trailing dot", "branch.", false, "branch"},
		{"trailing slash", "branch/", false, "branch"},
		{"max length truncation", "a-" + strings.Repeat("b", 100), false, "a-" + strings.Repeat("b", 58)},
		{"slash preserved", "parent/child", false, "parent/child"},
		{"leading slash stripped", "/child", false, "child"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Sanitize(tt.input, tt.asciiOnly)
			if got != tt.want {
				t.Errorf("Sanitize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeSummary(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		asciiOnly bool
		want      string
	}{
		{"basic", "Fix Login Bug", false, "fix-login-bug"},
		{"special chars", "Add feature (v2) & test!", false, "add-feature-v2-test"},
		{"ascii only", "Umlaut aeoeue", true, "umlaut-aeoeue"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeSummary(tt.input, tt.asciiOnly)
			if got != tt.want {
				t.Errorf("SanitizeSummary(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
