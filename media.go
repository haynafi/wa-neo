package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	_ "golang.org/x/image/webp"
)

const avatarMaxAge = 7 * 24 * time.Hour

func cover(b image.Rectangle, tw, th int) image.Rectangle {
	w, h := b.Dx(), b.Dy()
	if w*th > h*tw {
		nw := h * tw / th
		x := b.Min.X + (w-nw)/2
		return image.Rect(x, b.Min.Y, x+nw, b.Max.Y)
	}
	nh := w * th / tw
	y := b.Min.Y + (h-nh)/2
	return image.Rect(b.Min.X, y, b.Max.X, y+nh)
}

func mean(img image.Image, x0, y0, x1, y1 int) (r, g, b, a uint32) {
	var n uint32
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			pr, pg, pb, pa := img.At(x, y).RGBA()
			r, g, b, a, n = r+pr, g+pg, b+pb, a+pa, n+1
		}
	}
	if n == 0 {
		return 0, 0, 0, 0
	}
	return r / n, g / n, b / n, a / n
}

func average(img image.Image, x0, y0, x1, y1 int, bg color.RGBA) color.RGBA {
	r, g, b, a := mean(img, x0, y0, x1, y1)
	inv := 65535 - a
	mix := func(c uint32, back uint8) uint8 {
		return uint8((c + uint32(back)*257*inv/65535) >> 8)
	}
	return color.RGBA{R: mix(r, bg.R), G: mix(g, bg.G), B: mix(b, bg.B), A: 255}
}

func blocks(img image.Image, cols, rows int, bg color.RGBA) []string {
	th := rows * 2
	c := cover(img.Bounds(), cols, th)
	pixel := func(col, row int) color.RGBA {
		x0 := c.Min.X + col*c.Dx()/cols
		y0 := c.Min.Y + row*c.Dy()/th
		x1 := max(x0+1, c.Min.X+(col+1)*c.Dx()/cols)
		y1 := max(y0+1, c.Min.Y+(row+1)*c.Dy()/th)
		return average(img, x0, y0, x1, y1, bg)
	}

	out := make([]string, rows)
	var sb strings.Builder
	for row := 0; row < rows; row++ {
		sb.Reset()
		for col := 0; col < cols; col++ {
			top, bottom := pixel(col, row*2), pixel(col, row*2+1)
			fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%d;48;2;%d;%d;%dm▀", top.R, top.G, top.B, bottom.R, bottom.G, bottom.B)
		}
		sb.WriteString("\x1b[0m")
		out[row] = sb.String()
	}
	return out
}

func fitSize(b image.Rectangle, maxCols, maxRows int) (int, int) {
	w, h := max(1, b.Dx()), max(1, b.Dy())
	cols := maxCols
	pixelH := cols * h / w
	if pixelH > maxRows*2 {
		pixelH = maxRows * 2
		cols = pixelH * w / h
	}
	return max(cols, 2), max((pixelH+1)/2, 1)
}

func (a *App) Image(key string, data []byte, maxCols, maxRows int, bg color.RGBA) []string {
	cacheKey := fmt.Sprint(key, maxCols, maxRows, bg)
	if cached, ok := a.blockCache.Load(cacheKey); ok {
		return cached.([]string)
	}
	var out []string
	if img, _, err := image.Decode(bytes.NewReader(data)); err == nil {
		cols, rows := fitSize(img.Bounds(), maxCols, maxRows)
		out = blocks(img, cols, rows, bg)
	}
	a.blockCache.Store(cacheKey, out)
	return out
}

func (a *App) AvatarRows(jid string, cols, rows int, bg color.RGBA) []string {
	img := a.avatar(jid)
	if img == nil {
		return nil
	}
	cacheKey := fmt.Sprint("avatar", jid, cols, rows, bg)
	if cached, ok := a.blockCache.Load(cacheKey); ok {
		return cached.([]string)
	}
	out := blocks(img, cols, rows, bg)
	a.blockCache.Store(cacheKey, out)
	return out
}

func (a *App) avatar(jid string) image.Image {
	a.mu.Lock()
	img, tried := a.avatars[jid], a.avatarTried[jid]
	a.avatarTried[jid] = true
	a.mu.Unlock()
	if !tried {
		go a.loadAvatar(jid)
	}
	return img
}

func (a *App) loadAvatar(jid string) {
	a.sem <- struct{}{}
	defer func() { <-a.sem }()

	name := strings.NewReplacer("@", "_", ":", "_", "/", "_").Replace(jid) + ".jpg"
	path := filepath.Join(a.dir, "avatars-hd", name)

	data, _ := os.ReadFile(path)
	if info, err := os.Stat(path); data == nil || err != nil || time.Since(info.ModTime()) > avatarMaxAge {
		if fresh := a.fetchAvatar(jid); fresh != nil {
			data = fresh
			os.MkdirAll(filepath.Dir(path), 0o700)
			os.WriteFile(path, data, 0o600)
		}
	}
	if data == nil {
		return
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return
	}
	a.mu.Lock()
	a.avatars[jid] = img
	a.mu.Unlock()
	a.notify()
}

func (a *App) AvatarB64(jid string, dim int) string {
	img := a.avatar(jid)
	if img == nil {
		return ""
	}
	cacheKey := fmt.Sprint("avatar64", jid, dim)
	if cached, ok := a.blockCache.Load(cacheKey); ok {
		return cached.(string)
	}
	out := encodeImage(jpegOf(img, dim))
	a.blockCache.Store(cacheKey, out)
	return out
}

