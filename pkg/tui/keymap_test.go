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

func TestDefaultKeymap_managedTabKeys(t *testing.T) {
	t.Parallel()

	km := DefaultKeymap()

	testkit.AssertEqual(t, "manageTab default", km.Match("M"), ActManageTab)
	testkit.AssertEqual(t, "deleteManagedTab default", km.Match("D"), ActDeleteManagedTab)
	testkit.AssertEqual(t, "reorderTabLeft default", km.Match("<"), ActReorderTabLeft)
	testkit.AssertEqual(t, "reorderTabRight default", km.Match(">"), ActReorderTabRight)
	// No collision with existing single-key bindings.
	testkit.AssertEqual(t, "S stays createSubtask", km.Match("S"), ActCreateSubtask)
	testkit.AssertEqual(t, "x stays closeJQLTab", km.Match("x"), ActCloseJQLTab)
}

func TestKeymapFromConfig_overridesManagedTabKeys(t *testing.T) {
	t.Parallel()

	var kcfg config.KeybindingConfig
	kcfg.Issues.ManageTab = "ctrl+s"
	kcfg.Issues.DeleteManagedTab = "ctrl+d"
	kcfg.Issues.ReorderTabLeft = "ctrl+h"
	kcfg.Issues.ReorderTabRight = "ctrl+l"

	km := KeymapFromConfig(kcfg)

	testkit.AssertSliceEqual(t, "manageTab override", km[ActManageTab], []string{"ctrl+s"})
	testkit.AssertSliceEqual(t, "deleteManagedTab override", km[ActDeleteManagedTab], []string{"ctrl+d"})
	testkit.AssertSliceEqual(t, "reorderTabLeft override", km[ActReorderTabLeft], []string{"ctrl+h"})
	testkit.AssertSliceEqual(t, "reorderTabRight override", km[ActReorderTabRight], []string{"ctrl+l"})
}

func TestKeymap_MatchUnknownReturnsEmpty(t *testing.T) {
	t.Parallel()

	keymap := DefaultKeymap()

	testkit.AssertEqual(t, "unknown key", keymap.Match("this-key-is-unbound"), Action(""))
	testkit.AssertEqual(t, "unknown nav key", keymap.MatchNav("this-key-is-unbound"), components.NavNone)
}
