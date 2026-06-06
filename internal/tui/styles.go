package tui

import "github.com/charmbracelet/lipgloss"

// Styles is the single source of truth for the TUI theme. Every view consumes
// these tokens — no hard-coded colors live in the view code. Colors are
// AdaptiveColor pairs so the UI reads correctly on light and dark terminals,
// and lipgloss degrades them gracefully under NO_COLOR / 16-color profiles.
//
// Palette is a calm teal accent (#137FA3) + semantic green/amber/red, for a
// trustworthy, Apple-like feel.
type Styles struct {
	App        lipgloss.Style // outer container padding
	Title      lipgloss.Style // "AirTone" badge
	ModeBadge  lipgloss.Style // ◈ Party / ◈ Instant
	Live       lipgloss.Style // ● LIVE (success)
	Idle       lipgloss.Style // ○ idle (muted)
	Panel      lipgloss.Style // bordered card
	PanelTitle lipgloss.Style // card heading
	Key        lipgloss.Style // key hints (accent)
	Dim        lipgloss.Style // secondary text
	URL        lipgloss.Style // the LAN link
	QR         lipgloss.Style // forces dark-on-white QR on any terminal
	ErrText    lipgloss.Style // inline error line
	CheckOK    lipgloss.Style // ✓
	CheckFail  lipgloss.Style // ✗
	Gate       lipgloss.Style // "terminal too small"
}

func newStyles() Styles {
	accent := lipgloss.AdaptiveColor{Light: "#0E6483", Dark: "#137FA3"}
	success := lipgloss.AdaptiveColor{Light: "#248A3D", Dark: "#30D158"}
	errc := lipgloss.AdaptiveColor{Light: "#D70015", Dark: "#FF453A"}
	muted := lipgloss.AdaptiveColor{Light: "#6E6E73", Dark: "#8E8E93"}
	border := lipgloss.AdaptiveColor{Light: "#C6C6C8", Dark: "#3A3A3C"}
	onAccent := lipgloss.Color("#FFFFFF")

	return Styles{
		App:        lipgloss.NewStyle().Padding(1, 2),
		Title:      lipgloss.NewStyle().Bold(true).Foreground(onAccent).Background(accent).Padding(0, 1),
		ModeBadge:  lipgloss.NewStyle().Bold(true).Foreground(accent),
		Live:       lipgloss.NewStyle().Bold(true).Foreground(success),
		Idle:       lipgloss.NewStyle().Foreground(muted),
		Panel:      lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).Padding(1, 2),
		PanelTitle: lipgloss.NewStyle().Bold(true).Foreground(accent),
		Key:        lipgloss.NewStyle().Bold(true).Foreground(accent),
		Dim:        lipgloss.NewStyle().Foreground(muted),
		URL:        lipgloss.NewStyle().Bold(true).Foreground(accent),
		QR:         lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#000000")),
		ErrText:    lipgloss.NewStyle().Foreground(errc),
		CheckOK:    lipgloss.NewStyle().Foreground(success),
		CheckFail:  lipgloss.NewStyle().Foreground(errc),
		Gate:       lipgloss.NewStyle().Foreground(muted).Padding(1, 2),
	}
}
