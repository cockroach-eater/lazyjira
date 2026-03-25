package git

import (
	"regexp"
	"slices"
	"strings"
)

// BranchSearchResult holds branches matching an issue key.
type BranchSearchResult struct {
	Local  []string
	Remote []string
}

// SearchBranches finds local and remote branches containing the issue key.
// Uses case-insensitive matching with digit word boundary to avoid
// PLAT-3 matching PLAT-30.
func SearchBranches(dir, issueKey string) (*BranchSearchResult, error) {
	// Build pattern: (?i)PLAT-3([^0-9]|$)
	pattern := `(?i)` + regexp.QuoteMeta(issueKey) + `([^0-9]|$)`
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	result := &BranchSearchResult{}

	local, err := LocalBranches(dir)
	if err != nil {
		return nil, err
	}
	for _, b := range local {
		if re.MatchString(b) {
			result.Local = append(result.Local, b)
		}
	}

	remote, err := RemoteBranches(dir)
	if err != nil {
		return nil, err
	}
	for _, b := range remote {
		if re.MatchString(b) {
			// Skip if there's already a matching local branch with same name.
			if _, localEquiv, ok := strings.Cut(b, "/"); ok {
				if slices.Contains(result.Local, localEquiv) {
					continue
				}
			}
			result.Remote = append(result.Remote, b)
		}
	}

	return result, nil
}
