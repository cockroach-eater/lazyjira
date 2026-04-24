package jira

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestParseSprintRaw(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []Sprint
	}{
		{
			name:  "empty input returns nil",
			input: "",
			want:  nil,
		},
		{
			name:  "modern JSON array",
			input: `[{"id":42,"name":"Sprint 1","state":"active"}]`,
			want:  []Sprint{{ID: 42, Name: "Sprint 1", State: "active"}},
		},
		{
			name:  "modern JSON skips empty entries",
			input: `[{"id":0,"name":""},{"id":42,"name":"Sprint 1","state":"active"}]`,
			want:  []Sprint{{ID: 42, Name: "Sprint 1", State: "active"}},
		},
		{
			name:  "legacy stringified array",
			input: `["com.atlassian.greenhopper.service.sprint.Sprint@1a2b3c[id=42,state=ACTIVE,name=Sprint 1]"]`,
			want:  []Sprint{{ID: 42, Name: "Sprint 1", State: "ACTIVE"}},
		},
		{
			name:  "non-array JSON returns nil",
			input: `{"id":42}`,
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSprintRaw(json.RawMessage(tt.input))
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseLegacySprint(t *testing.T) {
	tests := []struct {
		name  string
		input string
		ok    bool
		want  Sprint
	}{
		{
			name:  "full attributes",
			input: "com.atlassian.greenhopper.service.sprint.Sprint@1a2b3c[id=42,state=ACTIVE,name=Sprint 1]",
			ok:    true,
			want:  Sprint{ID: 42, Name: "Sprint 1", State: "ACTIVE"},
		},
		{
			name:  "id only is enough",
			input: "Sprint@x[id=7]",
			ok:    true,
			want:  Sprint{ID: 7},
		},
		{
			name:  "commas inside parens do not split attributes",
			input: "Sprint@x[id=3,extra=call(a,b),name=Keep]",
			ok:    true,
			want:  Sprint{ID: 3, Name: "Keep"},
		},
		{
			name:  "no brackets",
			input: "Sprint@x",
			ok:    false,
		},
		{
			name:  "empty brackets",
			input: "Sprint@x[]",
			ok:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseLegacySprint(tt.input)
			if ok != tt.ok {
				t.Fatalf("ok=%v, want %v", ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestPickSprint(t *testing.T) {
	active := Sprint{ID: 1, Name: "A", State: "active"}
	future := Sprint{ID: 2, Name: "F", State: "future"}
	closed := Sprint{ID: 3, Name: "C", State: "closed"}
	unknown := Sprint{ID: 4, Name: "U", State: "draft"}

	tests := []struct {
		name  string
		input []Sprint
		want  *Sprint
	}{
		{
			name:  "empty returns nil",
			input: nil,
			want:  nil,
		},
		{
			name:  "active wins over future and closed",
			input: []Sprint{closed, future, active},
			want:  &active,
		},
		{
			name:  "future wins over closed when no active",
			input: []Sprint{closed, future},
			want:  &future,
		},
		{
			name:  "closed alone is picked",
			input: []Sprint{closed},
			want:  &closed,
		},
		{
			name:  "unknown state falls back to first",
			input: []Sprint{unknown},
			want:  &unknown,
		},
		{
			name: "state matching is case insensitive",
			input: []Sprint{
				{ID: 1, Name: "F", State: "future"},
				{ID: 9, Name: "A", State: "ACTIVE"},
			},
			want: &Sprint{ID: 9, Name: "A", State: "ACTIVE"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pickSprint(tt.input)
			if (got == nil) != (tt.want == nil) {
				t.Fatalf("nil mismatch: got %v, want %v", got, tt.want)
			}
			if got != nil && *got != *tt.want {
				t.Errorf("got %+v, want %+v", *got, *tt.want)
			}
		})
	}
}

func TestFindSprintInCustomFields(t *testing.T) {
	sprintJSON := json.RawMessage(`[{"id":42,"name":"Sprint 1","state":"active"}]`)
	emptySprintJSON := json.RawMessage(`[{"id":0,"name":""}]`)
	otherJSON := json.RawMessage(`"not a sprint"`)

	tests := []struct {
		name  string
		input map[string]json.RawMessage
		want  *Sprint
	}{
		{
			name: "sprint found in custom field",
			input: map[string]json.RawMessage{
				"customfield_10020": sprintJSON,
			},
			want: &Sprint{ID: 42, Name: "Sprint 1", State: "active"},
		},
		{
			name: "non custom field keys are ignored",
			input: map[string]json.RawMessage{
				"summary": sprintJSON,
			},
			want: nil,
		},
		{
			name: "empty sprint payload is skipped",
			input: map[string]json.RawMessage{
				"customfield_10010": emptySprintJSON,
				"customfield_99999": otherJSON,
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findSprintInCustomFields(tt.input)
			if (got == nil) != (tt.want == nil) {
				t.Fatalf("nil mismatch: got %v, want %v", got, tt.want)
			}
			if got != nil && *got != *tt.want {
				t.Errorf("got %+v, want %+v", *got, *tt.want)
			}
		})
	}
}

func TestRemapSprintField(t *testing.T) {
	tests := []struct {
		name       string
		fields     map[string]any
		resolvedID string
		want       map[string]any
	}{
		{
			name:       "no sprint key returns same map",
			fields:     map[string]any{"summary": "hi"},
			resolvedID: "customfield_10020",
			want:       map[string]any{"summary": "hi"},
		},
		{
			name:       "empty resolvedID returns same map",
			fields:     map[string]any{"sprint": 42},
			resolvedID: "",
			want:       map[string]any{"sprint": 42},
		},
		{
			name:       "resolvedID equal to alias returns same map",
			fields:     map[string]any{"sprint": 42},
			resolvedID: "sprint",
			want:       map[string]any{"sprint": 42},
		},
		{
			name:       "rewrites alias to resolved id and preserves other fields",
			fields:     map[string]any{"sprint": 42, "summary": "hi"},
			resolvedID: "customfield_10020",
			want:       map[string]any{"customfield_10020": 42, "summary": "hi"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := remapSprintField(tt.fields, tt.resolvedID)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
