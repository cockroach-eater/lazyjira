package jira

import (
	"encoding/json"
	"regexp"
	"strconv"
)

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
	for index := range len(pair) {
		if pair[index] == '=' {
			key := pair[:index]
			value := pair[index+1:]
			if key != "" {
				attributes[key] = value
			}
			return
		}
	}
}

func pickSprint(sprints []Sprint) *Sprint {
	if len(sprints) == 0 {
		return nil
	}
	var future, closed *Sprint
	for index := range sprints {
		sprint := &sprints[index]
		switch sprint.State {
		case "active", "ACTIVE":
			return sprint
		case "future", "FUTURE":
			if future == nil {
				future = sprint
			}
		case "closed", "CLOSED":
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
