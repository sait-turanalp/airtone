// Package tui is the interactive AirTone front-end (Bubble Tea). It orchestrates
// the two modes and shows live status; it never touches the audio path itself.
//
//   - Party   : multi-device synced playback via snapcast (buffered, ~1s).
//   - Instant : low-latency single/loose playback via WebRTC (~tens of ms).
//
// The view is theme-driven (styles.go), responsive (sizes from WindowSizeMsg),
// and keeps a single key map (keys.go) shared by the footer and dispatch.
package tui

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mdp/qrterminal/v3"

	"github.com/sait-turanalp/airtone/internal/doctor"
	"github.com/sait-turanalp/airtone/internal/engine"
	"github.com/sait-turanalp/airtone/internal/instant"
	"github.com/sait-turanalp/airtone/internal/rpc"
)

const partyPort = "1780"

// Layout floors. Below these the UI can't render usefully, so we show a gate
// instead of a broken/overflowing screen.
const (
	minWidth  = 50
	minHeight = 16
)

type mode int

const (
	modeParty mode = iota
	modeInstant
)

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
type doctorMsg struct {
	checks   []doctor.Check
	ffmpegOK bool
}
type startedMsg struct{ err error }
type stoppedMsg struct{ err error }
type setupDoneMsg struct{ err error }

// --- model ---

type model struct {
	width, height int

	styles Styles
	keys   keyMap
	help   help.Model
	spin   spinner.Model

	screen screen
	mode   mode

	checks   []doctor.Check
	ready    bool // all Party checks pass
	syncOK   bool // AirTone Sync device present (needed by both modes)
	ffmpegOK bool // ffmpeg present (needed by Instant)
	status   *rpc.Status
	bufferMS int

	instantOn     bool
	instantCancel context.CancelFunc
	instantPrev   string

	busy string // async op label; spinner shows while set
	note string // inline message; cleared on next interaction
}

// AirTone switches the system output to its Multi-Output device while running.
// On *any* exit — q, Ctrl+C, or a closed terminal (SIGHUP) — we put the output
// back. fireRestore is unconditional and idempotent: if we're still on AirTone's
// device, return to the remembered previous one (or the built-in speakers).
var (
	restoreMu sync.Mutex
	restoreTo string
)

func rememberPrev(prev string) {
	if prev == "" || prev == engine.SyncDevice {
		return
	}
	restoreMu.Lock()
	restoreTo = prev
	restoreMu.Unlock()
}

func fireRestore() {
	if engine.CurrentOutput() != engine.SyncDevice {
		return
	}
	restoreMu.Lock()
	t := restoreTo
	restoreMu.Unlock()
	if t == "" {
		t = engine.BuiltinOutput()
	}
	if t != "" {
		_ = engine.SetOutput(t)
	}
}

// Run launches the interactive TUI. Default mode is Instant (low latency).
func Run() error {
	m := model{
		mode:     modeInstant,
		bufferMS: currentBuffer(),
		styles:   newStyles(),
		keys:     newKeyMap(),
		help:     help.New(),
	}
	m.help.Width = 80
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	sp.Style = m.styles.Key
	m.spin = sp

	// Closing the terminal sends SIGHUP, which skips the normal quit path — restore
	// the audio output ourselves on the way out (only if Instant armed it).
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP)
	go func() { <-sig; fireRestore(); os.Exit(0) }()

	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	fireRestore() // safety net for Ctrl+C / normal quit
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

func checkDoctor() tea.Msg {
	checks := doctor.Run()
	_, err := exec.LookPath("ffmpeg")
	return doctorMsg{checks: checks, ffmpegOK: err == nil}
}

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
		m.width, m.height = msg.Width, msg.Height
		m.help.Width = msg.Width
	case tea.KeyMsg:
		if key.Matches(msg, m.keys.Quit) {
			m.stopInstant()
			if m.mode == modeParty {
				var b bytes.Buffer
				_ = engine.Stop(&b) // stop snapserver + restore the output
			}
			fireRestore() // return to the previous device on every quit
			return m, tea.Quit
		}
		m.note = "" // clear stale message on any interaction
		if key.Matches(msg, m.keys.Help) {
			m.help.ShowAll = !m.help.ShowAll
			return m, nil
		}
		if m.screen == screenSettings {
			return m.updateSettings(msg)
		}
		return m.updateHome(msg)
	case spinner.TickMsg:
		if m.busy != "" {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
		return m, nil
	case tickMsg:
		return m, tea.Batch(pollStatus, tick())
	case statusMsg:
		if msg.err != nil {
			m.status = nil
		} else {
			m.status = msg.st
		}
	case doctorMsg:
		m.checks = msg.checks
		m.ready = doctor.OK(msg.checks)
		m.ffmpegOK = msg.ffmpegOK
		m.syncOK = false
		for _, c := range msg.checks {
			if strings.Contains(c.Name, "AirTone Sync") {
				m.syncOK = c.OK
			}
		}
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
	k := m.keys
	switch {
	case key.Matches(msg, k.Mode):
		if m.running() {
			m.note = "stop first (x) before switching mode"
			break
		}
		if m.mode == modeParty {
			m.mode = modeInstant
		} else {
			m.mode = modeParty
		}
	case key.Matches(msg, k.Start):
		return m.start()
	case key.Matches(msg, k.Stop):
		return m.stop()
	case key.Matches(msg, k.Setup):
		if m.mode == modeParty && !m.ready && m.busy == "" {
			m.busy = "Running setup…"
			return m, tea.Batch(runSetup, m.spin.Tick)
		}
	case key.Matches(msg, k.Settings):
		if m.mode == modeParty {
			m.screen = screenSettings
		} else {
			m.note = "latency presets apply to Party mode"
		}
	case key.Matches(msg, k.Recheck):
		return m, checkDoctor
	}
	return m, nil
}

