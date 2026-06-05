// Package tui is the interactive AirTone front-end (Bubble Tea). It orchestrates
// the engine and shows live status read from the Snapcast control API. It does
// not touch the audio path itself.
package tui

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mdp/qrterminal/v3"

	"github.com/sait-turanalp/airtone/internal/doctor"
	"github.com/sait-turanalp/airtone/internal/engine"
	"github.com/sait-turanalp/airtone/internal/rpc"
)

const httpPort = "1780"

// --- messages ---

type tickMsg time.Time
type statusMsg struct {
	st  *rpc.Status
	err error
}
type doctorMsg []doctor.Check
type startedMsg struct{ err error }
type stoppedMsg struct{ err error }

// --- model ---

type model struct {
	width  int
	checks []doctor.Check
	ready  bool
	status *rpc.Status
	url    string
	busy   string // "Starting…" / "Stopping…" while a transition runs
	note   string // transient message (e.g. an error)
}

// Run launches the interactive TUI.
func Run() error {
	p := tea.NewProgram(model{url: "http://" + engine.LANIP() + ":" + httpPort}, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m model) Init() tea.Cmd {
	return tea.Batch(checkDoctor, pollStatus, tick())
}

// --- commands ---

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func checkDoctor() tea.Msg { return doctorMsg(doctor.Run()) }

func pollStatus() tea.Msg {
	st, err := rpc.GetStatus(rpc.DefaultAddr)
	return statusMsg{st: st, err: err}
}

func startEngine() tea.Msg {
	var b bytes.Buffer
	return startedMsg{err: engine.Start(&b)}
}

func stopEngine() tea.Msg {
	var b bytes.Buffer
	return stoppedMsg{err: engine.Stop(&b)}
}

// --- update ---

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "s":
			if m.ready && m.busy == "" && !m.running() {
				m.busy, m.note = "Starting…", ""
				return m, startEngine
			}
		case "x":
			if m.running() && m.busy == "" {
				m.busy, m.note = "Stopping…", ""
				return m, stopEngine
			}
		case "r":
			return m, checkDoctor
		}
	case tickMsg:
		return m, tea.Batch(pollStatus, tick())
	case statusMsg:
		if msg.err != nil {
			m.status = nil
		} else {
			m.status = msg.st
		}
	case doctorMsg:
		m.checks = msg
		m.ready = doctor.OK(msg)
	case startedMsg:
		m.busy = ""
		if msg.err != nil {
			m.note = "start failed: " + msg.err.Error()
		}
	case stoppedMsg:
		m.busy = ""
		m.status = nil
		if msg.err != nil {
			m.note = "stop failed: " + msg.err.Error()
		}
	}
	return m, nil
}

func (m model) running() bool {
	return m.status != nil && len(m.status.Streams) > 0
}

// --- view ---

func (m model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("AirTone") + "  " + dimStyle.Render("system audio → your phone, in sync") + "\n\n")

	switch {
	case !m.ready:
		b.WriteString(m.renderDoctor())
	case m.running():
		b.WriteString(m.renderLive())
	default:
		b.WriteString(boxStyle.Render("Ready. Press "+keyStyle.Render("s")+" to start streaming.") + "\n")
	}

	if m.busy != "" {
		b.WriteString("\n" + dimStyle.Render(m.busy) + "\n")
	}
	if m.note != "" {
		b.WriteString("\n" + badStyle.Render(m.note) + "\n")
	}
	b.WriteString("\n" + m.footer())
	return b.String()
}

func (m model) renderDoctor() string {
	var lines []string
	lines = append(lines, "Setup check (press "+keyStyle.Render("r")+" to recheck, run "+keyStyle.Render("airtone setup")+" to fix):")
	for _, c := range m.checks {
		if c.OK {
			lines = append(lines, okStyle.Render("  ✓ ")+c.Name)
		} else {
			detail := ""
			if c.Detail != "" {
				detail = dimStyle.Render(" — " + c.Detail)
			}
			lines = append(lines, badStyle.Render("  ✗ ")+c.Name+detail)
		}
	}
	return boxStyle.Render(strings.Join(lines, "\n"))
}

func (m model) renderLive() string {
	streamState := idleStyle.Render("idle (play something on the Mac)")
	for _, s := range m.status.Streams {
		if s.Status == "playing" {
			streamState = liveStyle.Render("● LIVE")
		}
	}

	var listeners []string
	for _, c := range m.status.Clients {
		if c.Connected {
			listeners = append(listeners, fmt.Sprintf("  • %s (%d%%)", c.Name, c.Percent))
		}
	}
	listenerBlock := dimStyle.Render("  no listeners yet — scan the QR")
	if len(listeners) > 0 {
		listenerBlock = strings.Join(listeners, "\n")
	}

	info := strings.Join([]string{
		"Stream:    " + streamState,
		"Listeners:",
		listenerBlock,
		"",
		"Open on your phone:",
		keyStyle.Render("  " + m.url),
	}, "\n")

	return lipgloss.JoinHorizontal(lipgloss.Top, boxStyle.Render(info), "  ", qrCode(m.url))
}

func (m model) footer() string {
	keys := []string{}
	if m.ready && !m.running() {
		keys = append(keys, keyStyle.Render("s")+" start")
	}
	if m.running() {
		keys = append(keys, keyStyle.Render("x")+" stop")
	}
	keys = append(keys, keyStyle.Render("r")+" recheck", keyStyle.Render("q")+" quit")
	hint := ""
	if m.running() {
		hint = dimStyle.Render("  (quitting keeps streaming)")
	}
	return dimStyle.Render(strings.Join(keys, "   ")) + hint
}

func qrCode(s string) string {
	var b bytes.Buffer
	qrterminal.GenerateWithConfig(s, qrterminal.Config{
		Level:     qrterminal.L,
		Writer:    &b,
		BlackChar: qrterminal.BLACK,
		WhiteChar: qrterminal.WHITE,
		QuietZone: 1,
	})
	return b.String()
}
