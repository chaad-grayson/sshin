package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/chaad-grayson/sshin/internal/secret"
	"github.com/chaad-grayson/sshin/internal/tunnel"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// tunnelTab is the state of the Tunnels tab of the list view
type tunnelTab struct {
	active  bool
	list    []tunnel.Tunnel
	table   table.Model
	form    *tunnelForm
	confirm string // tunnel that d deletes when pressed again
}

type tunnelStartedMsg struct {
	t   tunnel.Tunnel
	err error
}

// Tunnel form fields; the host field is a selector, not a text input
const (
	tfName = iota
	tfHost
	tfLocalPort
	tfRemoteHost
	tfRemotePort
	tfCount
)

type tunnelForm struct {
	editing string // name of the edited tunnel, "" for a new one
	inputs  [tfCount]textinput.Model
	hosts   []string
	host    int
	focused int
	err     string
}

// showTunnels switches the list view to the Tunnels tab
func (m *Model) showTunnels() {
	if m.tunnels.table.Columns() == nil {
		t := table.New(table.WithFocused(true))
		s := table.DefaultStyles()
		s.Header = s.Header.BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color(SecondaryColor)).BorderBottom(true).Bold(false)
		s.Selected = m.styles.Selected
		t.SetStyles(s)
		m.tunnels.table = t
	}
	m.tunnels.active = true
	m.reloadTunnels()
}

// reloadTunnels reads the saved tunnels and whether each one runs
func (m *Model) reloadTunnels() {
	list, err := tunnel.Load()
	if err != nil {
		m.errorMessage, m.showingError, m.messageOK = err.Error(), true, false
	}
	m.tunnels.list = list

	nameW, hostW := 6, 8
	rows := make([]table.Row, len(list))
	for i, t := range list {
		status, lastActive := "⚫ off", "never"
		if !t.LastActive.IsZero() {
			lastActive = formatTimeAgo(t.LastActive)
		}
		if t.Running() {
			status, lastActive = "🟢 on", "now"
		}
		nameW, hostW = max(nameW, len(t.Name)+2), max(hostW, len(t.Host)+2)
		rows[i] = table.Row{status, t.Name, t.Host, fmt.Sprintf("localhost:%d", t.LocalPort), fmt.Sprintf("%s:%d", t.RemoteHost, t.RemotePort), lastActive}
	}
	// 16 = cell padding (2 per column), table border and app padding
	remoteW := max(m.width-nameW-hostW-8-17-14-16, 20)
	m.tunnels.table.SetColumns([]table.Column{
		{Title: "Status", Width: 8},
		{Title: "Name", Width: nameW},
		{Title: "SSH Host", Width: hostW},
		{Title: "Local", Width: 17},
		{Title: "Remote", Width: remoteW},
		{Title: "Last Active", Width: 14},
	})
	m.tunnels.table.SetRows(rows)
	m.tunnels.table.SetHeight(min(len(rows), max(m.height-17, 3)) + 2)
	// The table sets the cursor to -1 while it has no row
	if c := m.tunnels.table.Cursor(); len(rows) > 0 && (c < 0 || c >= len(rows)) {
		m.tunnels.table.SetCursor(len(rows) - 1)
	}
}

func (m Model) selectedTunnel() (tunnel.Tunnel, bool) {
	i := m.tunnels.table.Cursor()
	if i < 0 || i >= len(m.tunnels.list) {
		return tunnel.Tunnel{}, false
	}
	return m.tunnels.list[i], true
}

