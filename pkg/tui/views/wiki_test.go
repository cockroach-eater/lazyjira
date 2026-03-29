package views

import "testing"

func TestWikiToPlain(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"plain text", "hello world", "hello world"},
		{"bold", "*bold text*", "bold text"},
		{"italic", "_italic text_", "italic text"},
		{"link with text", "[Google|https://google.com]", "Google (https://google.com)"},
		{"simple link", "[https://example.com]", "https://example.com"},
		{"heading h1", "h1. Title\nBody", "Title\nBody"},
		{"heading h3", "h3. Section", "Section"},
		{"code block", "{code}\nfmt.Println()\n{code}", "\nfmt.Println()\n"},
		{"noformat", "{noformat}raw{noformat}", "raw"},
		{"quote", "{quote}quoted{quote}", "quoted"},
		{"mixed", "*About* [Scrum|http://scrum.org]", "About Scrum (http://scrum.org)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wikiToPlain(tt.input)
			if got != tt.want {
				t.Errorf("wikiToPlain(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