func (m model) start() (tea.Model, tea.Cmd) {
	if m.busy != "" || m.running() {
		return m, nil
	}
	if m.mode == modeInstant {
		if !m.syncOK {
			m.note = "run setup first (g)"
			return m, nil
		}
		if !m.ffmpegOK {
			m.note = "instant needs ffmpeg — brew install ffmpeg"
			return m, nil
		}
		m.instantPrev = engine.CurrentOutput()
		_ = engine.SetOutput(engine.SyncDevice)
		rememberPrev(m.instantPrev) // restore on any exit, incl. a closed terminal
		ctx, cancel := context.WithCancel(context.Background())
		m.instantCancel = cancel
		go func() { _ = instant.Run(ctx, instant.Port) }()
		m.instantOn = true
		return m, nil
	}
	// party
	if !m.ready {
		m.note = "run setup first (g)"
		return m, nil
	}
	rememberPrev(engine.CurrentOutput()) // so quit / close returns to this device
	m.busy = "Starting…"
	return m, tea.Batch(startEngine, m.spin.Tick)
}

func (m model) stop() (tea.Model, tea.Cmd) {
	if m.mode == modeInstant {
		m.stopInstant()
		return m, nil
	}
	if m.running() && m.busy == "" {
		m.busy = "Stopping…"
		return m, tea.Batch(stopEngine, m.spin.Tick)
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
	k := m.keys
	switch {
	case key.Matches(msg, k.Back), key.Matches(msg, k.Settings):
		m.screen = screenHome
	case key.Matches(msg, k.Preset):
		i := int(msg.String()[0] - '1')
		if i < 0 || i >= len(presets) {
			return m, nil
		}
		m.bufferMS = presets[i].ms
		os.Setenv("AIRTONE_BUFFER", strconv.Itoa(m.bufferMS))
		if m.mode == modeParty && m.running() {
			m.busy = "Applying…"
			return m, tea.Batch(restartEngine, m.spin.Tick)
		}
	}
	return m, nil
}

// --- state helpers ---

func (m model) running() bool {
	if m.mode == modeInstant {
		return m.instantOn
	}
	return m.status != nil && len(m.status.Streams) > 0
}

func (m model) streamPlaying() bool {
	if m.status == nil {
		return false
	}
	for _, s := range m.status.Streams {
		if s.Status == "playing" {
			return true
		}
	}
	return false
}

func (m model) connectedClients() []rpc.Client {
	if m.status == nil {
		return nil
	}
	var out []rpc.Client
	for _, c := range m.status.Clients {
		if c.Connected {
			out = append(out, c)
		}
	}
	return out
}

func (m model) url() string {
	port := partyPort
	if m.mode == modeInstant {
		port = strconv.Itoa(instant.Port)
	}
	return "http://" + engine.LANIP() + ":" + port
}

// layoutWidth is the usable width, leaving a small margin so centered panels
// never touch the terminal edges.
func (m model) layoutWidth() int {
	if m.width <= 0 {
		return 80
	}
	return m.width - 4
}

// --- view ---

func (m model) View() string {
	st := m.styles
	if m.width > 0 && (m.width < minWidth || m.height < minHeight) {
		return st.Gate.Render("AirTone\n\nTerminal too small.\nResize to at least 60×20.")
	}

	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n\n")
	b.WriteString(m.body())
	if m.busy != "" {
		b.WriteString("\n\n" + m.spin.View() + " " + st.Dim.Render(m.busy))
	}
	if m.note != "" {
		b.WriteString("\n\n" + st.ErrText.Render(m.note))
	}
	b.WriteString("\n\n" + m.help.View(m.activeHelp()))
	content := b.String()
	// Center the whole UI in the terminal so it stays balanced at any size
	// instead of clinging to the top-left corner.
	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
	}
	return st.App.Render(content)
}

