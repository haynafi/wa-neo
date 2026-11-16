package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

var imageExt = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}

func jpegOf(img image.Image, maxDim int) []byte {
	small := scaleDown(img, maxDim)
	canvas := image.NewRGBA(small.Bounds())
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(canvas, canvas.Bounds(), small, image.Point{}, draw.Over)

	var buf bytes.Buffer
	jpeg.Encode(&buf, canvas, &jpeg.Options{Quality: 90})
	return buf.Bytes()
}

func (a *App) SendImage(chat string, data []byte, caption string) error {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("gambar tidak didukung: %w", err)
	}
	jid, err := types.ParseJID(chat)
	if err != nil {
		return err
	}

	full := jpegOf(img, 1600)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	up, err := a.Client.Upload(ctx, full, whatsmeow.MediaImage)
	if err != nil {
		return fmt.Errorf("gagal unggah: %w", err)
	}
	resp, err := a.Client.SendMessage(ctx, jid, &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
		URL:           proto.String(up.URL),
		DirectPath:    proto.String(up.DirectPath),
		MediaKey:      up.MediaKey,
		FileEncSHA256: up.FileEncSHA256,
		FileSHA256:    up.FileSHA256,
		FileLength:    proto.Uint64(up.FileLength),
		Mimetype:      proto.String("image/jpeg"),
		Caption:       proto.String(caption),
		JPEGThumbnail: jpegOf(img, 100),
	}})
	if err != nil {
		return err
	}

	a.insert(Message{
		ID:     resp.ID,
		Chat:   chat,
		Text:   strings.TrimSpace("[foto] " + caption),
		FromMe: true,
		Time:   resp.Timestamp,
		Thumb:  jpegOf(img, 480),
	})
	a.saveOriginal(chat, resp.ID, full)
	a.notify()
	return nil
}

func clipboardImage() ([]byte, error) {
	path := filepath.Join(os.TempDir(), fmt.Sprintf("wa-neo-clip-%d.png", os.Getpid()))
	defer os.Remove(path)

	lines := []string{
		"set png to the clipboard as «class PNGf»",
		`set f to open for access POSIX file "` + path + `" with write permission`,
		"set eof f to 0",
		"write png to f",
		"close access f",
	}
	var args []string
	for _, line := range lines {
		args = append(args, "-e", line)
	}
	if err := exec.Command("osascript", args...).Run(); err != nil {
		return nil, fmt.Errorf("tidak ada gambar di clipboard")
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil, fmt.Errorf("tidak ada gambar di clipboard")
	}
	return data, nil
}

func clipboardCmd() tea.Msg {
	data, err := clipboardImage()
	return pasteMsg{data: data, err: err}
}

func unescape(s string) string {
	var b strings.Builder
	escaped := false
	for _, r := range s {
		if r == '\\' && !escaped {
			escaped = true
			continue
		}
		escaped = false
		b.WriteRune(r)
	}
	return b.String()
}

func imagePath(s string) (string, bool) {
	s = strings.Trim(strings.TrimSpace(s), `'"`)
	if rest, ok := strings.CutPrefix(s, "file://"); ok {
		if decoded, err := url.PathUnescape(rest); err == nil {
			s = decoded
		}
	}
	s = unescape(s)
	if rest, ok := strings.CutPrefix(s, "~/"); ok {
		home, _ := os.UserHomeDir()
		s = filepath.Join(home, rest)
	}
	if !imageExt[strings.ToLower(filepath.Ext(s))] {
		return "", false
	}
	info, err := os.Stat(s)
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	return s, true
}

func splitImageArg(arg string) (path, caption string, ok bool) {
	if path, ok := imagePath(arg); ok {
		return path, "", true
	}
	arg = strings.TrimSpace(arg)
	for i := 1; i < len(arg); i++ {
		if arg[i] != ' ' || arg[i-1] == '\\' {
			continue
		}
		if path, ok := imagePath(arg[:i]); ok {
			return path, strings.TrimSpace(arg[i:]), true
		}
	}
	return "", "", false
}

func findImage(text string) (path, caption string, ok bool) {
	if path, caption, ok := splitImageArg(text); ok {
		return path, caption, true
	}
	text = strings.TrimSpace(text)
	for i := len(text) - 2; i > 0; i-- {
		if text[i] != ' ' || text[i-1] == '\\' {
			continue
		}
		if path, ok := imagePath(text[i+1:]); ok {
			return path, strings.TrimSpace(text[:i]), true
		}
	}
	return "", "", false
}
