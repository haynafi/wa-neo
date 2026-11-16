package main

import (
	"fmt"
	"hash/fnv"
	"image/color"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

const (
	leftWidth = 34
	itemH     = 4
	avCols    = 6
	avRows    = 3
	headH     = 7
	rightX    = leftWidth + 3
)

var (
	ink     = lipgloss.Color("#111111")
	yellow  = lipgloss.Color("#FFE14D")
	pink    = lipgloss.Color("#FF7EB6")
	green   = lipgloss.Color("#5EEAD4")
	paper   = lipgloss.Color("#FFFDF5")
	blue    = lipgloss.Color("#7AA2FF")
	orange  = lipgloss.Color("#FFB86B")
	violet  = lipgloss.Color("#C4A8FF")
	edge    = lipgloss.AdaptiveColor{Light: "#111111", Dark: "#FFFDF5"}
	palette = []lipgloss.Color{yellow, blue, orange, violet}
)

type updateMsg struct{}
type sentMsg struct{ err error }
type escMsg struct{ at time.Time }
type pasteMsg struct {
	data []byte
	err  error
}

type model struct {
	app     *App
	chats   []Chat
	msgs    []Message
	cur     string
	input   textinput.Model
	w, h    int
	notice  string
	top     int
	scroll  int
	frame   *frame
	blurred bool
}

func newModel(app *App) model {
	in := textinput.New()
	in.Placeholder = "Ketik pesan, atau /new 628123456789"
	in.Prompt = ""
	in.Focus()
	m := model{app: app, input: in, frame: &frame{}}
	m.reload()
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.wait())
}

func (m model) wait() tea.Cmd {
	return func() tea.Msg {
		<-m.app.Updates
		return updateMsg{}
	}
}

func (m *model) reload() {
	if m.cur != "" && !m.blurred {
		m.app.MarkRead(m.cur)
	}
	m.chats = m.app.Chats()
	for i := range m.chats {
		m.chats[i].Name = clean(m.chats[i].Name)
		m.chats[i].Preview = clean(m.chats[i].Preview)
	}
	m.loadMsgs()
}

func (m *model) loadMsgs() {
	m.msgs = m.app.Messages(m.cur, 300)
	for i := range m.msgs {
		m.msgs[i].Text = clean(m.msgs[i].Text)
	}
}

func (m *model) choose(jid string) {
	m.cur = jid
	m.scroll = 0
	m.app.SetView(jid, !m.blurred)
	if jid != "" {
		m.app.MarkRead(jid)
	}
	m.loadMsgs()
	m.chats = m.app.Chats()
}

func (m model) unreadTotal() int {
	total := 0
	for _, c := range m.chats {
		total += c.Unread
	}
	return total
}

func (m model) rows() int {
	return max(1, (m.h-3-3)/itemH)
}

func (m *model) keepVisible() {
	rows, sel := m.rows(), m.selected()
	if sel >= 0 && sel < m.top {
		m.top = sel
	}
	if sel >= m.top+rows {
		m.top = sel - rows + 1
	}
	m.top = max(0, min(m.top, len(m.chats)-rows))
}

func (m *model) move(step int) {
	idx := m.selected() + step
	if idx < 0 || idx >= len(m.chats) {
		return
	}
	m.choose(m.chats[idx].JID)
	m.keepVisible()
}

func (m model) selected() int {
	for i, c := range m.chats {
		if c.JID == m.cur {
			return i
		}
	}
	return -1
}

