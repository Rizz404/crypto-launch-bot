# QA browser dashboard managed — 20 Oktober 2026

Sesi Claude melanjutkan [handover](../handover-agent.md). Hanya fixture
`TestDashboardPreview` (memori, tanpa runtime/feed/DB/exchange/`.env`); tidak ada
`bot run`, deploy atau trial. Preview dihentikan setelah QA.

## Cara

Firefox headless lokal (`--screenshot`) pada `127.0.0.1:18082`:

- Desktop 1280 px dan mobile 390 px, light dan dark. Dark dipaksa lewat profil
  sementara (`ui.systemUsesDarkTheme=1`); `@media (prefers-color-scheme)` terpakai.
- Semua `<details>` dibuka (salinan HTML dengan `open`) untuk memeriksa form
  risiko, template, transfer dan tabel strategi pada mobile dark.
- Aksi form nyata lewat HTTP: pause instance `a`, flatten tanpa konfirmasi,
  CSRF salah. Screenshot hanya di scratchpad sesi, tidak masuk Git.

## Hasil

| Area | Hasil |
|---|---|
| Layout desktop/mobile | Kartu, form dan tabel tidak melewati lebar 390 px; tabel strategi di dalam `.scroll` |
| Light/dark | Latar, kartu, input dan teks kecil terbaca pada kedua tema |
| Kontrol scoped | Pause `a` → 303, hanya `a` paused, `b` tetap running, riwayat mencatat `pause` |
| Konfirmasi | Flatten tanpa konfirmasi ditolak, state tidak berubah |
| CSRF | Token salah → 403 |

## Temuan dan perbaikan

| Temuan | Perbaikan |
|---|---|
| Waktu dirender format Go mentah (`… 14:49:11.129824675 +0000 UTC`); reconciliation yang belum pernah terjadi tampil `0001-01-01` | Helper `timestamp`: `YYYY-MM-DD HH:MM:SS UTC`, nol → “belum ada”, sesi berjalan → “berjalan” |
| Grafik memakai indeks sampel sebagai sumbu X, jadi jam tanpa sampel (bot mati) tidak terlihat; SVG ter-letterbox ±75% lebar kartu; tidak ada skala | X mengikuti waktu sampel, `preserveAspectRatio="none"` + stroke tidak terdistorsi, label rentang nilai dan waktu |
| `RateLine` (perkiraan USD/IDR) diisi `cmd/bot` tetapi tidak pernah dirender | Ditampilkan di bawah baris akun |
| Penolakan aksi form menghasilkan halaman teks polos 400/409 tanpa jalan kembali; `?error=` dibaca tetapi tidak dirender | Form diarahkan kembali (303) dengan alasan di `role="alert"`; klien JSON tetap mendapat status 409 |

Regression: `TestDashboardChartAndTimes`, `TestDashboardFormRejectionAndRate`
(`internal/managed/managed_test.go`). Setelah perbaikan: `go test -race ./...`,
`golangci-lint run` 0 issues, `go vet -tags integration ./...`, `make build` lulus.
Integration PostgreSQL tidak dijalankan ulang karena perubahan hanya HTTP/template.

## Batas

- Firefox headless, bukan Chrome/Safari atau perangkat mobile fisik; tidak ada
  pemeriksaan screen reader.
- Fixture tidak berisi eksekusi, attention atau order pending, jadi tabel
  eksekusi dan banner attention hanya diperiksa lewat template/test.
- Basic Auth tidak aktif pada preview; auth tetap dicakup `TestHTTPAuthCSRFAndEscaping`.
- Telegram scoped/whitelist/digest/shutdown bukan bagian QA browser dan tetap P2-9.
