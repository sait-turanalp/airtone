package tui

import "github.com/charmbracelet/bubbles/key"

// keyMap is the single source of truth for key bindings. The footer (bubbles
// help) and the Update dispatch both read from it, so they can never drift.
type keyMap struct {
	Start    key.Binding
	Stop     key.Binding
	Mode     key.Binding
	Settings key.Binding
	Setup    key.Binding
	Recheck  key.Binding
	Preset   key.Binding
	Help     key.Binding
	Back     key.Binding
	Quit     key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Start:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "start")),
		Stop:     key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "stop")),
		Mode:     key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "switch mode")),
		Settings: key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "settings")),
		Setup:    key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "run setup")),
		Recheck:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "recheck")),
		Preset:   key.NewBinding(key.WithKeys("1", "2", "3", "4", "5"), key.WithHelp("1-3", "profile")),
		Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Back:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

// helpMap adapts context-built binding slices to the bubbles help.KeyMap
// interface, so the footer shows only what's actionable right now.
type helpMap struct {
	short []key.Binding
	full  [][]key.Binding
}

func (h helpMap) ShortHelp() []key.Binding  { return h.short }
func (h helpMap) FullHelp() [][]key.Binding { return h.full }
