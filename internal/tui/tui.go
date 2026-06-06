// Package tui is the interactive AirTone front-end (Bubble Tea). It orchestrates
// the two modes and shows live status; it never touches the audio path itself.
//
//   - Party   : multi-device synced playback via snapcast (buffered, ~1s).
//   - Instant : low-latency single/loose playback via WebRTC (~tens of ms).
package tui

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mdp/qrterminal/v3"

	"github.com/sait-turanalp/airtone/internal/doctor"
	"github.com/sait-turanalp/airtone/internal/engine"
	"github.com/sait-turanalp/airtone/internal/instant"
	"github.com/sait-turanalp/airtone/internal/rpc"
)

const partyPort = "1780"

type mode int

const (
	modeParty mode = iota
	modeInstant
)

func (m mode) String() string {
	if m == modeInstant {
		return "Instant (low-latency)"
	}
	return "Party (synced)"
}

type screen int

const (
	screenHome screen = iota
	screenSettings
)

type bufferPreset struct {
	label string
	ms    int
	hint  string
}

var presets = []bufferPreset{
	{"Low latency", 500, "snappier, needs a solid network"},
	{"Balanced", 1000, "good default"},
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
	mode     mode
	width    int
	checks   []doctor.Check
	ready    bool
	status   *rpc.Status
	bufferMS int

	instantOn     bool
	instantCancel context.CancelFunc
	instantPrev   string

	busy string
	note string
}

// Run launches the interactive TUI.
func Run() error {
	m := model{bufferMS: currentBuffer()}
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func currentBuffer() int {
	if v, err := strconv.Atoi(os.Getenv("AIRTONE_BUFFER")); err == nil && v > 0 {
		return v
	}
	return 1000
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
			m.stopInstant() // leave audio output as we found it
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
	case "m":
		if m.running() {
			m.note = "stop first (x)"
			break
		}
		if m.mode == modeParty {
			m.mode = modeInstant
		} else {
			m.mode = modeParty
		}
		m.note = ""
	case "s":
		return m.start()
	case "x":
		return m.stop()
	case "g":
		if m.mode == modeParty && !m.ready && m.busy == "" {
			m.busy, m.note = "Running setup…", ""
			return m, runSetup
		}
	case "c":
		if m.mode == modeParty {
			m.screen = screenSettings
		} else {
			m.note = "latency presets apply to Party mode"
		}
	case "r":
		return m, checkDoctor
	}
	return m, nil
}

func (m model) start() (tea.Model, tea.Cmd) {
	if m.busy != "" || m.running() {
		return m, nil
	}
	if m.mode == modeInstant {
		if !engine.DeviceExists(engine.SyncDevice) {
			m.note = "run setup first (g)"
			return m, nil
		}
		if _, err := exec.LookPath("ffmpeg"); err != nil {
			m.note = "instant needs ffmpeg (brew install ffmpeg)"
			return m, nil
		}
		m.instantPrev = engine.CurrentOutput()
		_ = engine.SetOutput(engine.SyncDevice)
		ctx, cancel := context.WithCancel(context.Background())
		m.instantCancel = cancel
		go func() { _ = instant.Run(ctx, instant.Port) }()
		m.instantOn, m.note = true, ""
		return m, nil
	}
	// party
	if m.ready {
		m.busy, m.note = "Starting…", ""
		return m, startEngine
	}
	return m, nil
}

func (m model) stop() (tea.Model, tea.Cmd) {
	if m.mode == modeInstant {
		m.stopInstant()
		return m, nil
	}
	if m.running() && m.busy == "" {
		m.busy, m.note = "Stopping…", ""
		return m, stopEngine
	}
	return m, nil
}

func (m *model) stopInstant() {
	if !m.instantOn {
		return
	}
	if m.instantCancel != nil {
		m.instantCancel()
		m.instantCancel = nil
	}
	target := m.instantPrev
	if target == "" || target == engine.SyncDevice {
		target = engine.BuiltinOutput()
	}
	if target != "" {
		_ = engine.SetOutput(target)
	}
	m.instantOn = false
}