func (m model) header() string {
	st := m.styles
	parts := []string{
		st.Title.Render("AirTone"),
		st.ModeBadge.Render(m.modeLabel()),
		m.stateBadge(),
	}
	return parts[0] + "  " + parts[1] + st.Dim.Render(" · ") + parts[2]
}

func (m model) modeLabel() string {
	if m.mode == modeInstant {
		return "◈ Instant"
	}
	return "◈ Party"
}

func (m model) stateBadge() string {
	st := m.styles
	if !m.running() {
		return st.Idle.Render("○ idle")
	}
	if m.mode == modeParty && !m.streamPlaying() {
		return st.Idle.Render("◐ buffering")
	}
	return st.Live.Render("● LIVE")
}

func (m model) body() string {
	switch {
	case m.screen == screenSettings:
		return m.renderSettings()
	case m.mode == modeInstant:
		if m.instantOn {
			return m.renderInstantLive()
		}
		return m.renderInstantIdle()
	case !m.ready:
		return m.renderDoctor()
	case m.running():
		return m.renderLive()
	default:
		return m.renderPartyIdle()
	}
}

// dashboard lays out a status panel beside the QR/URL card, collapsing
// responsively: side-by-side → stacked → URL-only (QR dropped) as width shrinks.
func (m model) dashboard(title string, lines []string) string {
	st := m.styles
	url := m.url()
	qrCard := st.Panel.Render(strings.Join([]string{
		st.PanelTitle.Render("Scan to join"),
		"",
		st.QR.Render(qrCode(url)),
		"",
		st.URL.Render(url),
	}, "\n"))
	qw := lipgloss.Width(qrCard)
	avail := m.layoutWidth()

	panel := st.Panel.Render(st.PanelTitle.Render(title) + "\n\n" + strings.Join(lines, "\n"))
	pw := lipgloss.Width(panel)

	const gap = "    "
	switch {
	case avail >= pw+qw+len(gap):
		return lipgloss.JoinHorizontal(lipgloss.Center, panel, gap, qrCard)
	case avail >= qw:
		return lipgloss.JoinVertical(lipgloss.Left, panel, "", qrCard)
	default:
		// Too narrow for the QR: fold the URL into the status panel.
		lines = append(lines, "", st.Dim.Render("Open on your phone:"), st.URL.Render(url))
		return st.Panel.Render(st.PanelTitle.Render(title) + "\n\n" + strings.Join(lines, "\n"))
	}
}

func (m model) renderLive() string {
	st := m.styles
	state := st.Idle.Render("idle — play something on the Mac")
	if m.streamPlaying() {
		state = st.Live.Render("● LIVE")
	}
	lines := []string{
		"Stream    " + state,
		"Buffer    " + fmt.Sprintf("%d ms", m.bufferMS),
	}
	conn := m.connectedClients()
	lines = append(lines, "", fmt.Sprintf("Listeners (%d)", len(conn)))
	if len(conn) == 0 {
		lines = append(lines, st.Dim.Render("  none yet — scan the QR"))
	} else {
		for _, c := range conn {
			muted := ""
			if c.Muted {
				muted = st.Dim.Render(" muted")
			}
			lines = append(lines, fmt.Sprintf("  • %-12s %3d%%", c.Name, c.Percent)+muted)
		}
	}
	return m.dashboard("Session", lines)
}

func (m model) renderInstantLive() string {
	st := m.styles
	lines := []string{
		"Status     " + st.Live.Render("● running"),
		fmt.Sprintf("Listeners  %d", instant.Listeners()),
		"",
		st.Dim.Render("low latency · no cross-device sync"),
	}
	return m.dashboard("Session", lines)
}

func (m model) renderInstantIdle() string {
	st := m.styles
	if m.syncOK && m.ffmpegOK {
		return st.Panel.Render(st.PanelTitle.Render("Instant mode") + "\n\n" +
			st.Dim.Render("Low latency · single/loose listener.") + "\n\n" +
			"Press " + st.Key.Render("s") + " to start.")
	}
	lines := []string{st.PanelTitle.Render("Instant mode — setup needed"), ""}
	lines = append(lines, m.checkLine("AirTone Sync device", m.syncOK, "run setup (g)"))
	lines = append(lines, m.checkLine("ffmpeg", m.ffmpegOK, "brew install ffmpeg"))
	return st.Panel.Render(strings.Join(lines, "\n"))
}

