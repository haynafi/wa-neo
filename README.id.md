# wa-neo

[English](README.md) | **Bahasa Indonesia**

Klien WhatsApp di terminal dengan tampilan neo-brutalism. Berjalan lokal di Mac kamu, tanpa server pihak ketiga. Sesi login dan riwayat chat disimpan di komputermu sendiri, di `~/Library/Application Support/wa-neo`.

Dibangun dengan [whatsmeow](https://github.com/tulir/whatsmeow) dan [Bubble Tea](https://github.com/charmbracelet/bubbletea).

> Ini klien tidak resmi dan tidak berafiliasi dengan WhatsApp. WhatsApp tidak mengizinkannya secara resmi, jadi ada risiko akun dibatasi atau diblokir. Pakai dengan wajar, jangan untuk kirim pesan massal, dan sebaiknya coba dulu dengan nomor sekunder.

## Fitur

- Daftar chat dengan foto profil (gaya pixel), lencana pesan belum dibaca, dan pratinjau pesan
- Baca dan kirim pesan teks, termasuk grup
- Foto dan stiker tampil di chat. Klik foto untuk membukanya di Preview
- Kirim gambar lewat paste (`Cmd+V`), drag file ke terminal, atau perintah `/img`
- Notifikasi macOS untuk pesan masuk
- Di iTerm2, foto dan gambar header tampil tajam. Di terminal lain tampil sebagai blok warna

## Pasang (macOS)

1. Unduh file yang sesuai dengan Mac kamu dari halaman [Releases](../../releases):
   - Apple Silicon (M1, M2, M3, M4): `wa-neo_<versi>_macos_arm64.tar.gz`
   - Intel: `wa-neo_<versi>_macos_amd64.tar.gz`
2. Buka Terminal lalu jalankan:

```bash
tar xzf wa-neo_*_macos_*.tar.gz
xattr -d com.apple.quarantine wa-neo
./wa-neo
```

Langkah `xattr` perlu karena aplikasi belum ditandatangani Apple. Tanpa langkah itu, macOS akan memblokirnya.

Disarankan memakai [iTerm2](https://iterm2.com) dengan jendela besar (minimal sekitar 110 kolom × 40 baris).

## Cara pakai

Saat pertama dijalankan, scan QR dengan HP kamu: **WhatsApp > Perangkat tertaut > Tautkan perangkat**.

| Yang ingin dilakukan | Caranya |
| --- | --- |
| Pilih chat | Klik, atau `↑` `↓` |
| Kirim pesan | Ketik lalu `Enter` |
| Tutup chat | `Esc` |
| Scroll pesan | Scroll mouse, atau `PgUp` `PgDn` |
| Mulai chat baru | `/new 628123456789` |
| Kirim gambar | `Cmd+V` setelah menyalin gambar, drag file ke terminal, atau `/img ~/foto.jpg caption` |
| Buka foto asli | Klik foto |
| Keluar | `Ctrl+C` |

Riwayat lama muncul setelah sinkronisasi pertama selesai.

## Pengaturan

Beberapa hal bisa diatur lewat variabel lingkungan:

| Variabel | Nilai | Fungsi |
| --- | --- | --- |
| `WA_NEO_IMAGES` | `blocks` | Selalu pakai blok warna, tanpa gambar iTerm2 |
| `WA_NEO_EMOJI` | `off` | Tampilkan emoji sebagai `□` |
| `WA_NEO_NOTIFY` | `off` | Matikan notifikasi |
| `WA_NEO_DIR` | path folder | Simpan data di folder lain |

Contoh: `WA_NEO_NOTIFY=off ./wa-neo`

## Data dan privasi

Semua data disimpan di `~/Library/Application Support/wa-neo`. File `session.db` berisi kunci login WhatsApp kamu. Jangan pernah dibagikan atau diunggah.

Untuk keluar dari akun, hapus folder itu, lalu hapus perangkat tertaut di HP.

## Build sendiri

Butuh [Go](https://go.dev/dl/).

```bash
go build -o wa-neo .
go test ./...
```

## Membuat rilis

GitHub Actions membuat rilis otomatis saat kamu push tag yang diawali `v`:

```bash
git tag v0.1.0
git push origin v0.1.0
```
