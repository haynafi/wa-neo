package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestViewShowsChatsAndMessages(t *testing.T) {
	app, err := OpenApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	app.insert(Message{ID: "1", Chat: "628111@s.whatsapp.net", Sender: "628111@s.whatsapp.net", Text: "halo dari budi", Time: now})
	app.insert(Message{ID: "2", Chat: "628111@s.whatsapp.net", Text: "halo balik", FromMe: true, Time: now})

	m := newModel(app)
	m.choose("628111@s.whatsapp.net")
	m.choose("628111@s.whatsapp.net")
	m.w, m.h = 100, 30
	view := m.View()

	for _, want := range []string{"WA-NEO", "628111", "halo dari budi", "halo balik"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q", want)
		}
	}
}

func TestOpenRejectsShortNumbers(t *testing.T) {
	app, err := OpenApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if app.Open("123") != "" {
		t.Error("short number should be rejected")
	}
	if got := app.Open("+62 812-3456-7890"); got != "6281234567890@s.whatsapp.net" {
		t.Errorf("got %q", got)
	}
}

func gradient(alpha bool) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 80, 60))
	for y := 0; y < 60; y++ {
		for x := 0; x < 80; x++ {
			a := uint8(255)
			if alpha && (x < 10 || y < 10) {
				a = 0
			}
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 3), G: uint8(y * 4), B: 120, A: a})
		}
	}
	return img
}

func TestViewRendersImagesAndAvatars(t *testing.T) {
	dir := t.TempDir()
	app, err := OpenApp(dir)
	if err != nil {
		t.Fatal(err)
	}

	var photo, sticker, avatar bytes.Buffer
	jpeg.Encode(&photo, gradient(false), nil)
	png.Encode(&sticker, gradient(true))
	jpeg.Encode(&avatar, gradient(false), nil)

	chat := "628111@s.whatsapp.net"
	os.MkdirAll(filepath.Join(dir, "avatars"), 0o700)
	os.WriteFile(filepath.Join(dir, "avatars", "628111_s.whatsapp.net.jpg"), avatar.Bytes(), 0o600)

	now := time.Now()
	app.insert(Message{ID: "1", Chat: chat, Sender: chat, Text: "[foto] liat nih", Time: now, Thumb: photo.Bytes()})
	app.insert(Message{ID: "2", Chat: chat, Text: "[stiker]", FromMe: true, Time: now, Thumb: sticker.Bytes()})

	m := newModel(app)
	m.choose(chat)
	m.w, m.h = 110, 40
	m.View()
	for i := 0; i < 50 && app.AvatarRows(chat, avCols, avRows, color.RGBA{}) == nil; i++ {
		time.Sleep(20 * time.Millisecond)
	}
	view := m.View()

	if strings.Count(view, "▀") < 200 {
		t.Errorf("expected image blocks in view, got %d", strings.Count(view, "▀"))
	}
	if strings.Contains(view, "[foto]") {
		t.Error("photo label should be replaced by the image")
	}
	if !strings.Contains(view, "liat nih") {
		t.Error("caption missing")
	}
	lines := strings.Split(view, "\n")
	if len(lines) > m.h {
		t.Errorf("too tall: %d", len(lines))
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w > m.w {
			t.Errorf("line %d too wide: %d", i, w)
		}
	}
}

func TestApplyOverlaysKeepsSurroundingText(t *testing.T) {
	lines := []string{
		"abcdefghijklmnopqrstuvwxyz",
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ",
		"0123456789abcdefghijklmnop",
	}
	screen := strings.Join(lines, "\n")
	out := applyOverlays(screen, []overlay{
		{row: 0, col: 2, cols: 3, rows: 2, data: "AAAA"},
		{row: 0, col: 15, cols: 4, rows: 2, data: "BBBB"},
	})

	got := strings.Split(out, "\n")
	if strings.Count(out, "1337;File=inline=1") != 2 {
		t.Errorf("expected 2 image sequences")
	}
	if !strings.Contains(got[0], "\x1b[3C") || !strings.Contains(got[1], "\x1b[4C") {
		t.Errorf("rows should skip image cells with cursor-forward: %q", got)
	}
	if strings.Contains(got[1], "1337") || strings.Contains(got[2], "\x1b") {
		t.Errorf("only the first image row draws, other rows stay untouched: %q", got)
	}
	want := []string{
		"ab" + "fghijklmno" + "tuvwxyz",
		"AB" + "FGHIJKLMNO" + "TUVWXYZ",
		lines[2],
	}
	for i, line := range got {
		if plain := ansi.Strip(line); plain != want[i] {
			t.Errorf("row %d: got %q want %q", i, plain, want[i])
		}
	}
}