func (m Model) handleTunnelKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key != "d" {
		m.tunnels.confirm = ""
	}
	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "t", "esc":
		m.tunnels.active = false
		return m, nil
	case "r":
		m.reloadTunnels()
		return m, nil
	case "a":
		m.openTunnelForm(nil)
		return m, textinput.Blink
	case "h":
		m.helpForm = NewHelpForm(m.styles, m.width, m.height)
		m.viewMode = ViewHelp
		return m, nil
	}

	t, ok := m.selectedTunnel()
	if !ok {
		return m, nil
	}
	switch key {
	case "enter", " ":
		if t.Running() {
			if err := t.Stop(); err != nil {
				return m, m.showMessage("could not stop "+t.Name+": "+err.Error(), false)
			}
			_ = tunnel.Touch(t.Name) // only feeds the Last Active column
			m.reloadTunnels()
			return m, m.showMessage("Tunnel "+t.Name+" stopped", true)
		}
		c, err := t.StartCmd(m.configFile)
		if err != nil {
			return m, m.showMessage(err.Error(), false)
		}
		c.Env = secret.SSHEnv(m.configFile, t.Host)
		return m, tea.ExecProcess(c, func(err error) tea.Msg { return tunnelStartedMsg{t, err} })
	case "e":
		if t.Running() {
			return m, m.showMessage("stop "+t.Name+" before editing it", false)
		}
		m.openTunnelForm(&t)
		return m, textinput.Blink
	case "d":
		if t.Running() {
			return m, m.showMessage("stop "+t.Name+" before deleting it", false)
		}
		if m.tunnels.confirm != t.Name {
			m.tunnels.confirm = t.Name
			return m, m.showMessage("press d again to delete "+t.Name, false)
		}
		m.tunnels.confirm = ""
		list := append([]tunnel.Tunnel{}, m.tunnels.list[:m.tunnels.table.Cursor()]...)
		list = append(list, m.tunnels.list[m.tunnels.table.Cursor()+1:]...)
		if err := tunnel.Save(list); err != nil {
			return m, m.showMessage(err.Error(), false)
		}
		m.reloadTunnels()
		return m, m.showMessage("Tunnel "+t.Name+" deleted", true)
	}

	var cmd tea.Cmd
	m.tunnels.table, cmd = m.tunnels.table.Update(msg)
	return m, cmd
}

// tunnelStarted reports how the start of a tunnel went
func (m Model) tunnelStarted(msg tunnelStartedMsg) (tea.Model, tea.Cmd) {
	running := msg.err == nil && msg.t.Running()
	if running {
		_ = tunnel.Touch(msg.t.Name) // only feeds the Last Active column
	}
	m.reloadTunnels()
	if !running {
		why := msg.t.LastError()
		if why == "" && msg.err != nil {
			why = msg.err.Error()
		}
		return m, m.showMessage("tunnel "+msg.t.Name+" failed: "+why, false)
	}
	return m, m.showMessage(fmt.Sprintf("Tunnel %s on: localhost:%d → %s:%d", msg.t.Name, msg.t.LocalPort, msg.t.RemoteHost, msg.t.RemotePort), true)
}

// openTunnelForm opens the form for t, or for a new tunnel when t is nil
func (m *Model) openTunnelForm(t *tunnel.Tunnel) {
	f := &tunnelForm{}
	for _, h := range m.allHosts {
		f.hosts = append(f.hosts, h.Name)
	}
	placeholders := [tfCount]string{"rds-prod", "", "3307", "mydb.xxxx.ap-southeast-1.rds.amazonaws.com", "3306"}
	for i := range f.inputs {
		f.inputs[i] = textinput.New()
		f.inputs[i].Placeholder = placeholders[i]
		f.inputs[i].Width = 50
	}
	host := ""
	if selected := m.table.SelectedRow(); len(selected) > 0 {
		host = extractHostNameFromTableRow(selected[0])
	}
	if t != nil {
		f.editing = t.Name
		host = t.Host
		f.inputs[tfName].SetValue(t.Name)
		f.inputs[tfLocalPort].SetValue(strconv.Itoa(t.LocalPort))
		f.inputs[tfRemoteHost].SetValue(t.RemoteHost)
		f.inputs[tfRemotePort].SetValue(strconv.Itoa(t.RemotePort))
	}
	for i, h := range f.hosts {
		if h == host {
			f.host = i
		}
	}
	f.inputs[tfName].Focus()
	m.tunnels.form = f
	m.viewMode = ViewTunnelForm
}

func (m Model) handleTunnelFormKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := m.tunnels.form
	switch msg.String() {
	case "esc", "ctrl+c":
		m.tunnels.form = nil
		m.viewMode = ViewList
		return m, nil
	case "tab", "down":
		f.focus((f.focused + 1) % tfCount)
		return m, nil
	case "shift+tab", "up":
		f.focus((f.focused + tfCount - 1) % tfCount)
		return m, nil
	case "left", "right":
		if f.focused == tfHost && len(f.hosts) > 0 {
			step := 1
			if msg.String() == "left" {
				step = len(f.hosts) - 1
			}
			f.host = (f.host + step) % len(f.hosts)
			return m, nil
		}
	case "enter", "ctrl+s":
		if msg.String() == "enter" && f.focused < tfCount-1 {
			f.focus(f.focused + 1)
			return m, nil
		}
		if err := m.saveTunnelForm(); err != nil {
			f.err = err.Error()
			return m, nil
		}
		m.tunnels.form = nil
		m.viewMode = ViewList
		m.reloadTunnels()
		return m, nil
	}
	if f.focused == tfHost {
		return m, nil
	}
	var cmd tea.Cmd
	f.inputs[f.focused], cmd = f.inputs[f.focused].Update(msg)
	return m, cmd
}

