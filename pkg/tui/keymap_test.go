package tui

import (
	"testing"

	"github.com/textfuel/lazyjira/v2/pkg/config"
	"github.com/textfuel/lazyjira/v2/pkg/internal/testkit"
	"github.com/textfuel/lazyjira/v2/pkg/tui/components"
)

func TestKeymapFromConfig_OverridesAndMatches(t *testing.T) {
	t.Parallel()

	var keybindingConfig config.KeybindingConfig
	keybindingConfig.Universal.Quit = "Q"
	keybindingConfig.Navigation.Down = "n"

	keymap := KeymapFromConfig(keybindingConfig)

	testkit.AssertSliceEqual(t, "quit binding overridden", keymap[ActQuit], []string{"Q"})
	testkit.AssertEqual(t, "Match resolves override", keymap.Match("Q"), ActQuit)
	testkit.AssertEqual(t, "MatchNav resolves override", keymap.MatchNav("n"), components.NavDown)
}

func TestKeymapFromConfig_EmptyKeepsDefaults(t *testing.T) {
	t.Parallel()

	defaults := DefaultKeymap()
	keymap := KeymapFromConfig(config.KeybindingConfig{})

	testkit.AssertSliceEqual(t, "quit default preserved", keymap[ActQuit], defaults[ActQuit])
}

func TestKeymap_MatchUnknownReturnsEmpty(t *testing.T) {
	t.Parallel()

	keymap := DefaultKeymap()

	testkit.AssertEqual(t, "unknown key", keymap.Match("this-key-is-unbound"), Action(""))
	testkit.AssertEqual(t, "unknown nav key", keymap.MatchNav("this-key-is-unbound"), components.NavNone)
}

// TestDefaultKeymapHasNoDuplicateKeys guards a silent failure mode: Keymap.Match
// iterates a map, so a key bound to two actions resolves to a random one of
// them, differently on each run.
func TestDefaultKeymapHasNoDuplicateKeys(t *testing.T) {
	t.Parallel()
	owners := map[string][]Action{}
	for action, keys := range DefaultKeymap() {
		for _, key := range keys {
			owners[key] = append(owners[key], action)
		}
	}
	for key, actions := range owners {
		if len(actions) > 1 {
			t.Errorf("key %q is bound to %v; Match would pick one at random", key, actions)
		}
	}
}

func TestKeymapFromConfigOverridesFilterAssignees(t *testing.T) {
	t.Parallel()
	km := KeymapFromConfig(config.KeybindingConfig{Issues: config.IssueKeys{FilterAssignees: "F"}})
	if got := km.Keys(ActFilterAssignees); got != "F" {
		t.Errorf("filterAssignees = %q, want F", got)
	}
}