func TestViewDrawsITermImages(t *testing.T) {
	itermImages = true
	defer func() { itermImages = false }()

	dir := t.TempDir()
	app, err := OpenApp(dir)
	if err != nil {
		t.Fatal(err)
	}
	var photo, avatar bytes.Buffer
	jpeg.Encode(&photo, gradient(false), nil)
	jpeg.Encode(&avatar, gradient(false), nil)

	chat := "628111@s.whatsapp.net"
	os.MkdirAll(filepath.Join(dir, "avatars-hd"), 0o700)
	os.WriteFile(filepath.Join(dir, "avatars-hd", "628111_s.whatsapp.net.jpg"), avatar.Bytes(), 0o600)
	app.insert(Message{ID: "1", Chat: chat, Sender: chat, Text: "[foto] halo", Time: time.Now(), Thumb: photo.Bytes()})

	m := newModel(app)
	m.choose(chat)
	m.w, m.h = 110, 40
	m.View()
	for i := 0; i < 50 && app.AvatarB64(chat, 96) == ""; i++ {
		time.Sleep(20 * time.Millisecond)
	}
	view := m.View()

	if n := strings.Count(view, "1337;File=inline=1"); n != 2 {
		t.Errorf("expected sharp header avatar and chat photo only, got %d", n)
	}
	if strings.Count(view, "▀") < avCols*avRows {
		t.Error("sidebar avatars should stay pixel blocks")
	}
	for i, line := range strings.Split(view, "\n") {
		if w := ansi.StringWidth(line); w > m.w-1 {
			t.Errorf("line %d wider than layout: %d", i, w)
		}
	}
}

func TestClickingPhotoIsRecognised(t *testing.T) {
	app, err := OpenApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var photo bytes.Buffer
	jpeg.Encode(&photo, gradient(false), nil)
	chat := "628111@s.whatsapp.net"
	app.insert(Message{ID: "p1", Chat: chat, Sender: chat, Text: "[foto]", Time: time.Now(), Thumb: photo.Bytes()})

	m := newModel(app)
	m.choose(chat)
	m.w, m.h = 110, 40
	m.View()
	if len(m.frame.hits) != 1 {
		t.Fatalf("expected 1 clickable photo, got %d", len(m.frame.hits))
	}
	h := m.frame.hits[0]
	if h.chat != chat || h.id != "p1" {
		t.Errorf("wrong hit: %+v", h)
	}

	_, cmd := m.mouse(tea.MouseMsg{X: h.col + 1, Y: h.row + 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if cmd == nil {
		t.Error("click on photo should open it")
	}
	_, cmd = m.mouse(tea.MouseMsg{X: h.col + h.cols + 5, Y: h.row, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if cmd != nil {
		t.Error("click outside photo should do nothing")
	}
}

func TestImagePathHandlesDroppedFiles(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "foto liburan.png")
	os.WriteFile(file, []byte("x"), 0o600)

	cases := map[string]bool{
		file:                                true,
		strings.ReplaceAll(file, " ", `\ `): true,
		"'" + file + "'":                    true,
		"file://" + strings.ReplaceAll(file, " ", "%20"): true,
		filepath.Join(dir, "tidak-ada.png"):              false,
		filepath.Join(dir, "catatan.txt"):                false,
		"halo apa kabar":                                 false,
	}
	for input, want := range cases {
		if _, ok := imagePath(input); ok != want {
			t.Errorf("imagePath(%q) = %v, want %v", input, ok, want)
		}
	}

	path, caption, ok := splitImageArg(strings.ReplaceAll(file, " ", `\ `) + " lihat ini")
	if !ok || path != file || caption != "lihat ini" {
		t.Errorf("splitImageArg: %q %q %v", path, caption, ok)
	}
}

func TestLeakedMouseCodesAreNotTyped(t *testing.T) {
	app, err := OpenApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var tm tea.Model = newModel(app)
	tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("[<65;78;28M")})
	tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("halo")})
	if got := tm.(model).input.Value(); got != "halo" {
		t.Errorf("input = %q, want only typed text", got)
	}
}

func TestUnreadBadgeAndEscClosesChat(t *testing.T) {
	app, err := OpenApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	budi, sari := "628111@s.whatsapp.net", "628222@s.whatsapp.net"
	now := time.Now()
	app.insert(Message{ID: "1", Chat: budi, Sender: budi, Text: "p", Time: now})
	app.insert(Message{ID: "2", Chat: budi, Sender: budi, Text: "ping", Time: now})
	app.insert(Message{ID: "3", Chat: sari, Text: "balasanku", FromMe: true, Time: now})

	m := newModel(app)
	m.w, m.h = 110, 40
	if m.cur != "" {
		t.Fatal("no chat should be open at startup")
	}
	counts := map[string]int{}
	for _, c := range m.chats {
		counts[c.JID] = c.Unread
	}
	if counts[budi] != 2 || counts[sari] != 0 {
		t.Errorf("unread counts: %v (incoming = 2, own message = 0)", counts)
	}
	if view := m.View(); !strings.Contains(view, "2 belum dibaca") {
		t.Error("title should show total unread")
	}

	m.choose(budi)
	for _, c := range m.chats {
		if c.JID == budi && c.Unread != 0 {
			t.Error("opening a chat should mark it read")
		}
	}

	app.insert(Message{ID: "4", Chat: budi, Sender: budi, Text: "masih di sini?", Time: now.Add(2 * time.Second)})
	m.reload()
	for _, c := range m.chats {
		if c.JID == budi && c.Unread != 0 {
			t.Error("messages arriving in the open chat stay read")
		}
	}

	var tm tea.Model = m
	tm, cmd := tm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	tm, _ = tm.Update(cmd())
	if tm.(model).cur != "" {
		t.Error("esc should close the open chat")
	}

	app.insert(Message{ID: "5", Chat: budi, Sender: budi, Text: "halo lagi", Time: now.Add(5 * time.Second)})
	closed := tm.(model)
	closed.reload()
	for _, c := range closed.chats {
		if c.JID == budi && c.Unread != 1 {
			t.Errorf("after closing, a new message should be unread, got %d", c.Unread)
		}
	}
}

