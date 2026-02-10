package tui

// Keybinding constants
const (
	KeyTab      = "tab"
	KeyShiftTab = "shift+tab"
	KeyQuit     = "q"
	KeyCtrlC    = "ctrl+c"
	KeyPane1    = "1"
	KeyPane2    = "2"
	KeyPane3    = "3"
	KeyUp       = "up"
	KeyDown     = "down"
	KeyJ        = "j"
	KeyK        = "k"
	KeySettings = "s"
)

// HelpView returns a one-line help bar with common keybindings.
func HelpView() string {
	return StyleHelp.Render("Tab: cycle focus | 1/2/3: jump to pane | j/k: scroll | q: quit | s: settings (reserved)")
}