func (m model) renderDoctor() string {
	st := m.styles
	lines := []string{st.PanelTitle.Render("Setup check (Party mode)"), ""}
	for _, c := range m.checks {
		if c.OK {
			lines = append(lines, st.CheckOK.Render("  ✓ ")+c.Name)
		} else {
			detail := ""
			if c.Detail != "" {
				detail = st.Dim.Render(" — " + c.Detail)
			}
			lines = append(lines, st.CheckFail.Render("  ✗ ")+c.Name+detail)
		}
	}
	lines = append(lines, "", st.Dim.Render("Press ")+st.Key.Render("g")+st.Dim.Render(" to set up, ")+st.Key.Render("r")+st.Dim.Render(" to recheck."))
	return st.Panel.Render(strings.Join(lines, "\n"))
}

func (m model) renderPartyIdle() string {
	st := m.styles
	return st.Panel.Render(st.PanelTitle.Render("Party mode") + "\n\n" +
		st.Dim.Render(fmt.Sprintf("Multi-device synced playback (~%dms).", m.bufferMS)) + "\n\n" +
		"Press " + st.Key.Render("s") + " to start.")
}

func (m model) renderSettings() string {
	st := m.styles
	lines := []string{st.PanelTitle.Render("Latency profile (Party buffer)"), ""}
	for i, p := range presets {
		marker, label := "  ", p.label
		if p.ms == m.bufferMS {
			marker, label = st.CheckOK.Render("▸ "), st.Live.Render(p.label)
		}
		lines = append(lines, fmt.Sprintf("%s%s  %-12s %s", marker, st.Key.Render(strconv.Itoa(i+1)), label, st.Dim.Render(fmt.Sprintf("%dms — %s", p.ms, p.hint))))
	}
	lines = append(lines, "", st.Dim.Render("Applies live while streaming (brief reconnect)."))
	return st.Panel.Render(strings.Join(lines, "\n"))
}

func (m model) checkLine(name string, ok bool, fix string) string {
	st := m.styles
	if ok {
		return st.CheckOK.Render("  ✓ ") + name
	}
	return st.CheckFail.Render("  ✗ ") + name + st.Dim.Render(" — "+fix)
}

// activeHelp returns the context-sensitive footer: only the keys that do
// something in the current mode/screen/state.
func (m model) activeHelp() helpMap {
	k := m.keys
	if m.screen == screenSettings {
		return helpMap{
			short: []key.Binding{k.Preset, k.Back, k.Quit},
			full:  [][]key.Binding{{k.Preset}, {k.Back, k.Quit}},
		}
	}
	var primary key.Binding
	switch {
	case m.mode == modeParty && !m.ready:
		primary = k.Setup
	case !m.running():
		primary = k.Start
	default:
		primary = k.Stop
	}
	short := []key.Binding{primary, k.Mode}
	if m.mode == modeParty {
		short = append(short, k.Settings)
	}
	short = append(short, k.Help, k.Quit)
	full := [][]key.Binding{
		{k.Start, k.Stop, k.Mode},
		{k.Settings, k.Setup, k.Recheck},
		{k.Help, k.Quit},
	}
	return helpMap{short: short, full: full}
}

// qrCode renders a compact half-block QR (two modules per character row, so
// roughly half the height of full blocks). The caller wraps it in styles.QR to
// force dark-on-white regardless of the terminal theme — best for phone cameras.
func qrCode(s string) string {
	var b bytes.Buffer
	qrterminal.GenerateWithConfig(s, qrterminal.Config{
		Level:          qrterminal.L,
		Writer:         &b,
		HalfBlocks:     true,
		BlackChar:      qrterminal.BLACK_BLACK,
		WhiteBlackChar: qrterminal.WHITE_BLACK,
		WhiteChar:      qrterminal.WHITE_WHITE,
		BlackWhiteChar: qrterminal.BLACK_WHITE,
		QuietZone:      2,
	})
	return evenQRBorders(strings.TrimRight(b.String(), "\n"))
}

// evenQRBorders fills the half-block quiet-zone row (which renders ragged —
// a stray dark strip — because the QR has an odd module height) with solid
// white. These are quiet-zone rows only (no data), so squaring them is safe
// and gives a clean, symmetric white frame top and bottom.
func evenQRBorders(s string) string {
	lines := strings.Split(s, "\n")
	solid := func(line string) string {
		half := false
		for _, r := range line {
			switch r {
			case '▀', '▄':
				half = true
			case '█':
			default:
				return line // contains data modules — not a pure border row
			}
		}
		if half {
			return strings.Repeat("█", len([]rune(line)))
		}
		return line
	}
	if len(lines) > 0 {
		lines[0] = solid(lines[0])
		lines[len(lines)-1] = solid(lines[len(lines)-1])
	}
	return strings.Join(lines, "\n")
}