func TestSplitMouseCodeNeitherTypesNorClosesChat(t *testing.T) {
	app, err := OpenApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	chat := "628111@s.whatsapp.net"
	app.insert(Message{ID: "1", Chat: chat, Sender: chat, Text: "hai", Time: time.Now()})
	m := newModel(app)
	m.choose(chat)

	var tm tea.Model = m
	tm, cmd := tm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	for _, piece := range []string{"[", "<", "65;63;20M"} {
		tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(piece)})
	}
	tm, _ = tm.Update(cmd())
	got := tm.(model)
	if got.cur != chat {
		t.Error("an Esc that starts a split mouse code must not close the chat")
	}
	if got.input.Value() != "" {
		t.Errorf("mouse code leaked into input: %q", got.input.Value())
	}

	tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("[<64;63;20M")})
	if tm.(model).input.Value() != "" {
		t.Error("whole leaked code should be dropped")
	}
}

func TestFindImageAcceptsPastedPathWithCaption(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "pasted-image-1.png")
	os.WriteFile(file, []byte("x"), 0o600)

	cases := []struct{ text, caption string }{
		{file, ""},
		{file + " penampakan full", "penampakan full"},
		{"penampakan full " + file, "penampakan full"},
	}
	for _, c := range cases {
		path, caption, ok := findImage(c.text)
		if !ok || path != file || caption != c.caption {
			t.Errorf("findImage(%q) = %q %q %v", c.text, path, caption, ok)
		}
	}
	for _, text := range []string{"lihat foto.png ini", "halo", "/tmp/tidak-ada.png caption"} {
		if _, _, ok := findImage(text); ok {
			t.Errorf("findImage(%q) should not match", text)
		}
	}
}

func TestMergeChatsCombinesLIDAndPhoneChats(t *testing.T) {
	app, err := OpenApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	lid, pn := "999@lid", "628111@s.whatsapp.net"
	now := time.Now()
	app.insert(Message{ID: "a", Chat: pn, Sender: pn, Text: "Ya", Time: now})
	app.insert(Message{ID: "a", Chat: lid, Sender: lid, Text: "Ya", Time: now})
	app.insert(Message{ID: "b", Chat: lid, Sender: lid, Text: "oke", Time: now.Add(time.Second)})

	app.mergeChats(func(chat string) string {
		if chat == lid {
			return pn
		}
		return chat
	})

	chats := app.Chats()
	if len(chats) != 1 || chats[0].JID != pn {
		t.Fatalf("expected one merged chat, got %+v", chats)
	}
	if n := len(app.Messages(pn, 10)); n != 2 {
		t.Errorf("duplicate message should collapse: got %d messages, want 2", n)
	}
}

func TestCleanKeepsSimpleEmojiAndDropsSequences(t *testing.T) {
	cases := map[string]string{
		"hai 😍":      "hai 😍",
		"makasih 🙏🏻": "makasih 🙏",
		"oke ✅":      "oke ✅",
		"❤️":         "❤",
		"bendera 🇮🇩": "bendera ",
		"tab\there":  "tab  here",
	}
	for in, want := range cases {
		if got := clean(in); got != want {
			t.Errorf("clean(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAlertSkipsOpenFocusedChatAndLimitsRate(t *testing.T) {
	app, err := OpenApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	chat := "628111@s.whatsapp.net"
	sent := make(chan string, 4)
	app.notifier = func(title, body string) error { sent <- title + "|" + body; return nil }

	app.SetView(chat, true)
	app.alert(chat, chat, "halo")
	if !app.lastNotify[chat].IsZero() {
		t.Error("no alert for the chat being read in a focused window")
	}

	app.SetView(chat, false)
	app.alert(chat, chat, "halo")
	if app.lastNotify[chat].IsZero() {
		t.Error("an unfocused window should still alert")
	}
	if got := <-sent; got != "628111|halo" {
		t.Errorf("notification = %q", got)
	}
	first := app.lastNotify[chat]
	app.alert(chat, chat, "halo lagi")
	if !app.lastNotify[chat].Equal(first) {
		t.Error("alerts for the same chat should be rate limited")
	}

	app.quietUntil = time.Now().Add(time.Minute)
	other := "628222@s.whatsapp.net"
	app.alert(other, other, "offline catch-up")
	if !app.lastNotify[other].IsZero() {
		t.Error("no alerts right after reconnecting")
	}
}
