# Runtime managed

Panduan konfigurasi dan kontrol multi-instance Binance Spot. Status deployment
di [handover VPS](handover-vps.md), desain di [multi-instance](multi-instance.md).
Trial ETH sudah dihentikan; contoh perintah di sini bukan izin memulainya lagi.

## Konfigurasi dan modal

`wallets[] → instances[] → strategies[]` menggantikan daftar legacy; jangan
mengaktifkan keduanya bersamaan. Satu wallet Spot USDT per akun/mode. Secret
lewat environment, bukan YAML/state/laporan; paper tidak perlu key trading.

- `configs/managed.example.yaml`: demo satu modal dibagi 70/25 USDT dan cadangan,
  bukan keputusan alokasi strategi.
- `configs/managed.eth-paper.yaml`: profil historis ETH-only 110,79104808331486
  USDT total, risk 3%, posisi 75%, daily kill 8%, drawdown 30%, leverage 1.
  Parameter lengkap di [Strategi](strategy.md); arah terbaru di [Strategi 2](strategy-2.md).
- `configs/managed.research-stage-one.yaml`: trial aktif di VPS, tiga instance
  Tren Rp500 ribu/2 juta/5 juta virtual; masing-masing BTC/ETH momentum 4h dan
  SOL pullback 4h. Modal dibagi sekali, sembilan binding tidak berarti sembilan
  saldo baru. Risk 1%, posisi maksimal 25%, satu posisi per instance. T0 dan
  hash deployment tercatat di [handover VPS](handover-vps.md).
- `allocation` instance berasal modal wallet. Binding berbagi cash; `cap`
  opsional mencakup inventori dan reserved buy, tanpa saldo baru.
- Default risk berlaku jika blok dihilangkan; blok nonzero harus valid lengkap.
  Batas parent tetap berlaku. Transfer/setoran bukan profit.
- Bootstrap config diberi hash. Jangan edit file lama lalu resume untuk mengganti
  eksperimen; gunakan kontrol terjurnal atau DB/wallet baru yang direview.

## Kontrol

Scope `account` berarti semua instance terdaftar; scope lain ID instance atau
`instance/binding`. Dashboard mendukung create/clone, rename, binding, cap/risk,
transfer, lifecycle dan preferensi notifikasi.

| Aksi | Perilaku |
|---|---|
| Pause | Tahan entry; exit/proteksi lanjut |
| Resume | Hanya jika state sehat dan engine cocok dengan konfigurasi |
| Cancel | Batalkan entry/raw pending, tanpa menjual posisi |
| Flatten | Cancel proteksi, jual inventori scope; tunggu konfirmasi |
| Finish retain / flatten | Selesaikan order; simpan / tutup inventori |
| Kill/reset | Halt sticky; reset tidak menghapus selisih saldo |
| Archive | Order/posisi termasuk dust kosong, cash sudah ke reserve |
| Stop proses | Persist untuk resume; order/koin tetap dapat ada |

Aksi dana/finish memerlukan konfirmasi; closing belum berarti selesai.
Create/clone mulai paused tanpa modal/riwayat. Binding baru/edit memerlukan
restart agar feed/indikator siap; risk/cap bisa berubah langsung. Transfer hanya
cash bebas. Tidak ada penghapusan dust atau koreksi saldo paksa.

Web memakai Basic Auth, CSRF, origin check, CSP dan escaping. Bind loopback;
gunakan HTTPS bila diproxy. JSON `POST /control?wallet=ID` memakai
`X-CSRF-Token`, schema `Command` di `internal/managed/control.go`, dan ID unik.
`GET /api/state` berisi ledger/run tanpa credentials, tetap harus dilindungi.