func (a *App) OpaqueB64(key string, data []byte, bg color.RGBA) string {
	cacheKey := fmt.Sprint("opaque64", key, bg)
	if cached, ok := a.blockCache.Load(cacheKey); ok {
		return cached.(string)
	}
	out := data
	if img, format, err := image.Decode(bytes.NewReader(data)); err == nil && format != "jpeg" {
		canvas := image.NewRGBA(img.Bounds())
		draw.Draw(canvas, canvas.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
		draw.Draw(canvas, canvas.Bounds(), img, img.Bounds().Min, draw.Over)
		var buf bytes.Buffer
		if png.Encode(&buf, canvas) == nil {
			out = buf.Bytes()
		}
	}
	encoded := encodeImage(out)
	a.blockCache.Store(cacheKey, encoded)
	return encoded
}

func (a *App) fetchAvatar(jidStr string) []byte {
	if !a.Client.IsConnected() {
		a.mu.Lock()
		delete(a.avatarTried, jidStr)
		a.mu.Unlock()
		return nil
	}
	jid, err := types.ParseJID(jidStr)
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	info, err := a.Client.GetProfilePictureInfo(ctx, jid, &whatsmeow.GetProfilePictureParams{})
	if err != nil || info == nil || info.URL == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, info.URL, nil)
	if err != nil {
		return nil
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	var buf bytes.Buffer
	if jpeg.Encode(&buf, scaleDown(img, 256), &jpeg.Options{Quality: 88}) != nil {
		return nil
	}
	return buf.Bytes()
}

func scaleDown(img image.Image, maxDim int) *image.NRGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	f := min(1.0, float64(maxDim)/float64(max(w, h)))
	tw, th := max(1, int(float64(w)*f)), max(1, int(float64(h)*f))

	out := image.NewNRGBA(image.Rect(0, 0, tw, th))
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			x0, y0 := b.Min.X+x*w/tw, b.Min.Y+y*h/th
			x1, y1 := max(x0+1, b.Min.X+(x+1)*w/tw), max(y0+1, b.Min.Y+(y+1)*h/th)
			r, g, bl, a := mean(img, x0, y0, x1, y1)
			if a == 0 {
				continue
			}
			out.SetNRGBA(x, y, color.NRGBA{R: uint8(r * 255 / a), G: uint8(g * 255 / a), B: uint8(bl * 255 / a), A: uint8(a >> 8)})
		}
	}
	return out
}

func thumbOf(m *waE2E.Message) []byte {
	switch {
	case m.GetImageMessage() != nil:
		return m.GetImageMessage().GetJPEGThumbnail()
	case m.GetStickerMessage() != nil:
		return m.GetStickerMessage().GetPngThumbnail()
	case m.GetVideoMessage() != nil:
		return m.GetVideoMessage().GetJPEGThumbnail()
	}
	return nil
}

func downloadable(m *waE2E.Message) whatsmeow.DownloadableMessage {
	switch {
	case m.GetStickerMessage() != nil:
		return m.GetStickerMessage()
	case m.GetImageMessage() != nil:
		return m.GetImageMessage()
	}
	return nil
}

func (a *App) fetchThumb(e *events.Message, chat string) {
	media := downloadable(e.Message)
	if media == nil {
		return
	}
	a.sem <- struct{}{}
	defer func() { <-a.sem }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	data, err := a.Client.Download(ctx, media)
	if err != nil {
		a.logf("unduh media %s gagal: %v", e.Info.ID, err)
		return
	}
	a.saveOriginal(chat, e.Info.ID, data)
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		a.logf("decode media %s gagal: %v", e.Info.ID, err)
		return
	}
	sticker := e.Message.GetStickerMessage() != nil
	maxDim := 480
	if sticker {
		maxDim = 256
	}
	small := scaleDown(img, maxDim)

	var buf bytes.Buffer
	if sticker {
		err = png.Encode(&buf, small)
	} else {
		err = jpeg.Encode(&buf, small, &jpeg.Options{Quality: 88})
	}
	if err != nil {
		return
	}
	a.setThumb(chat, e.Info.ID, buf.Bytes())
	a.notify()
}

func (a *App) logf(format string, args ...any) {
	f, err := os.OpenFile(filepath.Join(a.dir, "wa-neo.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, time.Now().Format("15:04:05")+" "+format+"\n", args...)
}

func (a *App) mediaPath(chat, id string) string {
	name := strings.NewReplacer("@", "_", ":", "_", "/", "_").Replace(chat + "_" + id)
	return filepath.Join(a.dir, "media", name)
}

func (a *App) saveOriginal(chat, id string, data []byte) {
	ext := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp", "image/gif": ".gif"}[http.DetectContentType(data)]
	if ext == "" {
		return
	}
	path := a.mediaPath(chat, id) + ext
	os.MkdirAll(filepath.Dir(path), 0o700)
	os.WriteFile(path, data, 0o600)
}

func (a *App) OpenMedia(chat, id string) error {
	matches, _ := filepath.Glob(a.mediaPath(chat, id) + ".*")
	if len(matches) == 0 {
		var thumb []byte
		a.db.QueryRow(`SELECT thumb FROM messages WHERE chat = ? AND id = ?`, chat, id).Scan(&thumb)
		if len(thumb) == 0 {
			return errors.New("foto tidak tersedia")
		}
		ext := ".jpg"
		if http.DetectContentType(thumb) == "image/png" {
			ext = ".png"
		}
		path := a.mediaPath(chat, id) + "-kecil" + ext
		os.MkdirAll(filepath.Dir(path), 0o700)
		if err := os.WriteFile(path, thumb, 0o600); err != nil {
			return err
		}
		matches = []string{path}
	}
	return exec.Command("open", matches[0]).Start()
}