func (m model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "c":
		m.screen = screenHome
	case "1", "2", "3", "4", "5":
		i := int(msg.String()[0] - '1')
		if i < 0 || i >= len(presets) {
			return m, nil
		}
		m.bufferMS = presets[i].ms
		os.Setenv("AIRTONE_BUFFER", strconv.Itoa(m.bufferMS))
		if m.mode == modeParty && m.running() {
			m.busy = "Applying…"
			return m, restartEngine
		}
	}
	return m, nil
}

// running reports whether the currently-selected mode is active.
func (m model) running() bool {
	if m.mode == modeInstant {
		return m.instantOn
	}
	return m.status != nil && len(m.status.Streams) > 0
}

func (m model) url() string {
	port := partyPort
	if m.mode == modeInstant {
		port = strconv.Itoa(instant.Port)
	}
	return "http://" + engine.LANIP() + ":" + port
}

// --- view ---

func (m model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("AirTone") + "  " + dimStyle.Render(m.mode.String()+"  ·  m to switch") + "\n\n")

	if m.screen == screenSettings {
		b.WriteString(m.renderSettings())
		b.WriteString("\n\n" + dimStyle.Render(keyStyle.Render(fmt.Sprintf("1-%d", len(presets)))+" pick profile   "+keyStyle.Render("esc")+" back   "+keyStyle.Render("q")+" quit"))
		return b.String()
	}

	switch {
	case m.mode == modeInstant:
		if m.instantOn {
			b.WriteString(m.renderInstant())
		} else {
			b.WriteString(boxStyle.Render("Instant mode (low-latency). Press " + keyStyle.Render("s") + " to start."))
		}
	case !m.ready:
		b.WriteString(m.renderDoctor())
	case m.running():
		b.WriteString(m.renderLive())
	default:
		b.WriteString(boxStyle.Render("Party mode (synced). Press " + keyStyle.Render("s") + " to start."))
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
		keyStyle.Render("  " + m.url()),
	}, "\n")
	return lipgloss.JoinHorizontal(lipgloss.Top, boxStyle.Render(info), "  ", qrCode(m.url()))
}

func (m model) renderInstant() string {
	info := strings.Join([]string{
		"Status:    " + liveStyle.Render("● running"),
		fmt.Sprintf("Listeners: %d", instant.Listeners()),
		"",
		dimStyle.Render("low latency · no cross-device sync"),
		"",
		"Open on your phone:",
		keyStyle.Render("  " + m.url()),
	}, "\n")
	return lipgloss.JoinHorizontal(lipgloss.Top, boxStyle.Render(info), "  ", qrCode(m.url()))
}

func (m model) renderSettings() string {
	lines := []string{"Latency profile (Party buffer):", ""}
	for i, p := range presets {
		marker, label := "  ", p.label
		if p.ms == m.bufferMS {
			marker, label = okStyle.Render("▸ "), okStyle.Render(p.label)
		}
		lines = append(lines, fmt.Sprintf("%s%s %-13s %s", marker, keyStyle.Render(strconv.Itoa(i+1)), label, dimStyle.Render(fmt.Sprintf("%dms — %s", p.ms, p.hint))))
	}
	lines = append(lines, "", dimStyle.Render("Applies live while streaming (brief reconnect)."))
	return boxStyle.Render(strings.Join(lines, "\n"))
}

func (m model) footer() string {
	var keys []string
	switch {
	case m.mode == modeParty && !m.ready:
		keys = append(keys, keyStyle.Render("g")+" run setup")
	case !m.running():
		keys = append(keys, keyStyle.Render("s")+" start")
	default:
		keys = append(keys, keyStyle.Render("x")+" stop")
	}
	keys = append(keys, keyStyle.Render("m")+" mode")
	if m.mode == modeParty {
		keys = append(keys, keyStyle.Render("c")+" settings")
	}
	keys = append(keys, keyStyle.Render("r")+" recheck", keyStyle.Render("q")+" quit")
	hint := ""
	if m.mode == modeParty && m.running() {
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
