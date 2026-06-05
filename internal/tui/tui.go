// Package tui is the interactive AirTone front-end (Bubble Tea). It orchestrates
// the engine and shows live status read from the Snapcast control API. It does
// not touch the audio path itself.
package tui

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
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

type screen int

const (
	screenHome screen = iota
	screenSettings
)

// bufferPreset is a latency/smoothness profile (snapserver buffer in ms).
type bufferPreset struct {
	label string
	ms    int
	hint  string
}

var presets = []bufferPreset{
	{"Low latency", 500, "snappier, needs a solid network"},
	{"Balanced", 1500, "good default"},
	{"Smooth", 4000, "max stability, more delay"},
}

// --- messages ---

type tickMsg time.Time
type statusMsg struct {
	st  *rpc.Status
	err error
}
type doctorMsg []doctor.Check
type startedMsg struct{ err error }
type stoppedMsg struct{ err error }
type setupDoneMsg struct{ err error }

// --- model ---

type model struct {
	screen   screen
	width    int
	checks   []doctor.Check
	ready    bool
	status   *rpc.Status
	url      string
	bufferMS int
	busy     string // transient status line
	note     string // transient message (e.g. an error)
}

// Run launches the interactive TUI.
func Run() error {
	m := model{
		url:      "http://" + engine.LANIP() + ":" + httpPort,
		bufferMS: currentBuffer(),
	}
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func currentBuffer() int {
	if v, err := strconv.Atoi(os.Getenv("AIRTONE_BUFFER")); err == nil && v > 0 {
		return v
	}
	return 4000
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

func runSetup() tea.Msg {
	var b bytes.Buffer
	return setupDoneMsg{err: engine.Setup(&b)}
}

// restartEngine stops then starts so a new buffer value takes effect.
func restartEngine() tea.Msg {
	var b bytes.Buffer
	_ = engine.Stop(&b)
	return startedMsg{err: engine.Start(&b)}
}

// --- update ---

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.screen == screenSettings {
			return m.updateSettings(msg)
		}
		return m.updateHome(msg)
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
	case setupDoneMsg:
		m.busy = ""
		if msg.err != nil {
			m.note = "setup failed: " + msg.err.Error()
		}
		return m, checkDoctor
	}
	return m, nil
}

func (m model) updateHome(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
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
	case "g":
		if !m.ready && m.busy == "" {
			m.busy, m.note = "Running setup…", ""
			return m, runSetup
		}
	case "c":
		m.screen = screenSettings
	case "r":
		return m, checkDoctor
	}
	return m, nil
}

func (m model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "c":
		m.screen = screenHome
	case "1", "2", "3":
		i := int(msg.String()[0] - '1')
		m.bufferMS = presets[i].ms
		os.Setenv("AIRTONE_BUFFER", strconv.Itoa(m.bufferMS))
		if m.running() {
			m.busy = "Applying…"
			return m, restartEngine
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

	if m.screen == screenSettings {
		b.WriteString(m.renderSettings())
		b.WriteString("\n\n" + dimStyle.Render(keyStyle.Render("1/2/3")+" pick profile   "+keyStyle.Render("esc")+" back   "+keyStyle.Render("q")+" quit"))
		return b.String()
	}

	switch {
	case !m.ready:
		b.WriteString(m.renderDoctor())
	case m.running():
		b.WriteString(m.renderLive())
	default:
		b.WriteString(boxStyle.Render("Ready. Press " + keyStyle.Render("s") + " to start streaming."))
	}

	if m.busy != "" {
		b.WriteString("\n" + dimStyle.Render(m.busy))
	}
	if m.note != "" {
		b.WriteString("\n" + badStyle.Render(m.note))
	}
	b.WriteString("\n\n" + m.footer())
	return b.String()
}

func (m model) renderDoctor() string {
	lines := []string{"Setup check:"}
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
		"Buffer:    " + fmt.Sprintf("%dms", m.bufferMS),
		"",
		"Open on your phone:",
		keyStyle.Render("  " + m.url),
	}, "\n")

	return lipgloss.JoinHorizontal(lipgloss.Top, boxStyle.Render(info), "  ", qrCode(m.url))
}

func (m model) renderSettings() string {
	lines := []string{"Latency profile (buffer):", ""}
	for i, p := range presets {
		marker := "  "
		label := p.label
		if p.ms == m.bufferMS {
			marker = okStyle.Render("▸ ")
			label = okStyle.Render(label)
		}
		lines = append(lines, fmt.Sprintf("%s%s %-13s %s", marker, keyStyle.Render(strconv.Itoa(i+1)), label, dimStyle.Render(fmt.Sprintf("%dms — %s", p.ms, p.hint))))
	}
	lines = append(lines, "", dimStyle.Render("Applies live while streaming (brief reconnect)."))
	return boxStyle.Render(strings.Join(lines, "\n"))
}

func (m model) footer() string {
	var keys []string
	switch {
	case !m.ready:
		keys = append(keys, keyStyle.Render("g")+" run setup")
	case !m.running():
		keys = append(keys, keyStyle.Render("s")+" start")
	default:
		keys = append(keys, keyStyle.Render("x")+" stop")
	}
	keys = append(keys, keyStyle.Render("c")+" settings", keyStyle.Render("r")+" recheck", keyStyle.Render("q")+" quit")
	hint := ""
	if m.running() {
		hint = dimStyle.Render("   (quitting keeps streaming)")
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
