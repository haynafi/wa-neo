# wa-neo

**English** | [Bahasa Indonesia](README.id.md)

A WhatsApp client for the terminal with a neo-brutalist look. It runs on your Mac. There is no server in the middle. Your login and chat history are saved on your own computer in `~/Library/Application Support/wa-neo`.

Built with [whatsmeow](https://github.com/tulir/whatsmeow) and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

> This is an unofficial client. It is not made by or connected to WhatsApp. WhatsApp does not officially allow it, so your account could be limited or banned. Use it with care. Do not send bulk messages. It is a good idea to try it with a spare number first.

## Features

- Chat list with profile photos (pixel style), unread counts, and message previews
- Read and send text messages, including groups
- Photos and stickers show in the chat. Click a photo to open it in Preview
- Send images by pasting (`Cmd+V`), dragging a file into the terminal, or using `/img`
- macOS notifications for new messages
- In iTerm2, photos and the header picture are sharp. In other terminals they are shown as colored blocks

## Install (macOS)

1. Download the file for your Mac from the [Releases](../../releases) page:
   - Apple Silicon (M1, M2, M3, M4): `wa-neo_<version>_macos_arm64.tar.gz`
   - Intel: `wa-neo_<version>_macos_amd64.tar.gz`
2. Open Terminal and run:

```bash
tar xzf wa-neo_*_macos_*.tar.gz
xattr -d com.apple.quarantine wa-neo
./wa-neo
```

The `xattr` step is needed because the app is not signed by Apple. Without it, macOS will block the app.

We recommend [iTerm2](https://iterm2.com) with a big window (at least about 110 columns by 40 rows).

## How to use

The first time you run it, scan the QR code with your phone: **WhatsApp > Linked devices > Link a device**.

| What you want | How |
| --- | --- |
| Pick a chat | Click it, or use `↑` `↓` |
| Send a message | Type, then press `Enter` |
| Close the chat | `Esc` |
| Scroll messages | Mouse wheel, or `PgUp` `PgDn` |
| Start a new chat | `/new 628123456789` |
| Send an image | `Cmd+V` after copying an image, drag a file into the terminal, or `/img ~/photo.jpg caption` |
| Open the original photo | Click the photo |
| Quit | `Ctrl+C` |

Old messages show up after the first sync finishes.

## Settings

You can change some things with environment variables:

| Variable | Value | What it does |
| --- | --- | --- |
| `WA_NEO_IMAGES` | `blocks` | Always use colored blocks, not iTerm2 images |
| `WA_NEO_EMOJI` | `off` | Show emoji as `□` |
| `WA_NEO_NOTIFY` | `off` | Turn off notifications |
| `WA_NEO_DIR` | a folder path | Save data in a different folder |

Example: `WA_NEO_NOTIFY=off ./wa-neo`

## Your data and privacy

All data is saved in `~/Library/Application Support/wa-neo`. The file `session.db` has the keys for your WhatsApp login. Never share it or upload it.

To log out, delete that folder, then remove the linked device on your phone.

## Build it yourself

You need [Go](https://go.dev/dl/).

```bash
go build -o wa-neo .
go test ./...
```

## Make a release

GitHub Actions builds a release when you push a tag that starts with `v`:

```bash
git tag v0.1.0
git push origin v0.1.0
```