Telegram memakai satu long-poller, whitelist, notifikasi info/warn/error/off per
instance, dedup per tingkat peringatan selama satu menit dan menu `/`.
Kode lokal kini melaporkan setiap eksekusi beli/jual setelah checkpoint engine
dan acknowledgement inbox tersimpan; notifikasi transaksi memerlukan level
`info`. Pesan mencantumkan akun/binding, jumlah eksekusi (termasuk parsial),
komisi, sisa posisi dan waktu. Penjualan memakai hasil realisasi ledger,
bukan menghitung ulang dari posisi engine. MM tetap hanya muncul di ringkasan.
Ringkasan harian pukul 07.00 WIB mencakup eksekusi periode 07.00–07.00 WIB,
realisasi pada eksekusi, komisi, PnL sejak awal dan status terkini. Komisi yang
belum diketahui atau dibayar dengan aset ketiga ditandai belum lengkap pada
data eksekusi; penilaian terlambat mengikuti total ledger sejak awal.
Notifikasi bersifat best effort: crash antara commit dan pengiriman atau
antrean Telegram penuh dapat menghilangkan pesan. Riwayat yang sudah selesai
tidak dikirim ulang saat restart. Perubahan ini belum menyatakan sudah dirilis
ke VPS; lihat checkpoint deployment pada handover.

Contoh format:

```text
/pause wallet instance
/flatten wallet instance/binding confirm
/finish wallet instance retain confirm
/resolve wallet order confirm
/stop confirm
```

Command laporan yang tersedia: `/status`, `/balance`, `/pnl`, `/positions`,
`/orders`, `/strategies`. Semua menerima `[akun] [sasaran] [halaman]`;
tanpa argumen menampilkan ringkasan semua wallet, misalnya `/pnl eth-research momentum`
atau `/orders eth-research momentum/eth4h`. `/balance` pada scope strategi tetap
menamai saldo sebagai pool instance, bukan modal bebas eksklusif strategi.
`/pnl` adalah kumulatif ledger sejak eksperimen, termasuk sebelum restart;
bukan ringkasan hari ini. Fee sudah diperhitungkan, tidak dikurangi dua kali.

Respons memakai bahasa Indonesia: nilai modal, uang tersedia, dana untuk order,
hasil transaksi tercatat dan perubahan nilai posisi terbuka. Kurs tersedia ditampilkan
sebagai perkiraan USD/rupiah; kurs yang belum tersedia tidak ditebak. `/status`
membedakan proses aktif dari izin entry menurut pengaturan; izin bukan bukti
sinyal sudah muncul atau koneksi sehat. Ringkasan akun `/pnl` menghitung semua
strategi tanpa mencetak seluruh detail. Daftar posisi/order/strategi dan rincian
PnL maksimal lima item per halaman, dengan jumlah total dan perintah halaman
berikutnya, misalnya `/strategies eth-research account 2`. Total PnL tetap
menghitung seluruh sasaran, bukan hanya lima item yang ditampilkan.
Balasan kontrol menyebut akun, sasaran dan efek tindakan. Permintaan cancel/
flatten/finish bukan bukti order sudah selesai; `/stop` mengakui permintaan
shutdown, bukan mengklaim proses sudah mati sebelum menyimpan state.

`/killswitch` adalah alias `/kill akun scope confirm`. Seluruh command managed,
termasuk `/help` dan `/start`, didaftarkan ke suggestion `/` untuk chat whitelist
melalui `setMyCommands` saat bot mulai. Perbaikan command 21 Oktober masih lokal;
belum dipublikasikan ke Telegram/VPS dan tidak menjadi izin resume trial lama.

### Uji format Telegram dari lokal

Arahan user **21 Oktober 2026**: selama belum live, env lokal dan VPS menurut
user masih sama; **gunakan `.env` lokal untuk pengujian Telegram**, tidak perlu
SSH untuk mengambil kredensial/mengirim contoh. Token lokal sudah diperiksa
tersedia tanpa mencetak nilainya; kesamaan seluruh isi env tidak dibandingkan.
Ini kondisi fase sekarang, bukan janji env selalu sama setelah live. Jangan
menyalin/sinkronisasi env sebagai bagian pengujian format.

Izin lokal ini khusus pengujian pesan, bukan menjalankan `bot run`, feed harga,
strategi atau trial di laptop. Gunakan data fixture di memori dan formatter yang
sama, lalu `sendMessage` ke chat user yang sudah diverifikasi. Jangan menjalankan
`getUpdates`, kontrol trading, mengubah menu aktif, mengirim order exchange atau
membuka database eksperimen. Labeli setiap pesan **DEMO — data simulasi, bukan
kondisi akun sebenarnya**; contoh start/stop/kill/penjualan bukan tindakan nyata.

Ekspor contoh tanpa koneksi Telegram/database/exchange:

