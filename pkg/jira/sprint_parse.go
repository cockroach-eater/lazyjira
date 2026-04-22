package jira

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// Example: com.atlassian.greenhopper.service.sprint.Sprint@1a2b3c[id=42,state=ACTIVE,name=Sprint 1,...]
var legacySprintPattern = regexp.MustCompile(`\[([^\[\]]*)\]$`)

func parseSprintRaw(data json.RawMessage) []Sprint {
	if len(data) == 0 {
		return nil
	}
	var modern []Sprint
	if err := json.Unmarshal(data, &modern); err == nil {
		filtered := modern[:0]
		for _, sprint := range modern {
			if sprint.ID != 0 || sprint.Name != "" {
				filtered = append(filtered, sprint)
			}
		}
		if len(filtered) > 0 {
			return filtered
		}
	}
	var legacy []string
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil
	}
	sprints := make([]Sprint, 0, len(legacy))
	for _, raw := range legacy {
		if sprint, ok := parseLegacySprint(raw); ok {
			sprints = append(sprints, sprint)
		}
	}
	return sprints
}

func parseLegacySprint(raw string) (Sprint, bool) {
	match := legacySprintPattern.FindStringSubmatch(raw)
	if len(match) < 2 {
		return Sprint{}, false
	}
	attributes := splitLegacyAttributes(match[1])
	sprint := Sprint{
		Name:  attributes["name"],
		State: attributes["state"],
	}
	if id, err := strconv.Atoi(attributes["id"]); err == nil {
		sprint.ID = id
	}
	if sprint.ID == 0 && sprint.Name == "" {
		return Sprint{}, false
	}
	return sprint, true
}

func splitLegacyAttributes(body string) map[string]string {
	attributes := make(map[string]string)
	start := 0
	depth := 0
	for index := 0; index <= len(body); index++ {
		if index == len(body) || (body[index] == ',' && depth == 0) {
			assignLegacyAttribute(attributes, body[start:index])
			start = index + 1
			continue
		}
		switch body[index] {
		case '[', '(':
			depth++
		case ']', ')':
			if depth > 0 {
				depth--
			}
		}
	}
	return attributes
}

func assignLegacyAttribute(attributes map[string]string, pair string) {
	eq := strings.IndexByte(pair, '=')
	if eq <= 0 {
		return
	}
	attributes[pair[:eq]] = pair[eq+1:]
}

func pickSprint(sprints []Sprint) *Sprint {
	if len(sprints) == 0 {
		return nil
	}
	var future, closed *Sprint
	for index := range sprints {
		sprint := &sprints[index]
		switch strings.ToLower(sprint.State) {
		case "active":
			return sprint
		case "future":
			if future == nil {
				future = sprint
			}
		case "closed":
			if closed == nil {
				closed = sprint
			}
		}
	}
	if future != nil {
		return future
	}
	if closed != nil {
		return closed
	}
	return &sprints[0]
}

// findSprintInCustomFields scans every customfield_* entry in the raw response
// and returns the first one whose payload parses as a non-empty sprint list
// with a concrete sprint ID. It lets us render the Sprint column before
// DiscoverFields finishes: Jira Cloud returns sprint data under the real
// custom field id (e.g. customfield_10020) even when queried by the "sprint"
// alias, and older Server instances respond the same way once the alias is
// passed through.
func findSprintInCustomFields(raw map[string]json.RawMessage) *Sprint {
	for key, data := range raw {
		if !strings.HasPrefix(key, "customfield_") {
			continue
		}
		sprints := parseSprintRaw(data)
		if sprint := pickSprint(sprints); sprint != nil && sprint.ID != 0 {
			return sprint
		}
	}
	return nil
}

// remapSprintField returns the fields map rewritten so that the "sprint" alias
// is sent as the real custom field id discovered at startup (for example
// customfield_10020 on Cloud or customfield_10010 on older Server/DC). Some
// Jira deployments reject writes to the alias, so PUT /issue must address the
// custom field directly. Returns the original map when the alias is absent or
// when no resolved id is available yet (first few seconds after startup), so
// callers can use it unconditionally.
func remapSprintField(fields map[string]any, resolvedID string) map[string]any {
	value, ok := fields[sprintFieldAlias]
	if !ok || resolvedID == "" || resolvedID == sprintFieldAlias {
		return fields
	}
	remapped := make(map[string]any, len(fields))
	for key, current := range fields {
		if key == sprintFieldAlias {
			continue
		}
		remapped[key] = current
	}
	remapped[resolvedID] = value
	return remapped
}
