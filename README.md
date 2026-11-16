# wa-neo

Klien WhatsApp di terminal dengan tampilan neo-brutalism. Berjalan lokal di Mac kamu, tanpa server pihak ketiga: sesi dan riwayat chat disimpan di `~/Library/Application Support/wa-neo`.

Dibangun dengan [whatsmeow](https://github.com/tulir/whatsmeow) dan [Bubble Tea](https://github.com/charmbracelet/bubbletea).

> Ini klien tidak resmi dan tidak berafiliasi dengan WhatsApp. WhatsApp tidak mengizinkannya secara resmi, jadi ada risiko akun dibatasi. Pakai dengan wajar, jangan untuk kirim massal, dan sebaiknya coba dulu dengan nomor sekunder.

## Fitur

- Daftar chat dengan foto profil (pixel), lencana pesan belum dibaca, dan pratinjau pesan
- Baca dan kirim pesan teks, termasuk grup
- Foto dan stiker tampil di chat, klik foto untuk membukanya di Preview
- Kirim gambar lewat paste (`Cmd+V`), drag file ke terminal, atau `/img`
- Notifikasi macOS untuk pesan masuk
- Di iTerm2, foto dan avatar header tampil tajam. Di terminal lain tampil sebagai blok warna

## Pasang (macOS)

1. Unduh `wa-neo_<versi>_macos_arm64.tar.gz` (Apple Silicon) atau `..._amd64.tar.gz` (Intel) dari halaman [Releases](../../releases).
2. Buka terminal lalu jalankan:

```bash
tar xzf wa-neo_*_macos_*.tar.gz
xattr -d com.apple.quarantine wa-neo
./wa-neo
```

Langkah `xattr` perlu karena binary belum ditandatangani Apple, jadi Gatekeeper akan memblokirnya tanpa langkah itu.

Disarankan memakai [iTerm2](https://iterm2.com) dengan jendela besar (minimal sekitar 110×40).

## Pemakaian

Pertama kali: scan QR di HP lewat **WhatsApp > Perangkat tertaut > Tautkan perangkat**.

| Aksi | Cara |
| --- | --- |
| Pilih chat | klik, atau `↑` `↓` |
| Kirim pesan | ketik lalu `Enter` |
| Tutup chat | `Esc` |
| Scroll pesan | scroll mouse, atau `PgUp` `PgDn` |
| Chat baru | `/new 628123456789` |
| Kirim gambar | `Cmd+V` setelah menyalin gambar, drag file ke terminal, atau `/img ~/foto.jpg caption` |
| Buka foto asli | klik foto |
| Keluar | `Ctrl+C` |

Riwayat lama muncul setelah sinkronisasi pertama selesai.

## Pengaturan

Diatur lewat variabel lingkungan:

| Variabel | Nilai | Fungsi |
| --- | --- | --- |
| `WA_NEO_IMAGES` | `blocks` | Paksa gambar blok warna, tanpa protokol gambar iTerm2 |
| `WA_NEO_EMOJI` | `off` | Tampilkan emoji sebagai `□` |
| `WA_NEO_NOTIFY` | `off` | Matikan notifikasi |
| `WA_NEO_DIR` | folder | Pakai folder data lain |

Contoh: `WA_NEO_NOTIFY=off ./wa-neo`

## Data dan privasi

Semua data ada di `~/Library/Application Support/wa-neo`. Folder `session.db` berisi kunci sesi WhatsApp kamu, jadi jangan dibagikan atau diunggah. Untuk keluar dari akun, hapus folder itu lalu hapus perangkat tertaut di HP.

## Build sendiri

Butuh [Go](https://go.dev/dl/).

```bash
go build -o wa-neo .
go test ./...
```

## Membuat rilis

Rilis dibuat otomatis oleh GitHub Actions saat tag `v*` di-push:

```bash
git tag v0.1.0
git push origin v0.1.0
```
