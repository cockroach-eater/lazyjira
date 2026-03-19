//go:build !demo

package main

import (
	"errors"

	"github.com/textfuel/lazyjira/pkg/config"
	"github.com/textfuel/lazyjira/pkg/jira"
	"github.com/textfuel/lazyjira/pkg/tui"
)

func startDemo(_ *config.Config) (jira.ClientInterface, tui.AuthMethod, func(), error) {
	return nil, "", nil, errors.New("demo mode not available (rebuild with: go build -tags demo)")
}
