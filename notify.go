package main

import (
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"
)

var notifyOff = os.Getenv("WA_NEO_NOTIFY") == "off"

func (a *App) SetView(chat string, focused bool) {
	a.mu.Lock()
	a.current, a.focused = chat, focused
	a.mu.Unlock()
}

func (a *App) alert(chat, sender, text string) {
	if notifyOff {
		return
	}
	now := time.Now()
	a.mu.Lock()
	skip := now.Before(a.quietUntil) ||
		(a.focused && a.current == chat) ||
		now.Sub(a.lastNotify[chat]) < 5*time.Second ||
		now.Sub(a.lastAny) < time.Second
	if !skip {
		a.lastNotify[chat], a.lastAny = now, now
	}
	a.mu.Unlock()
	if skip {
		return
	}

	title, body := a.Name(chat), text
	if strings.HasSuffix(chat, "@g.us") {
		body = a.Name(sender) + ": " + text
	}
	if utf8.RuneCountInString(body) > 160 {
		body = string([]rune(body)[:160]) + "…"
	}
	go a.notifier(title, body)
}

func notify(title, body string) error {
	return exec.Command("osascript",
		"-e", "on run argv",
		"-e", `display notification (item 2 of argv) with title (item 1 of argv) sound name "Pop"`,
		"-e", "end run",
		"--", title, body,
	).Run()
}