```sh
TB_PREVIEW_OUTPUT=/tmp/trading-bot-telegram-preview.json go test ./cmd/bot -run '^TestTelegramPreviewExport$' -v
```

Ekspor mencakup laporan/rincian, halaman, kontrol, pesan kosong/error, start/stop
dan format notifikasi pendukung; tidak mengirim pesan dan tidak membuktikan semua
notifikasi terpasang di runtime managed. Pengiriman terpisah membaca token env
ke memori tanpa mencetak token/URL bertoken, memakai HTML, notifikasi senyap dan
jeda minimal 1,2 detik antarpesan. Catat acknowledgment API per nomor contoh;
jangan retry buta pada timeout yang hasilnya belum pasti.

**Hasil demo sesi ini:** 32 contoh dikonfirmasi diterima API Telegram. Sebelas
terkirim lewat VPS sebelum arahan lokal diterima, lalu koneksi pengirim SSH
dihentikan dan 21 sisanya dikirim dari `.env` lokal. Satu pesan yang sedang
dikirim saat pemutusan mungkin diterima tanpa acknowledgment yang terlihat;
duplikat pada batas peralihan tidak dapat dikesampingkan. Tidak ada trial dimulai
ulang atau menu suggestion nyata diubah. Penerimaan API bukan bukti user sudah
membaca pesan atau memverifikasi tampilannya di aplikasi Telegram.

