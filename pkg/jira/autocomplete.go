package jira

import (
	"context"
	"net/http"
	"net/url"
)

// AutocompleteField represents a JQL field from the autocomplete API.
type AutocompleteField struct {
	Value       string   `json:"value"`       // field name (e.g. "status")
	DisplayName string   `json:"displayName"` // human-readable (e.g. "Status")
	Operators   []string `json:"operators"`   // valid operators
}

// AutocompleteSuggestion represents a value suggestion.
type AutocompleteSuggestion struct {
	Value       string `json:"value"`
	DisplayName string `json:"displayName"`
}

// GetJQLAutocompleteData fetches all JQL field names (one-time, cacheable).
// Endpoint: GET /jql/autocompletedata
func (c *Client) GetJQLAutocompleteData(ctx context.Context) ([]AutocompleteField, error) {
	var result struct {
		VisibleFieldNames []AutocompleteField `json:"visibleFieldNames"`
	}
	if err := c.do(ctx, http.MethodGet, "/jql/autocompletedata", nil, &result); err != nil {
		return nil, err
	}
	return result.VisibleFieldNames, nil
}

// GetJQLAutocompleteSuggestions fetches value suggestions for a field.
// Endpoint: GET /jql/autocompletedata/suggestions?fieldName={field}&fieldValue={partial}
func (c *Client) GetJQLAutocompleteSuggestions(ctx context.Context, fieldName, fieldValue string) ([]AutocompleteSuggestion, error) {
	params := url.Values{}
	params.Set("fieldName", fieldName)
	params.Set("fieldValue", fieldValue)

	var result struct {
		Results []AutocompleteSuggestion `json:"results"`
	}
	if err := c.do(ctx, http.MethodGet, "/jql/autocompletedata/suggestions?"+params.Encode(), nil, &result); err != nil {
		return nil, err
	}
	return result.Results, nil
}
