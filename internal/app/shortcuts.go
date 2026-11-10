package app

import (
	"runtime"

	"github.com/egoist/mygo/ui"
)

// Shortcuts follow the platform: Command on macOS, Control elsewhere, and
// Option or Alt. Hints and the help write them the same way the keys work.
func cmdSymbol() string {
	return map[bool]string{true: "⌘", false: "Ctrl"}[runtime.GOOS == "darwin"]
}

func altSymbol() string {
	return map[bool]string{true: "⌥", false: "Alt"}[runtime.GOOS == "darwin"]
}

// keyHint writes a shortcut as hints show it: ⌘K on macOS, Ctrl+K elsewhere.
func keyHint(key string) string {
	if runtime.GOOS == "darwin" {
		return "⌘" + key
	}
	return "Ctrl+" + key
}

// toggleMods is the modifier of the pane toggles: ⌘⌥ on macOS, Ctrl+Alt
// elsewhere, where Control+Option would reach the system's own gestures.
func toggleMods() ui.Modifiers {
	if runtime.GOOS == "darwin" {
		return ui.Cmd | ui.Alt
	}
	return ui.Ctrl | ui.Alt
}