Demo lanjutan memakai komposisi [rancangan strategi](strategy-2.md#portofolio-dan-target-paper):
24 instance, 72 binding dan empat modal (Rp500 ribu/2 juta/5 juta/10 juta),
total Rp105 juta virtual. Enam contoh detail mewakili tren, breakout, sideways,
pullback, rotasi dan campuran. Rotasi/squeeze/mean reversion diberi status
belum diimplementasikan; tidak diganti dengan strategi lain yang seolah aktif.
Ekspor 17 contoh dengan:

```sh
TB_STRATEGY_PREVIEW_OUTPUT=/tmp/trading-bot-telegram-strategies-preview.json go test ./cmd/bot -run '^TestTelegramStrategyPlanPreviewExport$' -v
```

Fixture hanya membuat `managed.State` untuk formatter, tanpa Runtime/feed atau
evaluasi sinyal. Alur entry/holding/exit ETH momentum 4h memakai angka sintetis,
komisi 0,1% per sisi, tanpa slippage pada ilustrasi; ini bukan model eksekusi
riset atau hasil strategi. Kurs Rp18.052 juga asumsi demo. Assertion memeriksa
perubahan equity setelah alur tersebut sama dengan PnL bersih 0,919 USDT.
Seluruh 17 contoh dikonfirmasi diterima API Telegram dari pengiriman lokal;
belum ada verifikasi visual dari aplikasi user atau eksekusi strategi nyata.

Revisi setelah user meninjau salinan pesan: pisahkan strategi belum tersedia
dari strategi dijeda (ketersediaan dari registry kode), tampilkan nama strategi
yang mudah dibaca, ringkas detail PnL nol tanpa menghapus total, pertahankan
presisi komisi, dan sertakan waktu laporan/mulai eksperimen/candle harga.
Waktu candle adalah waktu yang disimpan ledger, bukan klaim tick terbaru;
jika belum ada timestamp, tampilkan `belum tercatat`. Stop loss memakai istilah
perkiraan kerugian di harga stop, sebelum komisi/selisih eksekusi, tanpa jaminan
rugi maksimum. Persentase hasil posisi memakai nilai pembelian; dampak satu
transaksi memakai modal acuan instance bila tersedia dan disebutkan.

Jalur notifikasi legacy tidak membawa modal acuan instance dan trade hanya
membawa tag exit umum. Karena itu respons menandai denominator/alasan rinci
yang belum tersedia, bukan menebaknya. Demo menyediakan modal acuan Rp2 juta
sehingga hasil posisi +2,30% dan dampak +0,83% dapat dibandingkan. Alasan exit
sinyal pada demo tetap ilustrasi, bukan bukti alasan rinci sudah tersimpan
dalam trade runtime. Demo revisi memakai 17 skenario dan angka yang sama.
Seluruh 17 pesan revisi dikonfirmasi diterima API Telegram dari `.env` lokal;
label `DEMO STRATEGI REVISI` membedakannya dari pengiriman sebelumnya. Tes
paket bot/Telegram dengan race detector dan ekspor fixture revisi lulus.

Digest dijadwalkan 00.00 UTC/07.00 WIB; jadwal bukan bukti pesan diterima.
Mark ledger candle berjalan dibatasi sekali/10 detik/simbol; candle tutup,
fill/order/kontrol langsung dipersist. Engine memakai harga terbaru di memori.

## Backtest dan preview

Jalankan hanya saat pekerjaan pengujian telah diminta. CLI `portfolio-backtest`
tidak membuka DB/exchange/`.env`; URL dummy hanya memenuhi validasi config:

```sh
TB_DATABASE__URL=postgres://unused@localhost/unused ./bin/portfolio-backtest -config configs/managed.example.yaml -data /path/data -from 2024-01-01 -to 2024-01-03 -out var/managed-backtest/result.json
```

Data `SYMBOL-1m.csv`: timestamp UTC detik/ms/µs, OHLCV, header opsional.
`instruments.json` adalah array `domain.Instrument`, bukan raw `exchangeInfo`.
CSV digabung kronologis; data sebelum `from` hanya warmup. Gap/duplikat ditolak;
`-allow-gaps` membuang agregat tidak lengkap. Catat jika rules instrumen sekarang
menggantikan snapshot historis.

Baseline `-fee 0.001 -slippage-bps 2`; stress misalnya 0.002/10. Laporan berisi
hash data/config, satu modal total, return/drawdown, turnover/fee, estimasi exit,
benchmark tunai/buy-and-hold dan ownership. Buy-and-hold equal-weight penuh
berbeda exposure dari strategi. Model OHLC tanpa queue/latensi/market impact
bukan holdout baru atau bukti profit live.

Preview `TB_DASHBOARD_PREVIEW=127.0.0.1:18082 go test ./internal/managed -run '^TestDashboardPreview$' -v -timeout 10m`
hanya fixture memori, tanpa Runtime/feed/DB/`.env`; hentikan setelah QA.
Jangan memakai `bot run` di laptop. [Hasil QA historis](testing/plan-2-browser-qa.md).

## Migrasi dan recovery

Migrasi 4/5 diuji pada DB salinan dahulu. Wallet memakai row unik dan advisory
lease dedicated connection; pool minimal jumlah wallet + 2. Kehilangan lease
menahan I/O. Binary legacy lama tidak mengenal lock: jangan paralelkan worker.
Legacy import tetap `review`, tanpa adopsi saldo/inventori otomatis.

Putus DB/network atau mismatch aset menahan entry. Restart memulihkan checkpoint,
inbox dan reservasi, lalu reconciliation. Jangan reset attention untuk memaksa
entry; pulihkan trade ID/komisi asli. Fee aset ketiga menahan entry sampai harga
valuasinya tersedia. Transaksi manual/deposit/withdrawal harus ditelusuri.

**Order tak pasti:** setelah restart dan minimal lima menit sejak order dibuat,
pilih `resolve_order` dengan konfirmasi/ID unik. Exchange `-2013` melepas
reservasi dan memberi tahu engine; order open dilacak kembali; status terminal
memulihkan fill dari `myTrades`. Query gagal tetap unknown. Attention dihitung
ulang dari reconciliation, bukan dihapus langsung.

**Kompaksi:** tiap jam, order terminal/eksekusi/audit lebih dari tujuh hari
berpindah ke `managed_archive` secara atomik. Order belum selesai, saudara OCO
terbuka, inbox dan fee pending tetap di state. Agregat finansial dipertahankan;
unique journal request ID menolak replay lama. Dashboard/API hanya tujuh hari
terakhir; history lengkap di archive/journal. Grafik ringkas maksimal 720 jam.

**Penutupan:** finish dengan pilihan retain/flatten, verifikasi terminal dan
saldo, simpan DB/jurnal/config hash/waktu serta bukti exchange, baru tutup proses.
Stop proses tidak membatalkan order. Bukti historis jangan ditimpa.

**Rollback:** stop worker, backup schema/state, selesaikan order managed dan
periksa kompatibilitas binary. Jangan down migrasi 4/5 pada DB berisi dana/
history; down/up hanya DB test disposable.