func (f *tunnelForm) focus(i int) {
	f.inputs[f.focused].Blur()
	f.focused = i
	f.inputs[i].Focus()
}

// saveTunnelForm validates the form and saves the tunnel
func (m Model) saveTunnelForm() error {
	f := m.tunnels.form
	if len(f.hosts) == 0 {
		return fmt.Errorf("add an SSH host first: a tunnel goes through one")
	}
	t := tunnel.Tunnel{
		Name:       strings.TrimSpace(f.inputs[tfName].Value()),
		Host:       f.hosts[f.host],
		LocalPort:  tunnel.ParsePort(f.inputs[tfLocalPort].Value()),
		RemoteHost: strings.TrimSpace(f.inputs[tfRemoteHost].Value()),
		RemotePort: tunnel.ParsePort(f.inputs[tfRemotePort].Value()),
	}
	if err := t.Validate(); err != nil {
		return err
	}
	list, err := tunnel.Load()
	if err != nil {
		return err
	}
	replaced := false
	for i, other := range list {
		switch {
		case other.Name == f.editing && f.editing != "":
			t.LastActive = other.LastActive
			list[i], replaced = t, true
		case other.Name == t.Name:
			return fmt.Errorf("a tunnel named %s already exists", t.Name)
		}
	}
	if !replaced {
		list = append(list, t)
	}
	return tunnel.Save(list)
}

func (f *tunnelForm) view(s Styles) string {
	title := "Add Tunnel"
	if f.editing != "" {
		title = "Edit Tunnel"
	}
	labels := [tfCount]string{"Name *", "SSH Host * (←/→ to choose)", "Local Port *", "Remote Host * (reachable from the SSH host)", "Remote Port *"}
	var b strings.Builder
	b.WriteString(s.FormTitle.Render(title) + "\n\n")
	for i, label := range labels {
		style := s.FormField
		if i == f.focused {
			style = s.FocusedLabel
		}
		b.WriteString(style.Render(label) + "\n")
		if i == tfHost {
			host := "(no SSH host: add one first)"
			if len(f.hosts) > 0 {
				host = "‹ " + f.hosts[f.host] + " ›"
			}
			b.WriteString(style.Render("> "+host) + "\n\n")
			continue
		}
		b.WriteString(f.inputs[i].View() + "\n\n")
	}
	if f.err != "" {
		b.WriteString(s.ErrorText.Render("Error: "+f.err) + "\n\n")
	}
	b.WriteString(s.FormHelp.Render("Tab/↑↓: move • Enter: next/save • Ctrl+S: save • Esc: cancel"))
	return s.FormContainer.Render(b.String())
}

// renderTabs renders the SSH / Tunnels switch of the list view
func (m Model) renderTabs() string {
	ssh, tunnels := m.styles.HelpText.Render(" SSH "), m.styles.FormTitle.Render(" Tunnels ")
	if !m.tunnels.active {
		ssh, tunnels = m.styles.FormTitle.Render(" SSH "), m.styles.HelpText.Render(" Tunnels ")
	}
	return " " + ssh + " " + tunnels + m.styles.HelpText.Render("   t: switch")
}

func (m Model) renderTunnelTab() []string {
	if len(m.tunnels.list) == 0 {
		return []string{
			m.styles.HelpText.Render("\n  No tunnel yet. Press a to forward a local port through one of your SSH hosts,\n  for example to a database that only that host can reach.\n"),
			m.styles.HelpText.Render(" a: add • t: SSH hosts • q: quit"),
		}
	}
	return []string{
		m.styles.TableFocused.Render(m.tunnels.table.View()),
		m.styles.HelpText.Render(" ↑/↓: navigate • Enter/Space: on/off • a: add • e: edit • d: delete • r: refresh • t: SSH hosts • q: quit"),
	}
}
