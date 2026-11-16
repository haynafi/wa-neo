package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mdp/qrterminal/v3"
)

var version = "dev"

func dataDir() string {
	if dir := os.Getenv("WA_NEO_DIR"); dir != "" {
		return dir
	}
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	return filepath.Join(base, "wa-neo")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

func login(app *App) {
	qr, err := app.Client.GetQRChannel(context.Background())
	if err != nil {
		fail(err)
	}
	if err := app.Client.Connect(); err != nil {
		fail(err)
	}
	for item := range qr {
		switch item.Event {
		case "code":
			fmt.Print("\033[H\033[2J")
			fmt.Println("Buka WhatsApp di HP > Perangkat tertaut > Tautkan perangkat, lalu scan QR ini:")
			qrterminal.GenerateHalfBlock(item.Code, qrterminal.L, os.Stdout)
		case "success":
			fmt.Println("Berhasil tertaut.")
			return
		case "error":
			fail(item.Error)
		case "timeout":
			fail(fmt.Errorf("QR kedaluwarsa, jalankan lagi"))
		}
	}
}

func main() {
	showVersion := flag.Bool("version", false, "tampilkan versi")
	flag.Parse()
	if *showVersion {
		fmt.Println("wa-neo", version)
		return
	}

	dir := dataDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		fail(err)
	}
	app, err := OpenApp(dir)
	if err != nil {
		fail(err)
	}

	if app.Client.Store.ID == nil {
		login(app)
	} else if err := app.Client.Connect(); err != nil {
		fail(err)
	}
	defer app.Client.Disconnect()

	if _, err := tea.NewProgram(newModel(app), tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithFPS(30), tea.WithReportFocus()).Run(); err != nil {
		fail(err)
	}
}