func (m model) submit() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(m.input.Value())
	m.input.Reset()
	if text == "" {
		return m, nil
	}
	if number, ok := strings.CutPrefix(text, "/new "); ok {
		jid := m.app.Open(number)
		if jid == "" {
			m.notice = "nomor tidak valid"
			return m, nil
		}
		m.notice = ""
		m.reload()
		m.choose(jid)
		m.keepVisible()
		return m, nil
	}
	if m.cur == "" {
		m.notice = "belum ada chat, mulai dengan /new 628123456789"
		return m, nil
	}
	chat := m.cur
	if arg, ok := strings.CutPrefix(text, "/img "); ok {
		path, caption, found := splitImageArg(arg)
		if !found {
			m.notice = "file gambar tidak ditemukan"
			return m, nil
		}
		return m.sendFile(chat, path, caption)
	}
	if path, caption, ok := findImage(text); ok {
		return m.sendFile(chat, path, caption)
	}
	m.notice = ""
	return m, func() tea.Msg { return sentMsg{err: m.app.Send(chat, text)} }
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.keepVisible()
	case tea.MouseMsg:
		return m.mouse(msg)
	case updateMsg:
		m.reload()
		return m, m.wait()
	case sentMsg:
		m.notice = ""
		if msg.err != nil {
			m.notice = msg.err.Error()
		}
	case pasteMsg:
		return m.pasted(msg)
	case tea.BlurMsg:
		m.blurred = true
		m.app.SetView(m.cur, false)
	case tea.FocusMsg:
		m.blurred = false
		m.app.SetView(m.cur, true)
		m.reload()
	case escMsg:
		if !msg.at.Equal(m.frame.escAt) {
			return m, nil
		}
		if m.input.Value() != "" {
			m.input.Reset()
		} else {
			m.choose("")
		}
		return m, nil
	case tea.KeyMsg:
		if m.frame.leak(string(msg.Runes), msg.Type == tea.KeyEsc) {
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "up":
			m.move(-1)
			return m, nil
		case "down":
			m.move(1)
			return m, nil
		case "pgup":
			m.scroll += max(3, m.h/2)
			return m, nil
		case "pgdown":
			m.scroll = max(0, m.scroll-max(3, m.h/2))
			return m, nil
		case "esc":
			at := time.Now()
			m.frame.escAt = at
			return m, tea.Tick(60*time.Millisecond, func(time.Time) tea.Msg { return escMsg{at} })
		case "ctrl+v":
			return m, clipboardCmd
		case "enter":
			return m.submit()
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) mouse(msg tea.MouseMsg) (model, tea.Cmd) {
	inList := msg.X <= leftWidth+1
	maxTop := max(0, len(m.chats)-m.rows())
	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
		if time.Since(m.frame.lastWheel) < 50*time.Millisecond {
			return m, nil
		}
		m.frame.lastWheel = time.Now()
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if inList {
			m.top = max(0, m.top-1)
		} else {
			m.scroll += 3
		}
	case tea.MouseButtonWheelDown:
		if inList {
			m.top = min(maxTop, m.top+1)
		} else {
			m.scroll = max(0, m.scroll-3)
		}
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			break
		}
		if !inList {
			for _, h := range m.frame.hits {
				if msg.X >= h.col && msg.X < h.col+h.cols && msg.Y >= h.row && msg.Y < h.row+h.rows {
					return m, func() tea.Msg { return sentMsg{err: m.app.OpenMedia(h.chat, h.id)} }
				}
			}
			break
		}
		if idx := m.top + (msg.Y-5)/itemH; msg.Y >= 5 && idx < len(m.chats) {
			m.choose(m.chats[idx].JID)
		}
	}
	return m, nil
}

func clean(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\n':
			b.WriteRune(r)
		case r == '\t':
			b.WriteString("  ")
		case unicode.IsControl(r), r >= 0x1F3FB && r <= 0x1F3FF, r >= 0x1F1E6 && r <= 0x1F1FF:
		default:
			switch runewidth.RuneWidth(r) {
			case 0:
			case 2:
				if emojiOK && isEmoji(r) {
					b.WriteRune(r)
				} else {
					b.WriteString("□")
				}
			default:
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func (m model) View() string {
	if m.w < 60 || m.h < 16 {
		return "Perbesar jendela terminal."
	}
	width := m.w - 1
	m.frame.hits = m.frame.hits[:0]
	rightW := width - leftWidth - 4
	bodyH := m.h - 3

	title := " WA-NEO  ·  " + m.app.Status() + "  ·  ↑↓ pilih chat  ·  esc tutup chat  ·  ctrl+c keluar"
	if n := m.unreadTotal(); n > 0 {
		title = fmt.Sprintf(" WA-NEO  ·  %d belum dibaca  ·  ", n) + m.app.Status() + "  ·  esc tutup chat"
	}
	if m.notice != "" {
		title = " " + m.notice
	}
	bar := lipgloss.NewStyle().Background(yellow).Foreground(ink).Bold(true).Width(width).MaxWidth(width).Render(title)

	panel := lipgloss.NewStyle().Border(lipgloss.ThickBorder()).BorderForeground(edge)
	var overlays []overlay
	left := panel.Width(leftWidth).Height(bodyH).Render(m.renderChats(bodyH, &overlays))
	right := panel.Width(rightW).Height(bodyH).Render(m.renderChat(rightW, bodyH, &overlays))

	screen := lipgloss.JoinVertical(lipgloss.Left, bar, lipgloss.JoinHorizontal(lipgloss.Top, left, right))
	screen = lipgloss.NewStyle().MaxWidth(width).MaxHeight(m.h).Render(screen)
	return applyOverlays(screen, overlays)
}

func pick(key string) lipgloss.Color {
	h := fnv.New32a()
	h.Write([]byte(key))
	return palette[h.Sum32()%uint32(len(palette))]
}

func initial(name string) string {
	for _, r := range name {
		if (unicode.IsLetter(r) || unicode.IsDigit(r)) && runewidth.RuneWidth(r) == 1 {
			return strings.ToUpper(string(r))
		}
	}
	return "#"
}

func rgba(c lipgloss.Color) color.RGBA {
	var r, g, b uint8
	fmt.Sscanf(string(c), "#%02x%02x%02x", &r, &g, &b)
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

func (m model) avatar(name, jid string, cols, rows int, bg lipgloss.Color) []string {
	if photo := m.app.AvatarRows(jid, cols, rows, rgba(bg)); photo != nil {
		return photo
	}
	style := lipgloss.NewStyle().Background(pick(jid)).Foreground(ink).Bold(true)
	blank := style.Render(strings.Repeat(" ", cols))
	out := make([]string, rows)
	for i := range out {
		out[i] = blank
	}
	letter := initial(name)
	left := (cols - 1) / 2
	out[(rows-1)/2] = style.Render(strings.Repeat(" ", left) + letter + strings.Repeat(" ", cols-left-1))
	return out
}

func fit(s string, width int) string {
	return runewidth.FillRight(runewidth.Truncate(s, width, "…"), width)
}

func (m model) renderChats(height int, overlays *[]overlay) string {
	brand := lipgloss.NewStyle().Background(yellow).Foreground(ink).Bold(true).Padding(1, 2).Width(leftWidth).Render("NEOCHAT")
	if len(m.chats) == 0 {
		return brand + "\n\n  Menunggu sinkronisasi...\n\n  Atau mulai dengan:\n  /new 628123456789"
	}

	rows := m.rows()
	selected := m.selected()
	start := m.top

	textW := leftWidth - avCols - 3
	line := lipgloss.NewStyle().Foreground(edge).Render(strings.Repeat("━", leftWidth))
	out := []string{brand}
	for i := start; i < len(m.chats) && i < start+rows; i++ {
		c := m.chats[i]
		base := lipgloss.NewStyle()
		sub := lipgloss.NewStyle().Faint(true)
		if c.Unread > 0 {
			sub = lipgloss.NewStyle().Bold(true)
		}
		if i == selected {
			base = base.Background(pink).Foreground(ink)
			sub = base
		}
		bg := paper
		if i == selected {
			bg = pink
		}
		av := m.avatar(c.Name, c.JID, avCols, avRows, bg)
		gap := base.Render("  ")
		end := base.Render(" ")
		out = append(out,
			av[0]+gap+m.nameLine(base, c, textW)+end,
			av[1]+gap+sub.Render(fit(c.Preview, textW))+end,
			av[2]+gap+base.Render(fit("", textW))+end,
			line,
		)
	}
	return strings.Join(out, "\n")
}

func (m model) renderChat(width, height int, overlays *[]overlay) string {
	name, sub := "Pilih chat", ""
	if m.cur != "" {
		name = clean(m.app.Name(m.cur))
		sub = "chat pribadi"
		if strings.HasSuffix(m.cur, "@g.us") {
			sub = "grup"
		}
	}
	if m.scroll > 0 {
		sub += "  ·  scroll ke bawah untuk pesan terbaru"
	}
	header := m.renderHeader(name, sub, width)
	if itermImages {
		if data := m.app.AvatarB64(m.cur, 192); data != "" {
			*overlays = append(*overlays, overlay{row: 3, col: rightX + 2, cols: 12, rows: headH - 1, data: data})
		}
	}

	inner := width - 2
	var bubbles []string
	var spots []spot
	offset := 0
	for _, msg := range m.msgs {
		block, s := m.bubble(msg, inner)
		if s != nil {
			s.line += offset
			spots = append(spots, *s)
		}
		bubbles = append(bubbles, block)
		offset += strings.Count(block, "\n") + 2
	}
	if len(bubbles) == 0 {
		bubbles = []string{"Belum ada pesan."}
		if m.cur == "" {
			bubbles = []string{"Pilih chat di kiri untuk mulai.", "", "Esc menutup chat yang sedang terbuka."}
		}
	}
	lines := strings.Split(strings.Join(bubbles, "\n\n"), "\n")
	msgH := height - headH - 3
	sc := min(m.scroll, max(0, len(lines)-msgH))
	end := len(lines) - sc
	first := max(0, end-msgH)
	lines = lines[first:end]
	areaTop, areaBottom := 2+headH, 2+headH+msgH
	for _, s := range spots {
		y0 := max(areaTop, areaTop+s.line-first)
		y1 := min(areaBottom, areaTop+s.line-first+s.rows)
		if y0 < y1 {
			m.frame.hits = append(m.frame.hits, hit{row: y0, col: rightX + 1 + s.x, cols: s.cols, rows: y1 - y0, chat: s.chat, id: s.id})
		}
		if s.data != "" && s.line >= first && s.line+s.rows <= end {
			*overlays = append(*overlays, overlay{row: areaTop + s.line - first, col: rightX + 1 + s.x, cols: s.cols, rows: s.rows, data: s.data})
		}
	}
	area := lipgloss.NewStyle().Padding(0, 1).Height(msgH).Render(strings.Join(lines, "\n"))

	return header + "\n" + area + "\n" + m.renderInput(width)
}

func (m model) renderHeader(name, sub string, width int) string {
	g := lipgloss.NewStyle().Background(green).Foreground(ink)
	av := m.avatar(name, m.cur, 12, headH-1, green)
	row := func(parts ...string) string {
		s := strings.Join(parts, "")
		return s + g.Render(strings.Repeat(" ", max(0, width-lipgloss.Width(s))))
	}
	lines := []string{g.Render(strings.Repeat(" ", width))}
	for i, part := range av {
		text := ""
		switch i {
		case 2:
			text = g.Bold(true).Render(runewidth.Truncate(name, width-18, "…"))
		case 3:
			text = g.Render(runewidth.Truncate(sub, width-18, "…"))
		}
		lines = append(lines, row(g.Render("  "), part, g.Render("  "), text))
	}
	return strings.Join(lines, "\n")
}

func (m model) renderInput(width int) string {
	send := lipgloss.NewStyle().Background(pink).Foreground(ink).Bold(true).Padding(1, 2).Render("KIRIM ↵")
	boxW := width - 2 - lipgloss.Width(send) - 1
	m.input.Width = boxW - 6
	box := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(edge).
		Padding(0, 1).
		Width(boxW - 2).
		Render(m.input.View())
	return lipgloss.NewStyle().Padding(0, 1).Render(lipgloss.JoinHorizontal(lipgloss.Top, box, " ", send))
}

func (m model) bubble(msg Message, width int) (string, *spot) {
	bg := paper
	pos := lipgloss.Left
	if msg.FromMe {
		bg = yellow
		pos = lipgloss.Right
	}

	text := msg.Text
	var parts []string
	var photo *spot
	if !msg.FromMe && strings.HasSuffix(msg.Chat, "@g.us") {
		parts = append(parts, runewidth.Truncate(clean(m.app.Name(msg.Sender)), width-12, "…"))
	}
	if len(msg.Thumb) > 0 {
		maxCols, maxRows := min(60, width-14), max(6, min(28, m.h-20))
		if strings.HasPrefix(text, "[stiker]") {
			maxCols, maxRows = min(24, maxCols), min(12, maxRows)
		}
		if img := m.app.Image(msg.Chat+msg.ID, msg.Thumb, maxCols, maxRows, rgba(bg)); img != nil {
			photo = &spot{line: 1 + len(parts), x: 2, cols: lipgloss.Width(img[0]), rows: len(img), chat: msg.Chat, id: msg.ID}
			if itermImages {
				photo.data = m.app.OpaqueB64(msg.Chat+msg.ID, msg.Thumb, rgba(bg))
			}
			parts = append(parts, img...)
			text = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(text, "[foto]"), "[stiker]"))
		}
	}
	if text != "" {
		parts = append(parts, text)
	}
	parts = append(parts, msg.Time.Format("15:04"))
	body := strings.Join(parts, "\n")

	textW := min(lipgloss.Width(body), width-9)
	box := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(edge).
		Background(bg).
		Foreground(ink).
		Padding(0, 1).
		Width(textW + 2).
		Render(body)

	shadow := lipgloss.NewStyle().Foreground(edge)
	lines := strings.Split(box, "\n")
	boxW := lipgloss.Width(lines[0])
	lines[0] += " "
	for i := 1; i < len(lines); i++ {
		lines[i] += shadow.Render("█")
	}
	lines = append(lines, " "+shadow.Render(strings.Repeat("▀", boxW)))
	if photo != nil && msg.FromMe {
		photo.x += width - (boxW + 1)
	}
	return lipgloss.PlaceHorizontal(width, pos, strings.Join(lines, "\n")), photo
}

func (m model) sendFile(chat, path, caption string) (tea.Model, tea.Cmd) {
	m.notice = "mengirim foto..."
	return m, func() tea.Msg {
		data, err := os.ReadFile(path)
		if err != nil {
			return sentMsg{err: err}
		}
		return sentMsg{err: m.app.SendImage(chat, data, caption)}
	}
}

func (m model) pasted(msg pasteMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil || m.cur == "" {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(tea.KeyMsg{Type: tea.KeyCtrlV})
		return m, cmd
	}
	caption := strings.TrimSpace(m.input.Value())
	m.input.Reset()
	m.notice = "mengirim foto..."
	chat := m.cur
	return m, func() tea.Msg { return sentMsg{err: m.app.SendImage(chat, msg.data, caption)} }
}

func (m model) nameLine(base lipgloss.Style, c Chat, width int) string {
	if c.Unread == 0 {
		return base.Bold(true).Render(fit(c.Name, width))
	}
	count := fmt.Sprint(c.Unread)
	if c.Unread > 99 {
		count = "99+"
	}
	badge := lipgloss.NewStyle().Background(yellow).Foreground(ink).Bold(true).Render(" " + count + " ")
	return base.Bold(true).Render(fit(c.Name, width-lipgloss.Width(badge)-1)) + base.Render(" ") + badge
}

var emojiOK = os.Getenv("WA_NEO_EMOJI") != "off"

func isEmoji(r rune) bool {
	return r >= 0x1F300 && r <= 0x1FAFF || r >= 0x2600 && r <= 0x27BF || r == 0x2B50 || r == 0x2B55
}
