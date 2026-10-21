# Plan 1 — Selesai dan diarsipkan

**Ditutup 20 Oktober 2026 atas permintaan user. Rencana aktif sekarang adalah
[Plan 2](plan-2.md).** Semua pekerjaan yang belum selesai diteruskan melalui
[daftar pemindahan C-01–C-19](plan-2.md#pemindahan-pekerjaan-dari-plan-1).
Penutupan siklus ini tidak menyatakan seluruh checklist lulus, target profit
tercapai, atau mainnet diizinkan.

## Ringkasan akhir Plan 1

| Area | Hasil akhir |
|---|---|
| Fondasi dan fitur Fase 0–8 | Dibangun; hasil validasi dan keterbatasan tetap pada arsip di bawah |
| Fase 9 | Ditutup untuk cakupan aktif Binance Spot pada 18 Oktober; pengujian tertunda tidak dianggap lulus |
| Fase 10 | Testnet dihentikan user 20 Oktober 18.52 WIB setelah sekitar 43 jam 9 menit; hasil dianalisis, target 72 jam dibatalkan |
| Perbaikan/temuan tersisa | Proteksi OCO, ledger/dust, stop terminal dan validasi lanjutan diteruskan ke Plan 2 |
| Fase 11 live mini | Tidak dijalankan; dipindahkan ke P2-10 dan tetap memerlukan persetujuan eksplisit |
| Futures/Bybit/funding/xspread | Ditunda sesuai dependensi, dipindahkan ke D-01/D-02/D-03 |
| Kandidat strategi | ETH momentum 4h untuk paper, BTC momentum harian sebagai pembanding; belum ada trial baru |
| Kondisi VPS terakhir | Bot mati; observasi nonaktif, DB/DNS/monitoring/backup aktif; detail di handoff |

Dokumen utama berikutnya: [Plan 2](plan-2.md),
[rancangan multi-instance](multi-instance.md),
[keputusan strategi](strategy.md),
[handoff VPS](handover-vps.md), dan
[laporan testnet final](testing/testnet-stop/analysis.md).

## Batas pembacaan arsip

Bagian berikut dipertahankan untuk sumber bukti, keputusan bertanggal, tautan
lama dan konteks implementasi. Checkbox kosong adalah catatan historis, bukan
antrean aktif kedua. Tugas yang sudah diselesaikan belakangan tidak dibuka ulang
hanya karena checkbox lama kosong; lihat C-17 pada Plan 2. Instruksi menunggu
21 Oktober, resume worker riset, trial lokal ETH dan Fase 11 di bagian lama
sudah digantikan status penutupan serta Plan 2.

## Handoff siklus pertama — arsip

> Arsip Plan 1. Status, instruksi fase, jadwal dan checklist di bawah merekam kondisi pada waktunya. Untuk pekerjaan selanjutnya gunakan [Plan 2](plan-2.md); jadwal lama tidak mengaktifkan worker, trial, atau live.

## Status saat ini (2026-10-20)

- **Fase 0–9 selesai.** Fase 9 ditutup 2026-10-18 untuk cakupan aktif (Binance Spot, VPS, Telegram, dashboard, monitoring, backtest/riset); item yang tertunda faktor luar tercatat di "Penutupan Fase 9" dan **tidak** dianggap lolos.
- **Fase 10 dihentikan lebih awal:** testnet VPS 18 Oktober 23.43 WIB–20 Oktober 18.52 WIB, sekitar 43 jam 9 menit; target 21 Oktober dibatalkan. Bot exit 0, tujuh order tersisa dibatalkan, inventori dan backup dipertahankan. [Analisis penutupan](testing/testnet-stop/analysis.md).
- **VPS:** bot testnet mati, timer observasi disabled/inactive; DB, DNS, monitoring dan timer backup tetap aktif. Source terakhir yang diverifikasi di VPS `f709429`, image `sha256:e5b8710bc76da2cd5d581511e19fb7bf9e6a472e0eb61b14fb3c099054fd0438`. Source lokal baru belum dirilis ke VPS. Baca [handoff](handover-vps.md). *(Arsip: sejak 2026-10-20 trial P2-9 berjalan; status terkini di handoff VPS.)*
- **Keputusan strategi berikutnya:** kandidat utama paper **ETHUSDT momentum 4h**, pembanding **BTCUSDT momentum harian**. Aturan, modal, risiko, biaya, alasan dan prasyarat lengkap di [keputusan 20 Oktober](strategy.md). Belum ada trial baru dimulai; dokumentasi bukan izin live.
- **Strategi: belum ada yang layak live.** Emacross (berbagai parameter dan filter tren), DCA, dan grid diuji 3 tahun × 5 pair secara walk-forward; tidak ada edge konsisten. Di paper, fee = 84% kerugian emacross 1m. Testnet tanpa fee, jadi hasil untung/rugi testnet **bukan** dasar go live.
- **Riset lokal 2026-10-19:** backtest Spot asli 1 menit, 2025-10-19–2026-10-19 UTC, BTC/ETH/BNB/XRP/SOL, modal **Rp2 juta per kombinasi**. Baseline empat strategi: 20 kasus semuanya rugi. Atas permintaan user, **maksimal lima kandidat** dikunci sebelum diuji (tanpa tuning ulang): EMA 4h, EMA harian, breakout harian, momentum harian, pullback 4h. Sebanyak 25 kasus selesai; rata-rata pullback **+1,05%**, positif pada 3/5 koin, negatif pada BNB/SOL; EMA harian tidak trading. Ini eksplorasi pada tahun yang sudah dilihat, **bukan bukti out-of-sample** dan belum layak live. Implementasi baru `trendspot` terdaftar tetapi tidak diaktifkan; konfigurasi aktif/VPS tidak diubah. [Ringkasan dan reproduksi](testing/candidates/summary.txt), [hasil lengkap](testing/candidates/results.json), [spesifikasi lima kandidat](testing/candidates/candidate-spec.json). Langkah riset berikutnya memerlukan periode terpisah yang belum dipakai memilih kandidat, bukan menambah percobaan melewati batas lima tanpa instruksi user.
- **Antrean riset lanjutan 2026-10-19:** user kemudian mengizinkan **banyak strategi/variasi**, target riset **+50% setahun atau lebih**, tetap **Rp2 juta per kombinasi**, dan meminta pekerjaan dijalankan lalu ditinggalkan dengan notifikasi, tanpa pemantauan AI berkala. Ini menggantikan batas lima untuk batch baru. [Plan yang dikunci](testing/research-queue.json): 216 konfigurasi (72 sinyal × 3 profil risk Spot tanpa leverage), maksimum 2.340 pekerjaan. Training dua periode tahunan 2022-10-19–2024-10-19 dengan fill 1h; pilih maksimal 3 per keluarga hanya dari training, validasi tahun berikutnya, konfirmasi tahun 2025–2026 dengan fill 1m dan stress biaya. Konfirmasi sudah pernah dilihat; jangan mengklaim holdout murni atau keuntungan terjamin. Satu worker systemd user `trading-bot-research-20261019t133722z.service`, CPU 150%, RAM 3 GiB, nice 10; code/binary/plan dibekukan. Run dan data besar tersimpan di `var/backtest-research/` (gitignored), run aktif `var/backtest-research/latest` → `run-20261019T133722Z`. Cek **hanya bila user meminta/melaporkan selesai**: `python scripts/backtest-queue/queue.py status --run-dir var/backtest-research/latest`. Hasil akhir `report.txt`, `report.json`, `DONE.txt`; kegagalan `FAILED.txt`; bukti pengiriman notif desktop `notification.json`. Tutup chat aman; suspend menunda komputasi, reboot memerlukan resume lewat `python scripts/backtest-queue/queue.py launch --run-dir var/backtest-research/latest`; checkpoint per pekerjaan dipakai ulang. Jangan menyalakan worker kedua atau men-deploy hasil otomatis. Lint, vet, race test helper, tes seleksi tanpa kebocoran data validasi, dan smoke end-to-end dengan data sintetis lolos sebelum launch.
- **Mainnet (Fase 11) belum disetujui.** Prasyarat: strategi dengan edge (atau keputusan sadar user untuk uji kecil), batas risk dari user, futures diuji bila dipakai.
- Exchange aktif hanya **Binance**. Bybit menunggu kabar user (regulatory restriction); OKX sudah dihapus. Futures Binance menunggu kredensial Futures Demo.

## Panduan melanjutkan (untuk sesi/AI berikutnya)

**Koreksi terbaru — ETH tidak boleh berjalan di laptop (2026-10-19):** user meminta VPS bila tidak mengganggu instance yang ada, atau dokumentasi untuk sesudah testnet selesai. Service lokal ETH dan container DB khususnya sudah dihentikan; data tetap disimpan, start/resume lokal diblokir marker user. Pemeriksaan VPS read-only: baseline `differences: []`, kapasitas tersedia, testnet tetap berjalan. Strategi baru belum menjadi image CI; dipilih fallback dokumentasi, **belum ada deploy ETH di VPS**, belum ada branch/push/restart VPS. [Referensi dan rencana deployment terisolasi](strategy-vps.md). Menunggu akhir tes 21 Oktober 23.43 WIB dan permintaan user selanjutnya; review lokal 26 Oktober dibatalkan. Jangan mengikuti paragraf riwayat berikut untuk menyalakan lagi paper lokal.

**Paper ETH lokal dimulai 2026-10-19 22:37 WIB:** user meminta mencoba rancangan berikutnya. Satu ETHUSDT Spot, momentum028-larger 4h, modal simulasi total Rp2 juta = 110,79104808331486 USDT pada kurs referensi Rp18.052, posisi maks 75%, risk 3%, fee standar 0,1% per sisi dan slippage 2bps. Konfigurasi [paper.eth.yaml](../configs/paper.eth.yaml), helper `python scripts/eth-paper.py status|stop|resume`; service user `trading-bot-eth-paper.service`, CPU 50%, RAM maks 512 MiB; DB/container khusus `trading-bot-eth-paper-db` port 55434, database `ethpaper_trial`, volume terpisah. Dashboard http://127.0.0.1:18080; metrics 19100. Binary/config/saldo awal dibekukan di `var/paper-eth/manifest.json`; tidak memuat `.env`, kredensial exchange, atau Telegram. Desktop notification bila service berhenti (bukan notifikasi bahwa strategi sudah terbukti). Bootstrap tanpa order/trade 22:35–22:37 dipertahankan di DB `ethpaper` dan `bootstrap-manifest.json`, dikeluarkan dari hasil trial; koreksi pembulatan saldo sekitar Rp10 dilakukan sebelum trial final. Sumber harga mainnet publik, seluruh order simulasi; tidak menyentuh VPS. Laptop harus menyala, terhubung internet, dan tidak suspend; layanan bertahan setelah chat ditutup, reboot perlu `resume`. Review operasional pertama setelah 7 hari (26 Oktober malam), hanya saat user meminta; jangan polling AI. Strategi sekitar 3 transaksi tutup per bulan di backtest, maka 7/30 hari belum cukup membuktikan target 50% setahun. Risk ini hanya paper, mainnet belum disetujui. Jangan mengubah parameter, mereset saldo, atau menyalakan eksperimen kedua diam-diam.

**Sensitivitas biaya 2026-10-19:** user menduga biaya ETH menyebabkan kerugian dan meminta mencoba koin berbiaya murah. Diluruskan bahwa simulasi Binance Spot tidak membayar gas jaringan ETH; semua lima pair sebelumnya memakai tarif sama 0,1% per sisi. Tarif publik regular dan skenario diskon BNB 0,075% diverifikasi dari [Binance](https://www.binance.com/en/fee/trading). Uji lima pair dengan `momentum-028-aggressive`, tahun 2025-10-19–2026-10-19, modal Rp2 juta/case, hanya fee berubah: BTC −4,09%→−2,95%; ETH +10,44%→+12,30%; BNB −9,27%→−7,54%; XRP −3,52%→−2,10%; SOL −5,74%→−3,99%. Jumlah transaksi sama; penghematan meningkatkan hasil tetapi empat pair tetap rugi. Diskon disimulasikan setara fee dalam quote, belum mengecek kelayakan akun atau memodelkan saldo/harga BNB untuk pembayaran komisi. Tidak ada pembelian BNB/perubahan akun/config. [Hasil](testing/fee-sensitivity/results.json), [ringkasan](testing/fee-sensitivity/summary.txt); reproduksi `python scripts/backtest-queue/fee_sensitivity.py` dengan run validasi sebagai parent.

**Hasil validasi lanjutan 2026-10-19 (selesai 22:08 WIB):** user kembali setelah notifikasi; **360/360 pekerjaan sukses, 0 gagal**, modal Rp2 juta/case dan plan hash diverifikasi. Ada kasus >50%: `momentum-028-aggressive` ETH 2024-10-19–2025-10-19 **+96,58%**, saldo Rp3.931.621; stress biaya +82,23%. Tetapi ETH tahun berikutnya hanya +10,44%, rata-rata lima koin strategi itu −2,43%, stress −16,00%; gagal saringan risiko/konsistensi. Replay SOL 2023–2024 1m +48,49% (sebelumnya 1h +48,53%). **0/12 kandidat memenuhi target +50% pada rata-rata masing-masing dua tahun validasi.** **1/12 lolos saringan awal untuk peninjauan paper**, `momentum-037-larger`: momentum 14 hari positif, harga di atas EMA50 harian, stop/trailing3ATR; risiko3% per trade, posisi cap75%, kill harian8%/drawdown30% (ini profil simulasi, bukan batas live yang disetujui user). Rata-rata lima case independen 2024–2025 +1,41%, 2025–2026 +3,40%; stress +0,18% dan +2,29%; penurunan terburuk termasuk stress11,72%; 177 penutupan normal dua tahun, 7/10 coin-year positif. Saringan yang dibuat adalah heuristik, bukan bukti statistik; target user tetap belum terpenuhi. Tidak ada paper/live otomatis dimulai oleh worker. Selanjutnya perlu meninjau apakah layak menjalankan paper prospektif kandidat kecil ini atau meriset pendekatan lain; jangan mengubah gate agar pemenang agresif terlihat lolos atau men-deploy ke VPS tanpa alur yang disepakati. [Hasil lengkap](testing/high-return-validation-results.json), [audit/batas interpretasi](testing/high-return-validation-analysis.json); narasi lokal `var/backtest-research/latest/analysis.txt`. Batch selesai, tidak perlu polling/restart.

**Batch validasi lanjutan 2026-10-19:** user meminta terus meneliti sampai ada bukti untuk mempertimbangkan uang sungguhan. Ini izin riset lanjutan, **bukan izin transaksi live** dan bukan alasan mengubah kriteria sampai terlihat lolos. Batch baru dibekukan: enam sinyal dengan rata-rata keuntungan training agresif tertinggi (`momentum-028`, `momentum-027`, `momentum-037`, `momentum-041`, `momentum-039`, `ema-009`), masing-masing profil `larger`/`aggressive`: 12 konfigurasi tanpa parameter baru. **360 pekerjaan**, lima koin, Rp2 juta per kasus, replay **1m** tiga periode 2023-10-19–2026-10-19, masing-masing normal dan stress biaya. Tambahan data minute 2023-10–2025-10 diunduh dengan dua koneksi dan checksum; gap tak dikenal tetap menghentikan proses. Patokan riset dikunci di [plan](testing/high-return-validation-plan.json): kedua tahun validasi harus rata-rata untung normal dan stress, >=7/10 kasus coin-year positif, >=50 penutupan transaksi, drawdown setiap kasus <=25%, kerugian akhir setiap kasus <=20%. Target +50% tahunan diperiksa terpisah dan harus tercapai pada rata-rata **masing-masing** tahun validasi. Lolos hanya berarti kandidat untuk peninjauan **paper prospektif**, bukan bukti pasti atau persetujuan live; risiko live dari user belum ditetapkan. Tahun pengujian sudah pernah dilihat, jadi jangan klaim holdout murni. Worker dibekukan di `var/backtest-research/validation-20261019T141536Z`, service `trading-bot-research-validation-20261019t141536z.service`, batas daya sama (1 worker, CPU150%, RAM3GiB). `latest` menunjuk batch ini; `last-screening` menunjuk batch 2.340 sebelumnya. Notifikasi desktop selesai/gagal, checkpoint dan report tetap aktif. Cek hanya saat user kembali: `python scripts/backtest-queue/validation.py status --run-dir var/backtest-research/latest`; resume setelah reboot dengan subcommand `launch`. Tes seleksi tanpa hasil validasi, grid 360 job unik, dan gate gagal/incomplete/risiko berlebihan lolos (7 tes Python total). Tidak ada deployment/VPS/config aktif diubah.

**Hasil antrean riset 2026-10-19 (selesai 21:06 WIB):** user melaporkan selesai, lalu 2.340 hasil unik diverifikasi, semuanya modal Rp2 juta dan tidak ada pekerjaan gagal (2.160 training + 60 validasi + 60 konfirmasi + 60 stress). **Tidak ada kasus mencapai +50%.** Tertinggi `momentum-028-aggressive` pada SOL 2023-10-19–2024-10-19: **+48,53%**, saldo Rp2.970.571, drawdown 19,63%; ETH periode itu +47,15%, tetapi kandidat sama pada ETH tahun sebelumnya −24,21% dan XRP tahun berikutnya −15,04%. Ini pemenang kasus tunggal dari training, **bukan hasil validasi**. Seleksi memberi penalti drawdown/variabilitas, sehingga semua 12 konfigurasi lanjutan adalah profil baseline; varian agresif belum diuji menyeluruh pada tahun berikutnya. Kandidat lanjutan dengan rata-rata konfirmasi tertinggi `ema-009-baseline`: +1,63%, stress +1,48%, masih jauh dari target user. [Hasil lengkap](testing/research-results.json), [audit dan batas interpretasi](testing/research-analysis.json); narasi lokal `var/backtest-research/latest/analysis.txt`. Batch sudah selesai, tidak ada worker yang perlu dipantau atau diluncurkan ulang otomatis. Belum ada strategi disetujui untuk live.

**Perbaikan antrean 2026-10-19:** persiapan data batch riset sempat berhenti sebelum satu pun backtest dimulai karena arsip resmi kelima koin tidak memiliki candle Spot 1h **2023-03-24 13:00 UTC**. Binance saat itu menghentikan Spot sementara dan melanjutkan pukul 14:00 UTC ([pengumuman resmi](https://www.binance.com/en/support/announcement/detail/813a31506e9f478ea8c1058b425df87a)). Helper riset kini hanya mengizinkan jeda spesifik yang terdokumentasi itu, tetap tanpa candle/harga/transaksi sintetis; agregasi memakai batas jam UTC, bukan jumlah bar. Celah lain tetap ditolak. Regression test dan replay satu tahun BTC dari arsip asli lolos, serta lint/vet bersih. Plan/binary/worker lama diarsipkan di `var/backtest-research/latest/revisions/001-before-halt-fix`; plan revisi 2 dibekukan ulang sebelum hasil strategi ada, tanpa mengubah 216 konfigurasi/biaya/risk/seleksi. Service antrean yang sama sudah dijalankan ulang; unduhan besar dipakai ulang. Jangan menyimpulkan notif "terhenti" sebagai hasil strategi atau kehabisan RAM.

### Langkah berikutnya

1. **Setelah 2026-10-21 23:43 WIB: analisis uji testnet** (perintah di bawah). Dua restart terencana pada 2026-10-18 (17:10 dan 17:23 UTC, rilis `69d8dcb` dan `f709429`; run testnet 7 → 8 → 9) bukan crash. Nilai crash/restart/OOM, WARN/ERROR, perilaku tiap strategi (grid: fill dan order balasan; DCA: siklus dan take profit; MM: requote dan inventori; emacross: sinyal + OCO), kill switch/risk limit, rate limit, notifikasi dan ringkasan harian Telegram (07:00 WIB), konsistensi saldo bot vs exchange. Catat hasil di bagian "Fase 10", perbaiki temuan dengan regression test.
2. Diskusikan dengan user: strategi apa (kalau ada) dan batas risk untuk Fase 11. Opsi yang sudah dibahas: riset strategi yang jarang trading (timeframe harian, momentum lintas pair) agar fee tidak dominan, atau live mini modal kecil (50–100 USDT) sebagai uji sistem, bukan untuk profit.
3. Bila user menyiapkan Futures Demo key atau Bybit bisa dipakai: uji seperti Spot (adapter testnet test, bot penuh, rekam fixture).
4. Sebelum live: audit ulang menyeluruh, salinan backup di luar server (keputusan user: setelah live), dan pertimbangan hukum/pajak (Binance diblokir Trust Positif di Indonesia; user sudah diberi tahu).

### Keputusan user (jangan ditanyakan ulang tanpa alasan kuat)

| Tanggal | Keputusan |
|---|---|
| 2026-10-17 | Bangun dulu (Fase 2–8), test di Fase 9, perbaiki di Fase 10, live di Fase 11 dengan persetujuan eksplisit. |
| 2026-10-17 | OKX dihapus. Bybit ditunda (restriction). Fokus Binance. |
| 2026-10-17 | Hosting paper/testnet di VPS `ssh plesk`; rilis lewat GitHub → Actions → GHCR → VPS (digest). Jangan salin folder laptop. Lindungi proyek lain di server. |
| 2026-10-17 | VPS memakai resolver DNS-over-TLS khusus bot (DNS ISP memblokir Binance). |
| 2026-10-17 | Spot live/testnet: bot hanya mengelola koin yang dibelinya sendiri; saldo lain diabaikan dan tidak pernah dijual. |
| 2026-10-17 | Backup di luar server ditunda sampai semua fase selesai dan benar-benar live. |
| 2026-10-18 | Observasi 24 jam dihitung ulang setelah reboot host tak terduga; reboot bukan dari user. |
| 2026-10-18 | Telegram aktif di VPS; chat ID user hanya di config server, bukan repo. Pesan Telegram berbahasa Indonesia dan ramah pemula. |
| 2026-10-18 | Riset strategi dilanjutkan sebelum Fase 11 (belum ada edge). |
| 2026-10-18 | Saham ter-tokenisasi (`bStocks`) dikecualikan dari universe. Futures ditunda sampai ada Futures Demo key. |
| 2026-10-18 | Uji semua strategi spot 3 hari di testnet VPS, lalu analisis sebelum memutuskan go live. |
| 2026-10-19 | Pesan Telegram menampilkan setiap nominal USDT juga dalam USD dan rupiah (kurs publik), agar untung/rugi langsung terbaca. |

### Cek cepat

```sh
# Status stack dan bot (dari laptop; izin SSH Claude terbatas pada project ini)
ssh -o BatchMode=yes plesk 'cd /opt/trading-bot-paper/repo && docker compose --env-file .env.vps -f deploy/compose.vps.yml ps'
ssh -o BatchMode=yes plesk 'cd /opt/trading-bot-paper/repo && python3 deploy/vps-observe.py'
# Order per strategi selama uji testnet
ssh -o BatchMode=yes plesk "cd /opt/trading-bot-paper/repo && docker compose --env-file .env.vps -f deploy/compose.vps.yml exec -T db psql -U bot -d tradingbot -c \"SELECT strategy, side, status, count(*) FROM orders WHERE exchange='binance-testnet' GROUP BY 1,2,3 ORDER BY 1,2,3\""
```

Analisis dan bukti final ada di [laporan testnet](testing/testnet-stop/analysis.md).

### Jebakan kerja yang sudah terjadi (hindari)

- **Format:** `echo "$(gofmt -l .)"` tidak menghentikan rantai `&&`. Gunakan `[ -z "$(gofmt -l .)" ]`; dua kali commit tak terformat lolos ke CI karena ini.
- **Test integrasi** (`-tags integration`, DB `tradingbot_phase9_test` di container lokal `trading-bot_db_1`) wajib dijalankan setelah mengubah strategi/sesi: test paper restart pernah bergantung pada bug DCA.
- **Scratchpad `/tmp` dan container lokal hilang setelah laptop dimatikan.** Nyalakan DB lokal dengan `podman start trading-bot_db_1`.
- **Testnet:** komisi 0 (jalur fee base asset hanya teruji unit/fixture), riwayat candle hanya sekitar 2 minggu (di-reset berkala), akun berisi saldo bawaan (1 BTC, 1 ETH, SOL/XRP/DOGE, 10.000 USDT) yang tidak boleh dianggap posisi bot.
- **Order di bawah minimum exchange** ditolak engine hanya dengan WARN di log: strategi bisa diam-diam tidak trading (contoh: BTC futures 50 USDT minimum).
- **Server Plesk bersama:** hanya sentuh project `trading-bot-paper`; bandingkan baseline 28 container + 5 service sebelum/sesudah perubahan (skrip di handoff VPS). Deploy memakai gerbang DNS: nyalakan `dns` dan cek jawabannya sebelum me-recreate bot.
- **`deploy/vps-set-env.py`** hanya menerima `TB_*` dan `BOT_IMAGE` (yang terakhir sejak `f709429`; sebelumnya ditolak dan deploy `69d8dcb` memakai `sed`).
- **Shared memory Hindsight** sering putus (`Broken pipe`); docs repo adalah handoff yang dapat diandalkan.

## Aturan kerja dari user

- Mesin dev project ini adalah **CachyOS (bukan NixOS)**. Go 1.27.2 dipasang lewat mise (`.mise.toml`), container memakai **podman** + podman-compose. Tidak ada `nix`, jadi `flake.nix` tidak dipakai.
- Jangan menjalankan perintah build/rebuild NixOS yang mengaktifkan konfigurasi (aturan global user).
- Commit: Conventional Commits satu baris tanpa body, `type(scope): deskripsi` (contoh: `feat(binance): add futures kline stream`). Satu commit per scope, diurutkan sesuai alur pengerjaan. Tanpa `Co-Authored-By`. Aturan lengkap ada di `~/.claude/CLAUDE.md`.
- Jangan pernah menaruh API key/secret di repo. Secret hanya lewat env/secret file.
- Jangan menjalankan order ke mainnet sebelum user secara eksplisit menyetujui Fase 11 (live).
- **Batas deployment VPS (user, 2026-10-17):** alias `ssh plesk` adalah server Plesk yang sudah berisi proyek lain. Lindungi proyek-proyek tersebut: deployment bot harus memakai direktori/project/network/volume tersendiri dan resource limit. Jangan mengubah Plesk/domain/web server/firewall, me-restart Docker, melakukan upgrade global, atau membersihkan resource Docker proyek lain. Audit awal hanya read-only; validasi keadaan layanan lama sebelum/sesudah perubahan.
- **Akses VPS untuk Claude** (user, 2026-10-17): `.claude/settings.local.json` lokal (tidak di-commit) mengizinkan `ssh -o BatchMode=yes plesk ...` dan membatasi classifier auto-mode pada project `trading-bot-paper`. Server diakses sebagai root: tetap hati-hati.
- **Secret ke VPS** hanya disalin lewat stdin SSH ke `.env.vps` (mode 600), tidak pernah dicetak, di-commit, atau ditulis ke chat/memori. Chat ID Telegram user hanya di `configs/config.yaml` server.
- **Pesan Telegram** berbahasa Indonesia dan ramah pemula (user, 2026-10-18): setiap angka dijelaskan, istilah (stop loss, equity) diberi penjelasan singkat. Setiap nominal USDT disertai perkiraan USD dan rupiah (user, 2026-10-19), contoh `+4.80 USDT (≈ +$4.80 · +Rp 86.400)`. Teks ada di `cmd/bot/messages.go`.
- **Satu strategi per pair** dalam satu akun: strategi pada pair yang sama berbagi satu posisi dan saling mengganggu.

## Aturan fase build (Fase 2–8)

- Fokus menulis fitur. **Jangan** menulis unit test atau integration test baru, dan jangan melakukan uji end-to-end (paper run, order testnet, probe mainnet). Semua itu dikerjakan di Fase 9.
- Pemeriksaan yang tetap wajib sebelum commit (cepat, bukan testing): `go build ./...`, `go vet ./...`, dan `golangci-lint run` bersih. Test lama dari Fase 0–1 harus tetap ter-compile (`go vet` mencakup file test).
- Setiap hal yang perlu diuji, asumsi yang belum diverifikasi, atau perilaku API yang belum pasti dicatat di bagian **"Backlog testing"** di bawah, supaya tidak terlewat di Fase 9.
- Test yang sudah ada dari Fase 0–1 dipertahankan; tidak perlu dihapus.

## Progres

- [x] Fase 0: Fondasi (2026-10-17)
- [x] Fase 1: Domain + Binance market data (2026-10-17)
- [x] Fase 2: Indikator + Strategy + Backtester (2026-10-17, belum dites)
- [x] Fase 3: Risk + OMS + Paper trading (2026-10-17, belum dites)
- [x] Fase 4: Binance LiveBroker (testnet/mainnet) (2026-10-17, belum dites)
- [x] Fase 5: Telegram + Prometheus/Grafana (2026-10-17, belum dites)
- [x] Fase 6: Web dashboard (2026-10-17, belum dites)
- [x] Fase 7: Bybit (2026-10-17, belum dites)
- [x] Fase 8: Strategi lanjutan (grid/DCA, arbitrase, market making) (2026-10-17, belum dites)
- [x] Fase 9: Testing (2026-10-18, cakupan aktif; lihat "Penutupan Fase 9")
- [ ] Fase 10: Fixing dan audit (temuan Fase 9 sudah diperbaiki saat ditemukan; sisa di "Penutupan Fase 9")
- [ ] Fase 11: Live mini (hosting VPS paper/testnet dimajukan ke Fase 9)

## Hasil testing (2026-10-17)

### Sudah diverifikasi secara lokal

- [x] `go test -race ./...`, `go build ./...`, `go vet ./...`, dan `golangci-lint run` lolos. Race detector tidak melaporkan race pada skenario yang dijalankan.
- [x] Indikator: seed/warmup EMA/SMA, Wilder RSI/ATR termasuk gap dan zero loss, MACD, Bollinger, crossover. Acuan berupa deret pendek dengan perhitungan aritmatika independen; perbandingan lengkap terhadap TA-Lib/TradingView masih tersisa.
- [x] Portfolio: average entry, partial close, flip long → short, alokasi fee/funding, saldo/equity/margin, salinan posisi. Risk: sizing stop/fallback, pembulatan lot/minimum, cap leverage/equity/free margin, posisi/exposure, UTC daily loss, drawdown, halt/reset.
- [x] Sim/engine: harga market/limit/stop/TP, fee/slippage, candle sebelum waktu order, stop lebih dahulu dari TP, cancel callback sinkron, entry → proteksi → exit, trailing, pause/kill/CloseAll, stop ditolak/terlewati, reversal dasar, warmup candle basi, adopsi proteksi, ID alfanumerik.
- [x] Backtest: referensi return/CAGR/MDD/Sharpe/Sortino/PF, PF tak hingga menjadi JSON `null`, funding 8-jam, timer DCA, equity multi-akun, ekspor CSV, error/cancel propagation. Golden `emacross`: 120 candle **sintetis** pada seri bernama BTCUSDT, 11 trade, dengan fee/slippage; fixture di `internal/backtest/testdata/emacross_synthetic.json`. Fixture ini tidak dipakai untuk memilih parameter strategi.
- [x] Semua strategi bawaan: perilaku dasar grid (fill penuh, restart, stop), DCA (jadwal/dip/max buys/TP), funding hedge (arah/grace/repair), xspread (dua arah/cooldown/convergence), mm (inventori spot/post-only/requote), dan validasi parameter tidak dikenal.
- [x] Contract REST bersama Binance/Bybit menggunakan **fixture sintetis dan HTTP transport lokal**: instrumen, satuan base, urutan klines, market order/cancel, payload signature HMAC. Parser WS tambahan: key kembar executionReport Binance dan confirm/direction Bybit. Fixture rekaman exchange dan lifecycle private WS nyata masih tersisa.
- [x] Telegram dengan transport palsu: whitelist, dispatch, escape, batas antrean, UTF-8 pesan panjang, redaksi error token. Web dengan `httptest`: basic auth, security headers, HX guard, pause/resume/kill/reset, halaman tanpa sesi dan static asset. Metrics: flag kill/pause dan label latensi broker.
- [x] Config: params strategi, merge fee default, secret ditolak dari YAML, password wajib di luar loopback. Recorder: FIFO, antrean penuh, shutdown flush.
- [x] Integration test TimescaleDB pada database sementara **terpisah** `tradingbot_phase9_test`: candle/instrumen, order sebelum fill, metadata, perpindahan run order yang diadopsi, posisi/saldo sesi, FinishRun, migrasi 00001–00003 up → down → up. Pipeline paper: DCA → Sim → engine → recorder → DB, dua sesi dengan restart dan posisi terbuka, tanpa error recorder; juga lolos race detector.

Perintah pengulangan (database harus terpisah dari database bot yang dipakai):

```sh
go test -race ./...
TB_TEST_DATABASE_URL='<URL database test>' go test -race -tags integration ./internal/storage
# Rollback migrasi hanya opt-in pada database dengan nama berakhiran _test:
TB_TEST_DATABASE_URL='<URL database test>' TB_TEST_MIGRATIONS=1 go test -tags integration ./internal/storage -run TestMigrationCycle
# Jalankan terpisah dari test storage/bot lain karena recovery memilih sesi terbaru:
TB_TEST_SESSION_DATABASE_URL='<URL database test yang sudah dimigrasi>' go test -race -tags integration ./cmd/bot -run TestPaperSession
```

Di sandbox Codex ini cache build perlu diarahkan ke direktori writable (`GOCACHE=/tmp/trading-bot-go-cache`). Test fixture HTTP/WS dan koneksi database juga memerlukan akses socket localhost.

### Temuan yang sudah diperbaiki dengan regression test

| Prioritas | Temuan | Perbaikan / bukti |
|---|---|---|
| Kritis | Sesi trading gagal dibuat karena `symbols=nil` dikirim sebagai SQL NULL ke kolom NOT NULL. | `storage.CreateRun` menormalisasi list kosong; `TestPaperSessionRestartPersistsNewOrders` menjalankan pipeline sesi sungguhan ke DB. |
| Tinggi | Bracket yang diterima broker tidak melewati hook order dan exchange ID hilang; fill/proteksi tidak tersimpan dengan benar. | Engine mencatat respons tiap leg sebelum event fill; `TestBracketOrdersPersistBeforeFills`. |
| Tinggi | Leg SL tetap aktif ketika leg TP gagal ditempatkan. | Engine membatalkan leg yang sudah diterima sebelum exit; `TestPartialBracketFailureCancelsAcceptedLeg`. |
| Tinggi | Adopsi order tidak dipersistensikan ke sesi aktif; restart berikutnya bisa kehilangan order yang belum berubah status. | `AdoptOrders` memanggil hook recorder; `TestAdoptOrdersRestoresProtection`. |
| Tinggi | Shutdown membatalkan write recorder yang sudah diambil dari antrean sehingga flush melewatkannya. | Write yang sedang berjalan memakai context terpisah dengan timeout, lalu antrean di-flush; test FIFO/flush dan integration restart. |
| Tinggi | Bybit menganggap kegagalan jaringan set leverage/mode one-way sebagai sukses karena `apiCode(err)==0`. | Error non-API diteruskan; `TestSetLeveragePropagatesNetworkFailure` dan contract test. |
| Tinggi | Error HTTP Telegram berlapis dapat menampilkan URL yang memuat token. | Semua lapisan URL error dilepas dan token diredaksi; `TestTransportErrorsDoNotExposeToken`. |
| Sedang | ID exchange Sim (`sim-1`, dst.) berulang setelah restart, melanggar unique index dan menggagalkan persistensi order/fill. | ID Sim diturunkan dari client ID; integration paper dua sesi. |
| Sedang | Equity backtest multi-akun diambil sebelum seluruh event pada timestamp yang sama selesai. | Sampel dibuat setelah event terakhir pada timestamp itu; `TestMultiAccountEquityIncludesAllEventsAtSameTimestamp`. |
| Sedang | Override fee maker saja mengubah fee taker default menjadi nol. | Default fee di-seed per field sebelum overlay YAML/env; `TestStrategyParamsAndFeeOverridesKeepDefaults`. |
| Rendah | Pemotongan pesan Telegram berbasis byte memutus karakter UTF-8. | Pemotongan berbasis rune; `TestMessageTruncationPreservesUTF8`. |
| Tinggi | Feed WS di VPS terputus berulang: resolver host (Tailscale → upstream ISP) kadang menjawab `stream.binance.com` dengan halaman blokir Trust Positif (`lamanlabuh.aduankonten.id`). ISP juga mencegat 8.8.8.8/9.9.9.9. | Keputusan user: sidecar resolver. CoreDNS khusus bot meneruskan lewat DNS-over-TLS (sertifikat `cloudflare-dns.com`); WS tersambung 0,26 detik setelah start. Commit `46fbbd3`, `1e8e78e`. |
| Tinggi | Write recorder yang gagal (DB tidak terjangkau) langsung dibuang; order/fill bisa hilang dari DB. | Retry dengan backoff 1–30 detik tanpa mengubah urutan FIFO; trade (tanpa kunci unik) hanya diulang bila error pasti terjadi sebelum data terkirim; write yang sedang di-retry ikut flush saat shutdown. `TestTransientFailureRetriesInOrder` dan tiga test lain. |
| Tinggi | Saat TP OCO spot ter-trigger, Binance mengirim leg SL `EXPIRED` **sebelum** fill TP (terekam di Spot Testnet). Engine menganggapnya stop ditolak dan mengirim market exit kedua: exit ganda/saldo tidak cukup dan notifikasi palsu. | `broker.LinkedBrackets` (Binance: spot); SL linked yang expired saat TP masih terbuka menunggu hasil TP, dan baru exit jika TP juga berakhir tanpa fill. `TestLinkedStopExpiryWaitsForTakeProfit` gagal pada kode lama. |
| Kritis | Setiap start testnet/live langsung mem-pause trading: `defer s.risk.Observe(..., s.book.Total().Equity)` mengevaluasi equity 0 sebelum reconcile → "daily loss 100%". | Observasi ditahan sampai `ArmRisk` setelah reconcile pertama; `TestLiveStartDoesNotTripKillSwitch` mereproduksi pesan aslinya. |
| Kritis | Spot live menganggap **seluruh saldo base asset** sebagai posisi bot (testnet: 1 BTC bawaan). Sinyal exit, kill switch, atau `CloseAll` akan menjual aset yang tidak dibeli bot. | Keputusan user: bot hanya mengelola miliknya. Posisi spot = fill bot (dipulihkan dari DB saat restart, disimpan setiap fill), dibatasi saldo; sisanya diabaikan. `TestReconcileSpotManagesOnlyTheBotsHoldings`. |
| Tinggi | Kill switch dievaluasi pada candle warmup/backfill historis dengan posisi pulihan dan kas belum direkonsiliasi → tripped saat restart, market sell sebelum user stream aktif. | Engine tidak mengevaluasi risk pada candle basi dan sebelum di-arm; `TestHeldRiskIgnoresWarmupUntilArmed`. |
| Tinggi | Baseline rugi harian diambil dari observasi pertama hari itu meskipun equity 0 → `day_start=0` menonaktifkan batas rugi harian sepanjang hari. | Baseline = equity positif pertama; `TestDailyBaselineIgnoresEmptyLedger` gagal pada kode lama. |
| Tinggi | OCO dari posisi yang sudah ditutup (sebelum diadopsi) tetap diadopsi; di spot bisa menjual aset milik user. | Order SL/TP tanpa posisi dibatalkan saat reconcile, bukan diadopsi; terbukti membatalkan OCO yatim nyata di testnet. `TestOrphanedProtectionIsCanceledNotAdopted`. |
| Sedang | Notice dari session (posisi dikoreksi/ditutup di luar bot, posisi tanpa stop, order yatim) hanya dikirim ke Telegram, tidak pernah ke log. | `session.notice` mencatat log lalu meneruskan. |
| Rendah | Membersihkan OCO (baru atau hasil adopsi) meng-cancel kedua leg → WARN `-2011` dan resync tiap exit; alasan exit strategi hilang dari order. | Bracket linked di-cancel sekali; `in.Reason` diteruskan ke order exit (label trade tetap tag agar kardinalitas metrics terbatas). |
| Sedang | CI memakai hasil test integration dari cache Go sehingga skema tidak dibuat di DB service baru; job paper recovery gagal begitu package `storage` tidak berubah. | `-count=1` di step integration CI dan `make test-integration`. |

### Pengujian lanjutan tanpa kredensial exchange (2026-10-17)

- Pada tahap pengujian ini ditambahkan **14 fungsi test Go** serta **15 pemeriksaan alert** di `deploy/prometheus/alerts.test.yml`. Cakupan test yang dipertahankan mengikuti scope exchange saat ini.
- Suite `go test -race ./...`, `go vet -tags integration ./...`, `golangci-lint run`, dan `make build` lolos. Verifikasi akhir gabungan `go test -race -p 1 -tags integration ./...` juga lolos pada database test terpisah. `make build` sekarang juga menghasilkan `bin/backtest`. Integration paper restart dengan posisi serta order pending lolos tiga kali dengan race detector.
- Partial fill: setiap bagian entry mendapat SL/TP sesuai kuantitas; bagian berikutnya mengganti ukuran bracket. Fill SL/TP parsial mengecilkan bracket sisa. Protective fill membatalkan sisa entry. Trade ID yang sama tidak mengubah ledger/fee dua kali. Snapshot status `filled` tidak menghapus order sebelum semua execution lokal diproses.
- Risk memasukkan **sisa kuantitas** order pending ke exposure dan slot posisi, termasuk order mentah strategi. Sizing entry mengurangi free margin dengan margin yang dicadangkan order pending. Reservasi baru dilepas setelah acknowledgement cancel, bukan setelah request cancel saja.
- Reversal menunggu fill exit hingga posisi flat sebelum mengirim entry lawan arah; pause mencegah deferred entry. `CancelAll` dibatasi exchange + market + simbol + strategi.
- Paper resume: posisi dan order strategi dipulihkan. **Entry pending dibatalkan**, sebab rencana ATR SL/TP/trailing belum dipersistensikan; status canceled dipindahkan ke sesi aktif. Order Sim parsial hanya mengisi `Remaining()`, termasuk fee dan weighted average yang benar.
- Browser headless Brave: overview, partial HTMX, pause/resume, konfirmasi kill/reset, daftar/detail backtest, chart equity/candle dengan **648 candle dan 28 marker**, desktop light dan mobile dark (390×844). Tidak ada overflow halaman mobile dan tidak ada error JS/CSP setelah perbaikan. State live saat uji browser adalah fixture, hasil backtest berasal dari data historis. Bukti ringkas: [browser-verification.json](testing/browser-verification.json).
- CSS atribusi bawaan lightweight-charts sebelumnya ditolak CSP. Kini hanya hash stylesheet versi library yang diizinkan; `unsafe-inline` dan `unsafe-eval` tetap tidak diaktifkan. Regression test menghitung hash dari library yang di-vendor.
- Monitoring: Grafana sehat, dashboard `tradingbot` terprovisi dengan 14 panel dan kedua datasource berstatus `OK`. Container lama Prometheus belum memiliki mount `alerts.yml`; compose diterapkan ulang. Kini config valid dan keenam rules berstatus `ok` di API. `make test-monitoring` menguji seluruh alert pada kondisi gagal/sehat dan hold time dalam container tanpa jaringan. Bukti: [monitoring-verification.json](testing/monitoring-verification.json). Tidak ada pesan Telegram nyata dikirim.

Temuan lanjutan di atas sudah diperbaiki untuk melanjutkan pengujian: replay fill, status snapshot mendahului execution, proteksi partial entry/exit, reservasi order pending, refill Sim setelah restart, reversal serentak, cancel lintas exchange, entry tanpa rencana proteksi setelah paper restart, dan CSP chart.

### Order nyata di Binance Spot Testnet (adapter, 2026-10-17)

Test opt-in [testnet_test.go](../internal/exchange/binance/testnet_test.go) memakai endpoint testnet yang di-hardcode dan hanya jalan dengan build tag `testnet` plus `TB_TESTNET_ORDERS=1` (tidak ikut CI). Dijalankan dari laptop dengan BTCUSDT 0,00025 (sekitar 20 USDT testnet), juga dengan race detector:

```sh
TB_TESTNET_ORDERS=1 [TB_TESTNET_RECORD=1] TB_EXCHANGES__BINANCE__API_KEY=... TB_EXCHANGES__BINANCE__API_SECRET=... \
  go test -count=1 -tags testnet -run TestSpotTestnet -v ./internal/exchange/binance
```

- **Lifecycle:** subscribe user stream WS API, market buy terisi lewat `executionReport`, OCO SL/TP diterima dan tampil di `openOrders` dengan tipe/harga benar, cancel satu leg membatalkan kedua leg (`CANCELED` dengan `C` = id asli), cancel kedua `-2011` memicu resync, flatten.
- **Restart/reconciliation:** sesi kedua menemukan dan mengadopsi limit order sesi pertama; `SyncOrders` mengirim status akhir order yang dibatalkan selama "bot mati"; order adopsi bisa di-cancel.
- **TP ter-trigger (best effort, lolos 2 dari 3 run; run gagal pertama berupa race di jalur "tidak ter-trigger" yang sudah diperbaiki):** fill TP dan leg SL `EXPIRED`. Urutan event asli menghasilkan temuan exit ganda di tabel atas. TP adalah market order setelah trigger: satu run terisi 81.535,01 untuk trigger sekitar 81.611 (slippage book testnet).
- Respons REST order/OCO/cancel/query dan pesan user stream terekam di `internal/exchange/binance/testdata/testnet/` (tanpa kredensial, saldo akun REST tidak disimpan) dan diputar ulang offline oleh `TestRecordedSpotTestnetPayloads` di CI.
- Batasan: komisi testnet 0, jadi jalur fee base/BNB belum diuji dengan nilai nyata. Futures (Demo key belum ada), engine/bot penuh dalam mode testnet, dan kill switch nyata belum dijalankan.

### Bot penuh mode testnet (laptop, 2026-10-17 22:40–23:11 WIB)

`bin/bot run` dengan `app.mode: testnet`, DB lokal terpisah `tradingbot_testnet`, config di luar repo: BTCUSDT spot, emacross 1m cepat (2/4, RSI 3, ATR 3, stop 3 ATR, target 2 ATR), `risk_per_trade` 0,0005 dan `max_position_pct` 0,003 (sekitar 30 USDT per posisi). Akun testnet berisi 1 BTC, 1 ETH, 1 BNB, 10.000 USDT sejak awal; komisi 0. Enam run, setiap temuan diperbaiki dan diuji ulang pada akun nyata:

- Run 1–2 menemukan kill switch saat start dan saldo bawaan dianggap posisi. Run 3–4 menemukan evaluasi risk saat warmup, baseline harian 0, market sell sebelum user stream, dan OCO yatim (lihat tabel temuan).
- Run 5 (setelah perbaikan): start tanpa kill switch, `spot holdings not bought by the bot are left alone ... BTC qty=1`, OCO yatim dibatalkan (`updateTime` exchange = saat reconcile), entry + OCO terisi lewat user stream. **Kill switch dashboard**: OCO dibatalkan, hanya 0,00036 BTC milik bot dijual, saldo BTC kembali tepat 1,00000000, tidak ada open order, metrics `kill_switch`/`paused` = 1; reset lewat dashboard bekerja (gauge diperbarui per 10 detik).
- Run 6: restart dengan posisi + OCO terbuka → posisi dipulihkan dari DB (`run_id=5`), kedua leg OCO diadopsi, kill switch tetap 0, tidak ada notice posisi tanpa stop. Kill switch akhir menutup posisi; temuan WARN `-2011` pada OCO adopsi diperbaiki (`fix(engine): treat adopted oco legs as linked`).
- Exit strategi (cross berlawanan) membatalkan OCO lalu market sell, sesuai desain.
- Run 7 (Telegram aktif, 23:34–23:56 WIB): TP OCO ter-trigger dan engine mencatat `linked stop expired, waiting for the take profit` lalu satu trade `take_profit`, membuktikan perbaikan exit ganda pada bot penuh. Semua perintah Telegram diuji user dari ponselnya; lihat Sisa Fase 9.
- Belum diuji: fee non-nol, Telegram nyata, futures, beberapa simbol/strategi sekaligus, dan run berjam-jam. Perbaikan risk/warmup juga berlaku untuk paper yang memulihkan posisi; VPS akan menerimanya setelah jendela observasi 24 jam.

### Eksperimen backtest historis

Arsip publik Binance **USDT-M BTCUSDT 1h, 2025-01-01 sampai sebelum 2025-04-01** berhasil diakses lewat `data.binance.vision`, tanpa API key. Ada 2160 candle berurutan tanpa gap; SHA-256 ketiga ZIP cocok dengan checksum resmi. API trading live masih memerlukan pemeriksaan jaringan terpisah.

CLI `backtest -split 0.7 -save -out ...` dijalankan untuk tiga pasangan EMA. Split pada **2025-03-05 UTC**: 1512 bar in-sample dan 648 bar out-of-sample, masing-masing memakai akun simulasi baru. Semua hasil, trade, dan equity curve tersimpan ke database test dan CSV. Rincian parameter, asumsi, sumber, checksum, dan metrik: [historical-backtest.json](testing/historical-backtest.json).

| EMA fast/slow | Return in-sample | Return out-of-sample | Trade IS/OOS | MDD IS/OOS |
|---|---:|---:|---:|---:|
| 12/26 (default) | -5,72% | -3,42% | 44 / 23 | 10,65% / 3,49% |
| 6/18 | -3,55% | +1,67% | 82 / 35 | 7,31% / 3,98% |
| 20/50 | -2,82% | -2,32% | 27 / 14 | 7,40% / 3,11% |

Asumsi: saldo awal 1000 per segmen, leverage 2, risk default, taker 0,05%, maker 0,02%, slippage 2 bps, funding konstan 0,01% per 8 jam, fill OHLC 1h, tanpa warmup terpisah. Tick/lot/min-notional adalah asumsi aturan instrumen, bukan snapshot historis. Belum ada bukti cukup untuk memilih parameter live; default konfigurasi belum diubah. Perbandingan tiga parameter pada satu pair dan satu kuartal belum memadai untuk validasi strategi.

### Validasi emacross lebih luas (2026-10-17)

Data publik REST mainnet Binance USDT-M (laptop kini bisa menjangkaunya): BTC, ETH, SOL, BNB, XRP, 1h dan 4h, 2023-10-01 sampai 2026-10-01, 164.400 candle, gap 0, aturan instrumen dari `exchangeInfo` saat unduh. DB riset terpisah `tradingbot_research_test`. Grid EMA 6/18, 9/21, 12/26, 20/50 × 1h/4h, split 50% (in-sample sampai 2025-04-01, out-of-sample sesudahnya), fill 1h, fee/slippage/funding default, leverage 2, risk default. Rincian: [emacross-grid.json](testing/emacross-grid.json).

| TF | EMA | Return IS rata-rata | Return OOS rata-rata | Simbol untung OOS | MDD OOS terburuk |
|---|---|---:|---:|---:|---:|
| 4h | 20/50 | +0,9% | +0,5% | 3/5 | 16,7% |
| 4h | 6/18 | +11,8% | −4,2% | 2/5 | 20,1% |
| 4h | 9/21 | +7,9% | −7,1% | 1/5 | 20,0% |
| 4h | 12/26 | +6,5% | −7,9% | 1/5 | 19,3% |
| 1h | 9/21 | −12,9% | −14,2% | 0/5 | 20,7% |
| 1h | 6/18 | −15,3% | −16,1% | 0/5 | 20,2% |
| 1h | 20/50 | −10,3% | −17,2% | 0/5 | 20,1% |
| 1h | 12/26 (default kode) | −8,1% | −18,8% | 0/5 | 20,2% |

Kesimpulan: semua varian 1h rugi di kedua segmen; varian 4h yang unggul in-sample berbalik rugi out-of-sample (overfitting/perubahan rezim). 4h 20/50 paling tidak buruk tetapi tidak konsisten per pair (OOS SOL +14,3%, XRP +8,6%, BTC +2,3%, BNB −8,2%, ETH −14,3%). 38 dari 80 segmen berhenti karena kill switch drawdown 20%. **Emacross dengan bentuk sekarang tidak layak untuk live.** Default kode (1h 12/26) tidak diubah; pilihan parameter/strategi diputuskan user sebelum Fase 11 (opsi: pakai 4h 20/50 hanya untuk uji operasional, menambah filter rezim/tren, atau menguji strategi lain seperti DCA/grid dengan validasi yang sama).

### Validasi DCA dan grid (2026-10-18)

Data dan asumsi sama dengan grid emacross; futures dipakai sebagai proksi spot. BTC dikeluarkan karena 50/40 USDT per order di bawah minimum futures BTC (step 0,001, min notional 50): engine menolaknya dengan WARN di log saja, sehingga strategi diam-diam tidak trading. Rincian: [dca-grid.json](testing/dca-grid.json).

- **DCA** (50 USDT/hari, maks 10 beli): tanpa dip filter dan TP 10% paling baik, +22,9% IS / +8,7% OOS rata-rata (4 pair). Dip filter 3% rugi di OOS untuk semua TP. DCA hanya unggul dari buy-and-hold di pasar turun (OOS SOL +23,8% vs −5,2%, XRP +8,0% vs −28,7%) dan jauh tertinggal di pasar naik (OOS ETH −6,3% vs +47,4%). MDD semua run sekitar 20%: kill switch drawdown default menutup posisi DCA di titik terendah, yang bertentangan dengan cara kerja DCA. Risk config harus disesuaikan per strategi bila DCA dipakai.
- **Grid** dipasang ulang tiap kuartal (±15%, 20 level geometris): rata-rata per kuartal BNB +0,91%, ETH −1,24%, SOL −0,17%, XRP −0,79%; sekitar separuh kuartal untung. Fill limit di simulasi optimistis, jadi hasil nyata kemungkinan lebih buruk.
- Kesimpulan gabungan dengan grid emacross: **belum ada strategi dengan edge yang meyakinkan.** DCA berguna sebagai otomasi akumulasi aset yang memang ingin dipegang, bukan sumber return di atas buy-and-hold. Keputusan strategi Fase 11 ada di user.

### Emacross dengan filter tren (2026-10-18)

Parameter baru `trend_ema` (default 0 = nonaktif): entry long hanya di atas EMA periode itu, short hanya di bawahnya; exit tidak difilter. Data, split, dan asumsi sama. Rincian: [emacross-trend.json](testing/emacross-trend.json).

| TF | EMA | Tren | Return OOS | Untung OOS | MDD OOS |
|---|---|---|---:|---:|---:|
| 4h | 20/50 | – | +0,5% | 3/5 | 16,7% |
| 4h | 20/50 | 200 | +1,2% | 2/5 | 10,6% |
| 4h | 9/21 | – | −7,1% | 1/5 | 20,0% |
| 4h | 9/21 | 200 | +0,3% | 2/5 | 9,7% |
| 1h | 9/21 | 100 | −11,8% | 0/5 | 20,2% |

Filter tren terutama memangkas drawdown (sekitar setengahnya di 4h) dan mengubah rugi menjadi kira-kira impas; return tetap sekitar 0–1% dalam 18 bulan. Timeframe 1h tetap rugi. Belum ada edge yang layak untuk uang sungguhan.

### Rilis GitHub dan persiapan VPS (2026-10-17)

- Repo private: [Rizz404/trading-bot](https://github.com/Rizz404/trading-bot), branch `main`. Riwayat lokal sudah di-push melalui SSH; kredensial di `.env` dan konfigurasi lokal tetap di-ignore. Varian `.env.*` juga di-ignore, kecuali `.env.example`.
- Workflow `.github/workflows/ci.yml`: format, modul, vet, lint, unit race, build, TimescaleDB sementara, migrasi up/down/up, paper recovery, alert Prometheus, dan smoke test CLI image. Publikasi GHCR hanya sesudah pemeriksaan lolos dan hanya dari `main` (push/pemicu manual). Perubahan docs saja tidak membangun ulang image.
- Image mendukung amd64/arm64, berisi bot/datafetch/backtest, ditandai commit SHA, dan deploy memakai digest. Actions dipin ke SHA. CI memakai `GITHUB_TOKEN` bawaan; tidak memerlukan API key exchange/Telegram atau SSH VPS.
- Manifest [deploy/compose.vps.yml](../deploy/compose.vps.yml) memakai image tanpa `build:`, volume persisten, log terbatas, Prometheus retention 7 hari, port host loopback, dan guard `allow_live=false`. [configs/config.vps.example.yaml](../configs/config.vps.example.yaml) dipakai untuk paper Spot BTC/ETH di VPS. Docker sudah aktif/enabled dan stack memiliki restart policy; reboot server tidak diuji. Backup terjadwal dan pengujian 24 jam belum dijalankan.
- Verifikasi lokal: actionlint lolos; manifest Compose berhasil diinterpolasi dengan nilai dummy; contoh YAML lolos loader/validasi; image native berhasil dibangun; versi serta bantuan ketiga CLI diuji tanpa jaringan. Test command Go dan alert Prometheus lolos. Temuan CLI `-h` sebelumnya exit 1 sudah diperbaiki agar exit 0.
- Run GitHub pertama: [39035767847](https://github.com/Rizz404/trading-bot/actions/runs/39035767847), commit `182ae2ba80899ec448ca571149393997d5e0d1c3`; **seluruh job sukses**, termasuk smoke test image serta publikasi amd64/arm64. Run pertama dipicu manual setelah push awal belum menghasilkan run.
- Image rilis: `ghcr.io/rizz404/trading-bot@sha256:7001d22390fe948ed976bc774c3e2b352079a75a37775cdef07e2c1b42b88c86`. Nilai diambil dari log job publikasi yang sukses. Ini rilis pertama, bukan image yang sedang berjalan di VPS. Metadata visibility paket belum bisa dibaca memakai token CLI saat ini; akses baca GHCR server kemudian berhasil setelah login manual user. Repo sudah diverifikasi private; paket baru GHCR default private, dan workflow tidak mengubah visibility.
- Deploy awal VPS sudah berjalan pada commit `d5a52e2b2f1aa1c5c78f2bbee7c55cb48468643f`, image `ghcr.io/rizz404/trading-bot@sha256:7818595501fa20eaa94a4a9ff0de74ad9a19678cd99de11f7e578fe969fec063`, CI [39037824371](https://github.com/Rizz404/trading-bot/actions/runs/39037824371) sukses. Run paper pertama mulai 2026-10-17 14:09:38 UTC (21:09:38 WIB). Ini belum merupakan kelulusan uji 24 jam.
- Target diberikan user: `ssh plesk`. Audit read-only menemukan AlmaLinux 9.8, amd64, 8 vCPU, RAM sekitar 32 GB (sekitar 10 GB available), disk sekitar 56 GB kosong/86% terisi, Docker 29.9.1 dan Compose 5.6.0. Docker aktif/enabled, SELinux disabled. Tidak ada project `trading-bot-paper` atau direktori calon `/opt/trading-bot-paper`; port loopback yang direncanakan belum terpakai. Pada audit awal, login GHCR dan host key GitHub belum siap; keduanya kemudian disiapkan khusus deployment ini. Kondisi awal dicatat sebelum perubahan, dan perbandingan terakhir setelah deploy menunjukkan 28 container lama serta 5 service Plesk/web server/Docker tidak berubah. Batas resource dan port loopback terverifikasi pada stack baru.

### Checkpoint handoff VPS (2026-10-17, 21:16:57 WIB)

- User meminta pekerjaan disudahi dan dokumentasi disiapkan agar Claude melanjutkan. Tidak ada deployment lanjutan setelah permintaan ini; hanya pemeriksaan read-only dan penulisan handoff. Bot paper dibiarkan berjalan.
- Lokasi server `/opt/trading-bot-paper/repo`, Compose project `trading-bot-paper`. Source di-clone langsung dari GitHub memakai deploy key read-only; image ditarik dari GHCR. Tidak ada folder source laptop yang disalin ke VPS. Password `.env.vps` dibuat di server, tanpa API key exchange/token Telegram.
- Migrasi 1–3 sukses. Keempat container baru running, restart count 0 dan tidak OOM saat checkpoint. Health DB/bot sehat; dashboard menolak tanpa password (401), menerima password (200). Grafana dashboard `tradingbot`, kedua datasource `OK`, keenam alert rules sehat. Semua host port hanya loopback dan DB tidak dipublikasikan.
- Uji isolasi network dilakukan **hanya pada container bot baru**, 14:10:45–14:12:10 UTC. Network sudah tersambung kembali. WS pulih otomatis 14:15:01 UTC, backfill 10 candle selesai 14:15:03 UTC. Checkpoint DB: BTCUSDT/ETHUSDT masing-masing 1007 candle, gap 1m = 0; candle terakhir open 14:15 UTC, equity terbaru 14:16:38 UTC. Belum ada order/fill/trade pada sesi ini.
- Temuan: isolasi network juga memutus akses DB; satu write snapshot equity timeout pada 14:11:48 UTC. Durasi reconnect setelah network dipulihkan sekitar 2 menit 52 detik. Audit retry/durabilitas recorder dan latensi pemulihan masih perlu dikerjakan; jangan menyebut semua data persistence sudah lolos.
- Script backup/observasi dan empat unit systemd sudah di-push pada commit `cedbfdc68389bacd2285239b0fe0a12eb03a7b7b`; docs pada `e34b989`. CI [39039542901](https://github.com/Rizz404/trading-bot/actions/runs/39039542901) seluruhnya sukses. Image baru `ghcr.io/rizz404/trading-bot@sha256:239c7ac671be4337c0927f9346043e369214d2e4676cd33e01dc0466729df837` **belum dideploy**. Script lolos pemeriksaan sintaks; eksekusi runtime, backup, restore, dan pemasangan timer belum diuji/dilakukan. Checkout server masih `d5a52e2`.
- Detail akses non-rahasia, bukti, urutan tindak lanjut, dan batasan ada di [handover-vps.md](handover-vps.md). Bukti checkpoint read-only: [vps-handoff.json](testing/vps-handoff.json). Tidak ada klaim uji 24 jam selesai atau notifikasi eksternal aktif.

### Lanjutan operasi VPS (Claude, 2026-10-17, sampai 22:05 WIB)

- Timeline log uji isolasi menunjukkan jeda ~3 menit **bukan** dari backoff: empat dial setelah network pulih masing-masing habis di timeout 15 detik. Error dial kini mencatat tahap terakhir (`dns`/`connect`/`tls`/`upgrade`). Pada deploy berikutnya log langsung menunjukkan `stage connect`, dan penelusuran menemukan blokir DNS di atas. Penyebab jeda awal kemungkinan sama, tetapi tidak dibuktikan ulang dengan uji isolasi.
- Backup pra-update manual: `/opt/trading-bot-paper/backups/pre-update-20261017T142721Z` (dump, config, commit, digest, checksum; mode 600).
- Deploy `2de9d44` (14:44:51 UTC) sekaligus uji restart graceful pertama di VPS: run 1 ditutup (`stopped_at`), run 2 melanjutkan **1 posisi + 2 order** paper yang dibuka strategi sejak checkpoint. Deploy `1e8e78e` (15:02:40 UTC) mengulanginya: run 3 melanjutkan state yang sama, gap candle tetap 0.
- Resolver: container `dns` (CoreDNS 1.14.7) di network `trading-bot-paper_resolver` 172.31.253.0/29 hanya untuk bot; DNS host/Tailscale/Plesk tidak diubah. Deploy pertama CoreDNS gagal `exec /coredns: operation not permitted` (file capability + nonroot + `no-new-privileges`); gerbang deploy mencegah bot ikut diganti. Perbaikan: UID 0 dengan `cap_drop: ALL` + hanya `NET_BIND_SERVICE`, read-only.
- `deploy/vps-backup.sh` dijalankan manual: `backup-20261017T150314Z`, checksum OK. **Uji restore** ke DB sementara `tradingbot_restore_test` (dengan `timescaledb_pre_restore`/`post_restore`): `pg_restore` exit 0, jumlah baris semua tabel sama dengan DB live; DB uji dihapus. Backup masih hanya di disk VPS yang sama; salinan luar server belum ada.
- Empat unit `trading-bot-paper-*` dipasang setelah nama dipastikan belum dipakai; `systemd-analyze verify` bersih. Timer observasi (5 menit) dan backup harian (~03:15 UTC) aktif; sampel observasi pertama `problems: []`.
- Setelah semua perubahan: 28 container lama dan 5 service tidak berubah. Pemakaian memori: bot 13 MiB, db 126 MiB, dns 11/64 MiB, prometheus 122 MiB, grafana 468/512 MiB (masih dekat limit).
- **Host VPS reboot di luar kendali kita** (tidak ada perintah dari sesi ini): shutdown tertib 15:18:40–15:19 UTC, mati sekitar 22 menit, boot 15:42:14. Journal boot sebelumnya tidak tersimpan; user menyatakan bukan ulahnya (2026-10-17), jadi kemungkinan dari provider atau pihak lain dengan akses panel. Pemicu belum diketahui. Semua proyek lain ikut restart. Stack bot pulih otomatis: start pertama gagal `ping database` karena DB belum siap, restart policy menjalankan ulang 2 detik kemudian, sesi paper dilanjutkan (`run_id=3` → run 4), backfill 48 candle, gap tetap 0, timer tetap aktif. Temuan: saat shutdown host semua container dihentikan bersamaan sehingga DB mati lebih dulu; satu write equity dibuang setelah retry, `FinishRun` gagal, dan run 3 tanpa `stopped_at`. Pertimbangkan retry koneksi DB saat start dan toleransi run yang tidak ditutup.
- **Observasi 24 jam** semula mulai 15:02:41 UTC, terpotong reboot host 15:18–15:42 UTC. **Keputusan user: dihitung ulang dari 15:42:27 UTC (bot siap setelah boot)**, paling cepat dinilai 2026-10-18 15:43 UTC (22:43 WIB) dari `evidence/samples-*.jsonl`. Reboot itu dicatat sebagai bukti pemulihan otomatis, bukan bagian jendela. Restart/redeploy di tengah jendela harus dicatat dan dinilai; jangan menyebutnya lulus hanya karena proses hidup. Bukti: [vps-ops.json](testing/vps-ops.json).

### Hasil observasi 24 jam dan rilis eb4c11b (2026-10-18)

- Jendela 2026-10-17 15:42 → 2026-10-18 15:43 UTC: 288 sampel observasi, **0 sampel bermasalah**, celah sampel maksimal 5,1 menit, bot tidak restart (start tunggal 15:42:25), tanpa OOM, umur candle maksimal 93 detik, gap 0, disk kosong minimal 53,7 GB. Log bot hanya berisi dua WARN dari reconnect proaktif 23 jam (pulih dalam 0,5 detik). Backup terjadwal pertama sukses (03:19 UTC). Paper emacross 1m: 45 trade di DB, equity 974–980 (strategi rugi, sesuai riset; ini uji operasional).
- Rilis `eb4c11b` (image `sha256:6d051bdacec5a433469ea1f64bf20863b152fc91b3a50938c611278a6c30e3c0`, CI sukses) dipasang 15:48 UTC dengan gerbang DNS. Run 4 tercatat `stopped`; run 3 sisa reboot otomatis ditutup `interrupted` di aktivitas terakhirnya (15:17:42). Limit memori `dns` kini 128 MiB. Baseline 28 container dan 5 service lain tidak berubah (20 container masih bertanggal start reboot host kemarin).
- Telegram aktif di VPS: token disalin ke `.env.vps` lewat stdin SSH tanpa ditampilkan, chat ID di `configs/config.yaml` server (bukan repo), `notify_level: warn`, ringkasan harian aktif. Restart graceful 15:48:50 UTC: log `bot stopped` tanpa `telegram send failed`, lalu `Bot started` (konfirmasi penerimaan dari user tersisa).

### Sisa Fase 9

- [x] Repo private, GitHub Actions, publikasi image GHCR, dan akses baca GHCR/repo dari VPS sudah berhasil. Digest yang sedang dipakai dan rilis baru yang belum dideploy tercatat di checkpoint handoff.
- [ ] Selesaikan verifikasi operasi VPS: deploy/health/resource limit, restart graceful dengan posisi terbuka, backup manual + restore, timer observasi/backup, dan pemulihan setelah reboot host (tak terduga) sudah lolos. Tersisa: hasil timer backup terjadwal pertama dan pemantauan memori Grafana. **Salinan backup di luar server ditunda sampai semua fase selesai dan benar-benar live (keputusan user, 2026-10-17).** Jangan sengaja me-reboot server Plesk bersama untuk tes.
- [ ] Audit lanjutan. Selesai (2026-10-17): order request ambigu (timeout/5xx/`-1007`) dicek ke exchange sebelum dianggap gagal (`fix(binance): resolve order requests that failed ambiguously`); siklus DCA dipulihkan dari posisi setelah restart, tidak membeli ulang dari 1 (`fix(strategy): resume dca cycles from restored positions`); DB ditunggu hingga 60 detik saat start dan run yang tertinggal ditutup `interrupted`. Private WS/SL/TP spot nyata lolos di testnet. Komisi spot dalam base asset (Binance/Bybit) kini mengurangi posisi sesuai yang diterima wallet dan grid menjual kuantitas bersih; sebelumnya exit/sell spot bisa ditolak "insufficient balance" atau memakan koin non-bot (testnet tidak menunjukkannya karena komisi 0). Tersisa: replay setelah reconciliation, reservasi inventori spot untuk beberapa order jual mentah sekaligus, state grid/mm setelah restart, futures.
- [x] Fixture REST/WS **terekam** Binance Spot dan integration test testnet opt-in (lihat "Order nyata di Binance Spot Testnet"). Futures belum terekam (butuh Demo key); Bybit ditunda karena pembatasan akses.
- [x] Paper live **24 jam** dan restart dengan data live: **lulus** 2026-10-18 (lihat "Hasil observasi 24 jam"). Catatan lama: Restart dengan data live dan posisi terbuka sudah lolos dua kali. Reboot host tak terduga (15:18–15:42 UTC) pulih otomatis. Observasi 24 jam dihitung ulang dari 15:42:27 UTC; nilai sampel setelah 2026-10-18 15:43 UTC. Retry recorder belum diuji ulang dengan pemutusan DB di VPS.
- [ ] Order spot/futures pada lingkungan pengujian Binance, native SL/TP, restart/reconciliation, kill switch. **Spot lolos di level adapter dan bot penuh `app.mode: testnet`** (entry, OCO, exit strategi, restart/reconciliation dengan posisi terbuka, kill switch dashboard, saldo non-bot tidak tersentuh). Tersisa: fee non-nol, run panjang, dan Futures (kredensial Demo belum disiapkan). Bybit ditunda.
- [x] Backtest historis nyata, split CLI, `-save`, CSV, dan perbandingan awal tiga parameter `emacross` (hasil di atas).
- [ ] Validasi strategi: grid emacross dan validasi DCA/grid 3 tahun selesai (lihat "Validasi emacross lebih luas" dan "Validasi DCA dan grid"); **tidak ditemukan edge yang konsisten**. Tersisa: keputusan user soal parameter default/strategi sebelum Fase 11, fill intrabar lebih halus dan funding historis.
- [ ] Telegram: **diuji nyata 2026-10-17** dengan bot testnet di laptop dan whitelist chat user. Notifikasi start/entry terkirim; 12 perintah (`/status`, `/positions`, `/balance`, `/pnl`, `/orders`, `/strategies`, `/pause`, `/resume`, `/killswitch` + `confirm`, `/reset confirm`, `/stop confirm`) diterima dan bekerja. Pesan "Bot stopped" gagal (`context canceled`) karena shutdown memotong pesan yang sedang dikirim; diperbaiki (`fix(telegram): finish sending a message when shutdown begins`), belum diuji ulang dengan bot nyata. Tersisa: daily summary (00:00 UTC), uji whitelist dengan chat lain, dan Telegram di VPS (chat ID dimasukkan ke config server, bukan repo).
- [x] Browser desktop light + mobile dark: HTMX/CSP, konfirmasi kontrol, chart candle/marker/equity dan halaman backtest dengan database test. Variasi tampilan lainnya dapat diperluas.
- [ ] Putuskan universe saham ter-tokenisasi bersama user. Risk live dan persetujuan Fase 11 tetap belum diputuskan.

### Penutupan Fase 9 (2026-10-18)

Fase 9 ditutup untuk cakupan aktif atas permintaan user. Status akhir per area:

| Area | Status | Bukti |
|---|---|---|
| Unit/contract/integration test, race, lint | Lolos; CI menjalankan semuanya per push | GitHub Actions |
| Paper live 24 jam di VPS, restart, reboot host | **Lolos** | "Hasil observasi 24 jam" |
| Binance Spot Testnet: adapter + bot penuh (OCO, restart, kill switch, saldo non-bot) | **Lolos** | "Order nyata di Binance Spot Testnet", "Bot penuh mode testnet" |
| Telegram (notifikasi, 12 perintah) | **Lolos** di testnet; aktif di VPS | "Sisa Fase 9" item Telegram |
| Dashboard browser, monitoring/alert | Lolos | browser/monitoring JSON |
| Backtest dan riset strategi (emacross, tren, DCA, grid) | Selesai; **tidak ada edge** | JSON riset di `docs/testing/` |
| Universe: saham ter-tokenisasi | **Dikecualikan** (keputusan user) via tag `bStocks`; uji nyata: SNDKB/SOXLB tidak lagi terpilih | `feat(config): exclude tokenized stocks…` |
| Audit: order ambigu, DCA restart, komisi base asset, replay setelah reconcile, grid/mm restart | Selesai; bug diperbaiki dengan test | "Sisa Fase 9" item audit |

**Dipindahkan ke Fase 10/11 (bukan lolos):**
- **Futures Binance**: ditunda (keputusan user 2026-10-18) sampai kredensial Futures Demo tersedia; wajib diuji (order, algo SL/TP, listenKey, reconciliation, rekaman fixture) sebelum futures dipakai.
- **Bybit dan `xspread`**: ditunda karena regulatory restriction.
- **Strategi**: belum ada yang layak live; riset lanjut (keputusan user). Fase 11 tidak boleh dimulai dengan strategi tanpa edge.
- **Verifikasi tertunda waktu**: ringkasan harian Telegram pertama dari VPS (07:00 WIB 2026-10-19), konfirmasi user atas pesan "Bot stopped"/"Bot started" dari VPS, uji whitelist dengan chat lain.
- **Fee non-nol nyata**: testnet berkomisi 0; jalur komisi base asset hanya teruji unit/fixture. Uji di Fase 11 dengan nominal kecil.
- **Risk live dan persetujuan Fase 11**: batas risk konkret masih harus diputuskan user; risk default (kill switch drawdown 20%) terbukti tidak cocok untuk DCA.
- Backup di luar server: setelah live (keputusan user).

## Fase 10: uji multi-strategi Spot Testnet (2026-10-18 → dihentikan 2026-10-20)

**Keputusan terbaru user, 20 Oktober 2026:** hentikan lebih awal dan analisis
karena hasil di bawah ekspektasi. Bot VPS sudah stop normal **18.52.43 WIB**,
exit 0; tujuh order tersisa dibatalkan dan exchange mengonfirmasi nol open order.
Inventori testnet, DB dan backup dipertahankan. Timer observasi disabled/inactive;
timer backup dan infrastruktur penyimpanan tetap aktif. Target 21 Oktober
23.43 WIB dibatalkan, bukan uji 72 jam yang lulus. Baseline proyek lain tetap
`differences: []`.

[Analisis penutupan dan bukti](testing/testnet-stop/analysis.md):
43 jam 9 menit, 521 sampel observasi tanpa masalah menurut script, ekuitas virtual
akhir 10.082,4309 USDT (+0,8243% terhadap 10.000), sebelum fee live. EMA ETH rugi,
SOL grid menyumbang laba terbesar, MM tipis dan DCA belum selesai satu siklus.
Ada kegagalan proteksi OCO dan penyesuaian inventori DOGE yang perlu diaudit.
Langkah berikutnya: riset strategi lain serta pengembangan keandalan transaksi,
rekonsiliasi PnL dan penutupan eksperimen. Belum ada strategi baru dijalankan;
referensi ETH tetap calon riset, bukan keputusan live.

Keputusan user 2026-10-18: fokus Binance, jalankan semua strategi spot selama 3 hari di testnet, lalu analisis sebelum memutuskan go live. Catatan: testnet tanpa fee (paper menunjukkan fee = 84% kerugian emacross 1m), jadi hasil untung/rugi testnet bukan dasar keputusan live; yang dinilai adalah perilaku operasional.

- Stack VPS dialihkan dari paper ke testnet (data paper tetap di DB; config paper disimpan sebagai `configs/config.paper.yaml` di server). Kredensial Spot Testnet di `.env.vps` (disalin lewat stdin SSH, tidak ditampilkan), `TB_APP__MODE=testnet`. Backup sebelum peralihan: `backup-20261018T164241Z`. Rilis `7cc7096`, image `sha256:0070bdc06e384868963a3c4a3392101a84ba66df4d634a63cf6b6e7f27ece3eb`.
- Config [config.testnet.example.yaml](../configs/config.testnet.example.yaml), satu strategi per pair: `ema-trend-btc` (1h 9/21, tren 100), `ema-fast-eth` (1m 12/26), `grid-sol` (87–117, 20 level), `dca-xrp` (20 USDT/jam, maks 24), `mm-doge` (spread 30 bps). Riwayat testnet hanya sejak 2026-10-07 (testnet di-reset berkala), jadi EMA tren 200 di 4h tidak akan siap; dipakai 1h.
- Uji lokal sebelum deploy menemukan **grid tidak pernah memasang order pada start live/testnet**: grid memulai diri pada candle warmup/backfill pertama, engine membuang intent dari candle lama, dan grid tidak mencoba lagi. Diperbaiki (`fix(strategy): start the grid on a live candle…`); di VPS grid langsung memasang 11 order beli. Backtest grid tidak berubah (71 trade, −0,14% pada kuartal uji).
- Pesan Telegram ditulis ulang dalam bahasa Indonesia untuk pemula: mode dijelaskan (uang asli/uji), entry menampilkan SL/TP beserta jarak % dan perkiraan rugi maksimal/target untung, fill DCA/grid diumumkan, hasil trade dengan % dan lama posisi, alasan dalam kalimat, ringkasan harian per strategi, `/help` berkelompok. Konfirmasi user: pesan "Bot stopped" dari VPS terkirim (perbaikan 2026-10-17 terbukti).
- Script observasi kini membaca mode dari `.env.vps` dan menilai gap/umur candle per timeframe untuk exchange aktif; sampel pertama `problems: []`. Baseline 28 container dan 5 service lain tidak berubah.
- **USD/IDR di Telegram** (permintaan user 2026-10-19 00:00 WIB, rilis `69d8dcb`, image `sha256:166ffc59c471e97717ba361a990e180836fc658a5d792175083294c3f20a1836`): entry, fill, trade selesai, ringkasan harian, `/status`, `/positions`, `/balance`, dan `/pnl` menampilkan nominal USDT beserta `≈ $… · Rp …` dan baris kurs (sumber dan jam WIB). Kurs dari [internal/fx](../internal/fx/fx.go): CoinGecko (harga USDT dalam USD dan IDR), cadangan Frankfurter/ECB (USD→IDR, dengan asumsi 1 USDT ≈ 1 USD), diperbarui tiap jam, kurs terakhir yang berhasil tetap dipakai bila sumber gagal (log WARN `exchange rates unavailable`). Bila belum ada kurs sama sekali, pesan menulis bahwa nilai hanya dalam USDT. Kurs hanya untuk pesan, tidak memengaruhi trading. Matikan dengan `telegram.fx: false`. Deploy 2026-10-18 17:10 UTC (restart di dalam jendela uji, run 7 → 8).
- **Temuan saat deploy itu:** restart dalam satu bar membuat candle warmup terakhir masih "segar", sehingga strategi bertindak **sebelum** rekonsiliasi pertama (ledger kosong, order sesi lama belum diadopsi). Grid mencoba memasang tangga kedua (11 order); kebetulan semuanya ditolak risk limit karena ekuitas ledger kosong hanya 0,55, jadi tidak ada order ganda di exchange. Diperbaiki di `f709429` (`fix(bot): act on fresh warmup candles only after the first reconcile`): warmup menahan candle segar dan menjalankannya setelah rekonsiliasi; regression test `TestFreshWarmupCandleRunsAfterReconcile`. Deploy 17:23 UTC (run 8 → 9): log `strategies warmed up held=10`, 14 order diadopsi, grid tetap 11 order, tanpa WARN, `problems: []`, baseline tidak berubah.
- Catatan untuk analisis: setiap start mengirim ERROR `open position without stop loss` untuk posisi DCA XRP. DCA memang tanpa stop loss, jadi peringatan ini wajar tetapi berisik di Telegram; pertimbangkan pengecualian untuk strategi tanpa stop.
- Yang dinilai saat analisis: crash/restart/OOM, error dan WARN, perilaku tiap strategi (fill grid dan order balasan, siklus DCA dan take profit, requote dan inventori MM, sinyal emacross dengan OCO), kill switch/risk limit, rate limit, notifikasi Telegram dan ringkasan harian, konsistensi saldo bot vs exchange.

## Backlog testing (diisi selama Fase 2–8, dikerjakan di Fase 9)

- Uji adapter Binance terhadap mainnet setelah DNS lokal diperbaiki (lihat catatan jaringan di bawah).
- Universe spot: putuskan dan uji pengecualian saham ter-tokenisasi (lihat pertanyaan terbuka).
- **Fase 2**
  - `indicator`: nilai EMA/SMA/RSI/ATR/MACD/Bollinger dibandingkan referensi (TA-Lib/TradingView); seed EMA dari SMA; RSI saat loss = 0.
  - `portfolio.Account`: average entry, partial close, flip posisi, fee pro-rata ke trade, funding (long bayar saat rate positif), `Snapshot` (equity, used margin).
  - `broker.Sim`: aturan fill (market di open bar berikutnya, gap di stop/TP, limit sebagai maker, urutan stop sebelum TP), order hanya cocok dengan candle yang open ≥ waktu order, slippage, fee.
  - `engine`: entry → SL/TP terpasang setelah fill; satu leg kena → leg lain dibatalkan; reversal (exit lalu entry); trailing stop bergeser hanya ke arah untung; stop yang sudah terlewati → exit market; stop ditolak → posisi ditutup; antrean event broker (tanpa deadlock saat broker memanggil handler secara sinkron); `CloseAll`; `SetPaused`.
  - `risk.Size`: sizing berbasis jarak stop, fallback `position_pct`, cap `max_position_pct` dan free margin, pembulatan lot, tolak di bawah minimum.
  - `backtest`: urutan event (TF eksekusi sebelum TF strategi pada close time yang sama), funding tiap 8 jam, metrik (CAGR, MDD, Sharpe/Sortino, profit factor ∞ → `null` di JSON), split in-sample/out-of-sample, CSV, `-save` ke DB.
  - Migrasi `00002` up/down; `config` men-decode `strategies[].params`, map `backtest.fees` di-merge dengan default.
  - Golden test `emacross` dan eksperimen parameter default.
- **Fase 3**
  - `risk.CheckOrder`: `max_open_positions` (hanya untuk entry simbol baru), `max_exposure_pct`; kill switch `Observe`: rugi harian sejak 00:00 UTC, drawdown dari puncak, trip sekali, `Reset`/`Halt`; `close_on_kill` menutup semua posisi dan membatalkan order entry/strategi.
  - Engine: candle basi (close lebih dari 1 bar yang lalu) tetap masuk ke strategi tapi intent-nya dibuang; `AdoptOrders` membangun ulang proteksi dari order SL/TP; `Unprotected`.
  - Recorder: urutan tulis (order sebelum fill-nya), antrean penuh, flush saat shutdown; `UpsertOrder` memindahkan `run_id` order yang diadopsi ke sesi aktif.
  - Paper: `bot run` mode paper dengan strategi aktif. Cek warmup dari DB (300 bar), fill dari tick harga live, resume sesi (`paper.resume`: saldo, posisi, order terbuka), snapshot equity tiap menit, `FinishRun` saat shutdown. Uji restart di tengah posisi.
  - Migrasi `00003` up/down.
- **Fase 4** (semua di testnet dulu; banyak perilaku API yang hanya diketahui dari dokumentasi/connector, belum pernah dicoba)
  - Signature HMAC REST (query string) dan WS API (param diurutkan alfabetis) — cek terhadap contoh di dokumentasi (`openssl dgst`).
  - Spot user data: `userDataStream.subscribe.signature` di `wss://ws-api.binance.com:443/ws-api/v3` (testnet `ws-api.testnet.binance.vision`); respons subscribe; event `executionReport` dengan semua key kembar; resubscribe setelah reconnect.
  - Futures user data: `listenKey` + `wss://fstream.binance.com/private/stream?streams=<key>`. **URL private testnet belum pasti** (`fstream.binancefuture.com/stream`?). Keepalive 30 menit, `listenKeyExpired` → `Kick`.
  - Futures SL/TP lewat **Algo Order** (`POST /fapi/v1/algoOrder`, `algoType=CONDITIONAL`, `triggerPrice`, `workingType=MARK_PRICE`, `priceProtect=true`). Belum pasti: format `ai` (string/angka), apakah order hasil trigger membawa `clientOrderId` sendiri, urutan `ALGO_UPDATE` vs `ORDER_TRADE_UPDATE` (fill ditahan ≤10 detik menunggu mapping), dan status `FINISHED` tanpa eksekusi.
  - Spot OCO `POST /api/v3/orderList/oco` (above TAKE_PROFIT / below STOP_LOSS untuk SELL); cancel satu leg membatalkan list (cancel leg kedua → -2011, diabaikan).
  - `SetLeverage` (isolated, -4046/-4048), `EnsureOneWay` (-4059), sinkronisasi jam dan -1021.
  - Reconciliation: posisi futures dari `positionRisk` v3, saldo dari `/fapi/v3/balance` (`balance` = wallet), spot dari saldo base asset; koreksi hanya setelah 2 kali selisih berturut-turut; order `tb*` diadopsi dengan tag dari DB; `SyncOrders` memulihkan fill yang terlewat (fee 0).
  - Fee spot dalam base asset (qty posisi < qty fill) dan fee BNB (tercatat 0) → dikoreksi lewat reconciliation; pastikan tidak ada drift.
  - Guard: mode `live` ditolak tanpa `app.allow_live: true`; mode `testnet` ditolak jika exchange tidak `testnet: true`.
- **Fase 5**
  - Telegram: long polling, whitelist `chat_ids` (chat lain diabaikan), escape HTML, pesan > 4096 karakter dipotong, antrean penuh, flush pesan "Bot stopped" saat shutdown, token tidak pernah muncul di log/error.
  - Perintah: `/status`, `/positions`, `/balance`, `/pnl`, `/orders`, `/strategies`, `/pause`, `/resume` (ditolak saat kill switch aktif), `/killswitch confirm`, `/reset confirm`, `/stop confirm`, `/help`.
  - Notifikasi: entry terisi, trade ditutup (PnL), notice engine ≥ `notify_level`, ringkasan harian 00:00 UTC.
  - Metrics trading (`tradingbot_*`) terisi dengan benar; dashboard Grafana termuat (panel Prometheus + tabel TimescaleDB); alert Prometheus menyala sesuai kondisi (promtool hanya mengecek sintaks).
- **Fase 6**
  - Dashboard di browser (desktop + HP, light/dark): overview, partial auto-refresh, tombol pause/resume/kill/reset (konfirmasi), chart candle dengan marker trade (posisi marker di bar yang benar), chart equity, daftar dan detail backtest.
  - Keamanan: basic auth saat `TB_WEB__PASSWORD` di-set; validasi menolak listen non-localhost tanpa password; aksi `POST` tanpa header `HX-Request` → 403; header CSP/X-Frame-Options; CSP tidak memblokir htmx/lightweight-charts.
  - Mode tanpa sesi (hanya market data): halaman tetap jalan, tanpa panel live.
- **Fase 7 — ditunda karena pembatasan akses Bybit** (format publik sudah dilihat 2026-10-17; semua jalur private belum pernah dijalankan)
  - Contract test bersama untuk kedua adapter (Binance dan Bybit) (`exchange.MarketData` + `broker.Live`) dengan fixture terekam.
  - Bybit: kline REST dibalik ke oldest-first, `confirm` di WS, subscribe maksimal 10 arg per request (spot), ping 20 detik; sign REST `ts+key+recvWindow+query|body`; auth WS `GET/realtime`; `triggerDirection` stop vs TP; spot `orderFilter=StopOrder` (cancel juga butuh `orderFilter`); `marketUnit=baseCoin`; fee spot buy dalam base; status `Deactivated`/`PartiallyFilledCanceled`; set margin mode isolated untuk UTA (account-wide!); kode error 110043/110025/110001/170213.
  - ID order engine sekarang alfanumerik (`tb` + hash 7 karakter + sequence) — pastikan diterima kedua exchange dan tidak bentrok.
  - Wallet unified (Bybit): sesi menolak dua market di exchange yang sama.
- **Fase 8**
  - `grid`: level aritmatika/geometris, buy awal di bawah harga, order balasan setelah fill penuh (pemetaan harga ke level setelah pembulatan tick), restart tidak menggandakan order (cek `OpenOrders`), `stop_below` membatalkan semua dan exit.
  - `dca`: jadwal beli (`interval`, `dip_pct`, `max_buys`), TP/SL dari harga rata-rata, siklus baru setelah exit, timer berjalan di backtest (`eng.Tick`).
  - `funding`: data funding (Binance `premiumIndex`, Bybit tickers) di-poll tiap 5 menit; buka/tutup dua kaki; kaki timpang dibongkar setelah grace; tidak bisa di-backtest (tidak ada histori funding).
  - `xspread` **ditunda untuk pengujian exchange nyata**: arah posisi benar untuk spread positif/negatif, cooldown saat order dalam perjalanan, butuh dua broker live di mode testnet/live. Unit test sintetis tetap dipertahankan; tidak bisa divalidasi dengan Binance saja.
  - `mm`: requote (CancelAll + ref baru) hanya saat harga bergerak/fill/tanpa order, skew inventori, batas inventori, spot tidak menjual tanpa inventori. Backtest market making memakai fill limit `Sim` yang terlalu optimistis (tanpa antrean order book).
  - Legs: routing candle per instrumen di engine, akun dan subscription untuk semua leg, cek wallet unified ikut menghitung leg, backtest multi-akun (equity total, `InitialBalance` per akun).
  - Risk `max_exposure_pct` menghitung kedua kaki strategi hedged sebagai exposure; sesuaikan batas atau ubah perhitungan.

## Pertanyaan terbuka (belum diputuskan)

- **Strategi untuk Fase 11:** emacross, emacross + filter tren, DCA, dan grid belum menunjukkan edge (lihat bagian validasi). Keputusan user 2026-10-18: riset lanjut dulu. Analisis uji testnet 3 hari menjadi masukan operasional, bukan bukti profit.
- **Batas risk live** (max drawdown %, rugi harian %, leverage, modal awal). Usulkan nilai konservatif lalu minta persetujuan user. Risk default (kill switch drawdown 20%) terbukti merusak DCA karena menjual di titik terendah; risk perlu disesuaikan per strategi.
- **Bybit**: menunggu kabar user. **Futures Binance**: menunggu Futures Demo key.
- (Diputuskan 2026-10-18) Saham ter-tokenisasi dikecualikan lewat tag `bStocks` (`universe.exclude_tags`).

## Catatan implementasi (penyimpangan dari plan rinci di bawah)

Bagian ini lebih baru daripada plan rinci. Kalau ada konflik, ikuti bagian ini.

- Module Go: `github.com/rizz404/trading-bot`.
- `flake.nix` diganti `.mise.toml` (lihat aturan kerja di atas).
- Migrasi memakai **goose sebagai library**. File SQL di [migrations/](../migrations/) di-embed ke binary, lalu dijalankan dengan `bot migrate up|down|status`. CLI goose tidak diperlukan. `bot run` menolak start kalau masih ada migrasi pending.
- **sqlc ditunda** sampai Fase 1, ketika query pertama dibutuhkan. Pertimbangkan pgx biasa kalau query-nya sedikit.
- Config: defaults → YAML (`configs/config.yaml`, di-gitignore) → `.env` → environment proses. Key env: prefix `TB_`, pemisah nesting `__` (contoh: `TB_EXCHANGES__BINANCE__API_KEY`). `api_key`/`api_secret` **ditolak** kalau ada di YAML. Mode `testnet` mewajibkan `testnet: true` di setiap exchange yang aktif, dan mode `live` mewajibkan `testnet: false`. Lihat [internal/config/config.go](../internal/config/config.go).
- Tipe `config.Secret` tidak pernah tercetak (`***`), baik lewat fmt maupun slog. Pakai `.Reveal()` hanya ketika nilai aslinya benar-benar dibutuhkan.
- Metrics: `/metrics` dan `/healthz` (ping DB) di `127.0.0.1:9100`. Prometheus di compose men-scrape `bot:9100` (bot di container, `--profile bot`) dan `host.containers.internal:9100` (bot di host). Di rootless podman, target host hanya `up` kalau bot listen `0.0.0.0` (`TB_METRICS__LISTEN=0.0.0.0:9100`), karena `host.containers.internal` tidak mengarah ke loopback host.
- Versi image yang di-pin: `timescale/timescaledb:2.30.2-pg17`, `prom/prometheus:v3.15.0`, `grafana/grafana:13.2.4`, build `golang:1.27.2-alpine`, runtime `distroless/static-debian12:nonroot`.
- **Jaringan dev (2026-10-17):** DNS lokal membelokkan semua domain Binance/Bybit ke `43.173.57.48` (diblokir). Koneksi lewat IP asli tembus, dan testnet Binance tidak diblokir. User memilih memperbaiki DNS sistem sendiri, jadi bot **tidak** punya resolver DoH bawaan. Sebelum DNS diperbaiki, uji mainnet hanya bisa lewat `curl --resolve` atau `binance.Options.HTTPClient` dengan dialer khusus.
- Grafana memprovisikan datasource `Prometheus` (uid `prometheus`) dan `TimescaleDB` (uid `timescaledb`). Dashboard JSON diletakkan di `deploy/grafana/dashboards/`.

### Fase 1 (market data)

- Interface dipecah: `exchange.MarketData` (publik: instruments, tickers, klines, stream) sekarang, dan interface order/akun terpisah di Fase 4. Lihat [internal/exchange/exchange.go](../internal/exchange/exchange.go).
- Endpoint Binance ada di [internal/exchange/binance/client.go](../internal/exchange/binance/client.go). **WS futures mainnet = `wss://fstream.binance.com/market/stream`**: URL lama `/stream` masih menerima koneksi, tapi tidak mengirim data. Testnet futures: REST `testnet.binancefuture.com`, WS `fstream.binancefuture.com/stream`.
- **Jebakan JSON:** `encoding/json` mencocokkan key tanpa membedakan huruf besar-kecil, sedangkan Binance memakai key kembar (`l`/`L`, `v`/`V`, `t`/`T`, `q`/`Q`, `e`/`E`). Setiap struct untuk pesan Binance wajib mendeklarasikan field untuk key huruf besar juga, supaya key itu tidak menimpa field huruf kecil secara diam-diam. Contohnya `klineMsg` di [stream.go](../internal/exchange/binance/stream.go).
- Data testnet disimpan dengan nama exchange `binance-testnet` (dari `MarketData.Name()`), jadi tidak tercampur dengan mainnet. `bot run` memakai data testnet hanya di mode `testnet`. Mode `paper` memakai data mainnet.
- `exchange.WSStream`: reconnect dengan backoff dan jitter, reconnect saat idle 60 detik (koneksi diam), dan reconnect proaktif sebelum 24 jam. Setelah setiap connect, stream mengirim `EventConnected`, lalu `marketdata.Feed` mem-backfill gap lewat REST sebelum memproses event berikutnya. `Feed.OnCandle` menerima setiap candle closed tepat sekali per seri, urut waktu. Ini titik sambung untuk engine strategi di Fase 3.
- Rate limit memakai `exchange.WeightLimiter` per market (budget 4800/menit spot, 1900/menit futures, di bawah limit resmi). Respons 429/418 men-pause limiter sesuai `Retry-After`.
- Universe (`marketdata.SelectUniverse`): quote cocok, status TRADING, base tidak ada di `exclude_bases`, volume quote 24 jam ≥ minimum, spread ≤ maksimum, diurutkan berdasarkan volume, lalu diambil top-N. Futures hanya `PERPETUAL`.
- Tidak ada agregasi timeframe di Go: setiap timeframe di-subscribe atau di-download langsung dari exchange. Itu lebih sederhana dan hasilnya identik dengan bar exchange. Agregasi 1m → timeframe besar untuk fill intrabar di backtest bisa ditambahkan di Fase 2 kalau dibutuhkan.
- sqlc tidak dipakai. Query cukup sedikit, jadi ditulis dengan pgx langsung di [internal/storage/marketdata.go](../internal/storage/marketdata.go), dengan decimal ↔ numeric lewat `pgx-shopspring-decimal`.
- Integration test DB: `make test-integration` (butuh `make up`), file `*_integration_test.go` dengan build tag `integration`.
- Batasan yang diketahui: kalau backfill satu seri gagal lalu candle live seri itu masuk, gap sebelumnya tidak diisi ulang otomatis. Isi lewat `datafetch` (aman diulang).

### Fase 2 (strategi dan backtest)

- **OMS ada di package `engine`**, bukan package `oms` terpisah ([internal/engine/orders.go](../internal/engine/orders.go)). Engine meneruskan candle ke strategi, men-sizing intent lewat `risk`, memasang SL/TP setelah entry fill, membatalkan leg lain saat satu leg kena, menggeser trailing stop saat candle close, dan menangani reversal.
- Event broker diantrekan, lalu diproses oleh pemegang lock engine (`unlockAndDrain`). Ini perlu karena broker (misalnya `Sim.CancelOrder`) boleh memanggil handler secara sinkron saat engine memegang lock.
- Strategi mengembalikan `strategy.Intent`: `Enter`/`Exit` untuk strategi arah, dan `Place`/`Cancel`/`CancelAll` untuk order mentah (grid, market making). Hook opsional: `FillHandler`, `PriceHandler` (candle yang belum close), `TimerHandler`. Strategi didaftarkan lewat `strategy.Register` dan diimpor oleh `internal/strategy/all`.
- `portfolio.Account` = satu wallet (exchange + market), karena wallet spot dan futures Binance terpisah. `portfolio.Book` menggabungkan semua akun.
- `broker.Broker` adalah antarmuka order. `BracketPlacer` dipakai untuk broker yang butuh SL+TP terhubung (OCO di Binance spot), dan `LeverageSetter` untuk futures. `broker.Sim` dipakai backtest dan nanti paper trading.
- Backtest tidak punya periode warmup terpisah: indikator mulai siap di dalam rentang `-from`. Funding memakai rate konstan (`backtest.funding_rate`), karena histori funding belum disimpan.
- Hasil `backtest -save` masuk ke `strategy_runs` (mode `backtest`, `summary` = metrik JSON), `trades`, dan `equity_snapshots` (sampel per jam).

### Fase 3 (risk, persistensi, paper)

- `bot run` sekarang punya **sesi trading** kalau ada strategi dengan `enabled: true` ([cmd/bot/session.go](../cmd/bot/session.go)). Satu sesi = satu baris `strategy_runs` dengan `instance_id = 'session'`, dan semua order/fill/trade/equity sesi ditulis ke run itu lewat `internal/recorder` (asinkron). Kolom `strategy` di tabel order dan posisi mencatat strategi pemiliknya.
- Paper restart membatalkan order `TagEntry` yang belum selesai karena rencana proteksinya belum disimpan. Posisi, native bracket, dan raw order strategi tetap dipulihkan. Sim mengisi sisa kuantitas order parsial, bukan seluruh kuantitas awal.
- Feed mencakup gabungan simbol dan timeframe dari `marketdata` dan dari strategi ([cmd/bot/feeds.go](../cmd/bot/feeds.go)). Nama exchange mengikuti mode: di mode `testnet` data, akun, dan order memakai nama `binance-testnet` (`app.ExchangeName`).
- Paper trading memakai `broker.Sim` dengan jam dinding. Setiap update candle yang belum close diperlakukan sebagai tick harga, dan candle close juga dicocokkan supaya lonjakan di antara tick tidak terlewat. Fee dan slippage diambil dari `backtest`.
- Kill switch hanya disimpan di memori: setelah restart, puncak equity dan equity awal hari dihitung ulang dari equity saat itu.
- Mode `testnet`/`live` dengan strategi aktif disambungkan ke LiveBroker di Fase 4.

### Fase 4 (Binance live/testnet)

- API Binance yang dipakai diverifikasi ke dokumentasi resmi (2026-10-17): **listenKey spot sudah dihapus 2026-02-20** → user data spot lewat WebSocket API; **order kondisional futures lewat Algo Order API** (`/fapi/v1/algoOrder`, event `ALGO_UPDATE`); user data futures di `/private/stream`. Sumber: `binance-spot-api-docs` (GitHub) dan `binance-connector-js` (di-generate dari OpenAPI resmi). Situs developers.binance.com dilindungi AWS WAF (tidak bisa diambil dengan curl).
- `broker.Live` adalah antarmuka broker exchange sungguhan ([internal/broker/live.go](../internal/broker/live.go)); `binance.Trader` mengimplementasikannya ([internal/exchange/binance/](../internal/exchange/binance/)). Bybit di Fase 7 mengikuti antarmuka yang sama.
- `bot run` mode `testnet`/`live`: satu broker per exchange (semua market di exchange itu), user stream jalan, reconciliation saat start (setelah warmup), tiap menit, dan setiap stream reconnect ([cmd/bot/reconcile.go](../cmd/bot/reconcile.go)). Exchange adalah sumber kebenaran.
- Order mainnet butuh **dua** saklar: `app.mode: live` dan `app.allow_live: true`.

### Fase 5 (Telegram dan observability)

- Telegram memakai HTTP Bot API langsung ([internal/notify/telegram](../internal/notify/telegram/)). Token hanya dari `TB_TELEGRAM__TOKEN`; `chat_ids` di YAML. Perintah berbahaya butuh argumen `confirm`. Perintah didaftarkan di [cmd/bot/commands.go](../cmd/bot/commands.go).
- Konversi USD/IDR di pesan (2026-10-19): `fx.Service` ([internal/fx](../internal/fx/fx.go)) berjalan hanya bila Telegram aktif dan `telegram.fx: true` (default). Tipe `money` di [cmd/bot/messages.go](../cmd/bot/messages.go) memformat `amount`/`change`/`rateLine`; rupiah memakai titik ribuan tanpa desimal.
- Metrics trading ada di [internal/metrics/trading.go](../internal/metrics/trading.go); sesi memperbarui gauge setiap 10 detik. Engine punya hook `OnPlaced` untuk latensi broker.
- Dashboard `deploy/grafana/dashboards/trading-bot.json` (uid `tradingbot`) dan alert `deploy/prometheus/alerts.yml`. Belum ada Alertmanager: alert terlihat di UI Prometheus, sedangkan notifikasi real-time dikirim bot lewat Telegram.

### Fase 6 (web dashboard)

- `internal/web`: server `net/http` + templ ([internal/web/views](../internal/web/views/), hasil generate `*_templ.go` ikut di-commit; regenerate dengan `make generate`, tool templ dipasang sebagai `go tool`). htmx 2.0.11 dan lightweight-charts 5.2.1 di-embed (`internal/web/static`), tanpa CDN saat runtime.
- Dashboard jalan di dalam `bot run` (`web.listen`, default `127.0.0.1:8080`). Sesi diakses lewat antarmuka `web.Live` ([cmd/bot/web.go](../cmd/bot/web.go)).
- Rute: `/` (overview), `/partials/*` (HTMX), `POST /actions/{pause,resume,kill,reset}`, `/chart?series=exchange:market:symbol:tf[&run=id]`, `/api/candles`, `/api/equity`, `/backtests`, `/backtests/{id}`.

### Fase 7 (Bybit)

- `internal/exchange/bybit` mengikuti pola Binance: `Client` (market data, `exchange.MarketData`) dan `Trader` (`broker.Live`). Sumber detail API: dokumentasi resmi Bybit (repo `bybit-exchange/docs`), diambil 2026-10-17.
- Simbol Binance dan Bybit memakai format asli exchange, misalnya `BTCUSDT`. Qty di seluruh bot selalu dalam satuan base.
- SL/TP Bybit memakai order kondisional (`triggerPrice`, `StopOrder` untuk spot). Order ini tidak mengunci saldo sebelum trigger, jadi SL dan TP dipasang sebagai dua order terpisah (tanpa OCO bawaan; engine membatalkan leg lainnya).
- `broker.UnifiedWallet`: Bybit (UTA) memakai satu wallet untuk spot dan futures; bot hanya memperdagangkan satu market pada Bybit dalam satu sesi.
- ID order engine memakai format alfanumerik yang konsisten antar-exchange.

### Fase 8 (strategi lanjutan)

- Strategi baru: `grid`, `dca`, `funding`, `xspread`, `mm` ([internal/strategy/](../internal/strategy/)); helper dua kaki di `internal/strategy/hedge`. Semua didaftarkan lewat `internal/strategy/all`.
- `strategy.Config.Legs` (YAML `strategies[].legs`) menambah instrumen di market/exchange lain; engine meneruskan candle semua instrumen ke strategi, dan sesi membuat akun + subscription untuk tiap leg. `strategy.Context` sekarang punya `LastPrice` dan `Funding`.
- Funding rate dari `exchange.FundingSource` (Binance, Bybit) di-poll sesi tiap 5 menit ke engine (`SetFunding`).
- Backtest sekarang multi-akun (satu `Sim` per exchange, satu akun per exchange+market, saldo awal per akun) dan menjalankan timer strategi.

---

# Plan Rinci: Bot Trading Crypto (Go)

## Context

Folder `/home/rizz404/kodingan/trading-bot` masih kosong. Tujuannya membangun bot trading crypto **otomatis penuh** di **Binance dan Bybit**, untuk **spot dan futures**, ditulis dalam **Go**. Belum ada strategi yang terbukti, jadi bot harus menjadi *platform eksperimen*. Strategi dimasukkan sebagai plug-in, dan kode strategi yang sama bisa diuji lewat backtest → paper → testnet → live dengan modal kecil (< $500).

Jangka panjang, bot mendukung indikator teknikal, grid/DCA, arbitrase, dan market making dengan timeframe campuran. **MVP fokus pada strategi indikator teknikal di Binance** untuk membuktikan seluruh pipeline: data → sinyal → risk → order → monitoring.

Keputusan yang sudah diambil:
| Aspek | Pilihan |
|---|---|
| Bahasa | Go |
| Exchange | Binance (MVP), lalu Bybit. Adapter ditulis sendiri di belakang satu interface |
| Pasar | Spot + futures (USDT-M perpetual), margin **isolated, one-way** sebagai default |
| Pair | Top 10–20 berdasarkan volume, lewat universe selector dengan filter likuiditas |
| Strategi | Kode Go yang mengimplementasikan interface, dengan parameter di YAML |
| Risk | SL/TP + trailing, batas rugi harian / max drawdown (kill switch), position sizing (% / ATR), batas leverage & exposure |
| Data | PostgreSQL + TimescaleDB |
| Kontrol | Telegram, web dashboard (templ + HTMX), CLI, Prometheus + Grafana |
| Testing | Backtest → paper → testnet → live mini |
| Deploy | GitHub private → CI/GHCR → VPS untuk paper/testnet Fase 9; laptop untuk pengembangan |

## Arsitektur

Engine berbasis event. Strategi tidak pernah memanggil exchange secara langsung. Strategi hanya menerima event market dan mengeluarkan **Signal/OrderIntent**. Intent itu melewati **Risk Manager**, lalu diteruskan ke **Broker**. Ada tiga implementasi Broker yang bisa ditukar:

- `SimBroker` untuk backtest: fill disimulasikan dari candle historis, lengkap dengan fee, slippage, dan funding.
- `PaperBroker` untuk paper trading: data live, fill disimulasikan.
- `LiveBroker`: order sungguhan ke exchange, baik testnet maupun mainnet.

Karena logika strategi dan risk **identik** di ketiga mode, hasil backtest sebanding dengan hasil live.

```
 Exchange WS/REST ──► MarketData Feed ──► Event Bus ──► Strategy (N instance)
                                              │               │ OrderIntent
                                              │               ▼
                                              │         Risk Manager ──(tolak)──► log/alert
                                              │               │ approved
                                              ▼               ▼
                                         Storage ◄──── Order Manager (OMS) ──► Broker (Sim/Paper/Live)
                                    (Timescale/PG)            │ fills, positions
                                              ▲               ▼
                          Telegram / Web / Prometheus ◄── Portfolio / PnL
```

### Struktur folder

```
trading-bot/
  cmd/
    bot/            # main: jalankan engine (mode: paper|testnet|live)
    backtest/       # CLI backtest + laporan
    datafetch/      # download candle historis ke Timescale
  internal/
    domain/         # tipe inti: Symbol, Candle, Order, Fill, Position, Side, MarketType (pakai decimal)
    exchange/       # interface Exchange + tipe umum, rate limiter, ws reconnect helper
      binance/      # spot + usdm futures (REST + WS)
      bybit/        # (fase 7)
    marketdata/     # feed candle/ticker, agregasi multi-timeframe, universe selector (top-N volume)
    indicator/      # EMA, SMA, RSI, MACD, ATR, Bollinger (streaming/incremental)
    strategy/       # interface Strategy + registry
      emacross/     # strategi contoh MVP
    risk/           # pre-trade checks, sizing, kill switch, exposure limits
    oms/            # order manager: lifecycle order, SL/TP/trailing, reconciliation
    broker/         # SimBroker, PaperBroker, LiveBroker
    portfolio/      # posisi, saldo, PnL realized/unrealized, equity curve
    backtest/       # runner, model fee/slippage/funding, metrik (Sharpe, MDD, win rate, PF)
    storage/        # pgx pool, migrasi goose (embed); query sqlc menyusul
    notify/telegram/
    web/            # templ + HTMX + lightweight-charts
    metrics/        # Prometheus collectors
    config/         # load YAML + env (secret dari env, bukan file)
  migrations/
  configs/
    config.example.yaml
    strategies/emacross.yaml
  deploy/
    docker-compose.yml   # timescaledb, prometheus, grafana
    grafana/dashboards/
  .mise.toml             # versi Go (pengganti flake.nix, lihat catatan implementasi)
```

### Interface inti (sketsa)

```go
// exchange
type Exchange interface {
    Name() string
    Instruments(ctx, MarketType) ([]Instrument, error) // tick size, lot size, min notional
    Klines(ctx, sym, tf, from, to) ([]Candle, error)
    Tickers24h(ctx, MarketType) ([]Ticker, error)      // untuk universe selector
    SubscribeKlines(ctx, []Sub) (<-chan Candle, error)
    SubscribeUserData(ctx) (<-chan UserEvent, error)   // fills, order update, balance
    PlaceOrder(ctx, OrderRequest) (Order, error)
    CancelOrder(ctx, ...) error
    OpenOrders(ctx, ...) ([]Order, error)
    Balances(ctx, MarketType) ([]Balance, error)
    Positions(ctx) ([]Position, error)                 // futures
    SetLeverage(ctx, sym, int) error
    SetMarginMode(ctx, sym, MarginMode) error
}

// strategy
type Strategy interface {
    Name() string
    Init(ctx StrategyContext, params map[string]any) error
    Timeframes() []Timeframe
    OnCandle(c Candle) []OrderIntent
    OnFill(f Fill)
}
```

### Prinsip teknis penting
- **Semua harga dan kuantitas memakai `shopspring/decimal`**, tidak pernah `float64`. Indikator boleh float.
- **Pembulatan** mengikuti tick size, lot size, dan min notional dari `Instruments()` sebelum order dikirim.
- **Rate limiter** per exchange (token bucket, mengikuti header weight Binance) dan **WS auto-reconnect** dengan resubscribe serta backfill gap candle lewat REST.
- **Sinkronisasi jam** dengan server time exchange untuk request yang di-sign.
- **Reconciliation saat startup dan berkala**: state order dan posisi di DB dicocokkan dengan exchange. Exchange adalah sumber kebenaran.
- **SL/TP dipasang di exchange** (stop order native), bukan hanya dipantau oleh bot. Posisi tetap terlindungi kalau bot mati.
- **Idempotensi**: `clientOrderId` deterministik supaya retry tidak membuat order ganda.
- **Keamanan**: API key dari env/secret file. Izin key: trade saja, **tanpa withdraw**, dengan IP whitelist. Setiap environment (testnet atau live) memakai key terpisah.
- **Kill switch**: kalau rugi harian atau drawdown melewati batas, bot membatalkan order, menutup posisi sesuai config, menghentikan entry baru, lalu mengirim alert Telegram.
- Logging memakai `log/slog` dalam format JSON.

## Library yang dipakai
- `github.com/shopspring/decimal`: aritmatika uang
- `github.com/jackc/pgx/v5`, `sqlc`, `pressly/goose`: database dan migrasi
- `github.com/coder/websocket` (atau `gorilla/websocket`): WebSocket
- `github.com/knadh/koanf`: config YAML + env
- `github.com/go-telegram/bot`: Telegram
- `github.com/prometheus/client_golang`: metrics
- `github.com/a-h/templ` + HTMX + TradingView `lightweight-charts`: dashboard
- `golang.org/x/time/rate`: rate limit
- `github.com/stretchr/testify`: test

## Fase pengerjaan

Fase 0–1 dikerjakan dengan test langsung. Mulai Fase 2, ikuti "Aturan fase build": bangun dulu, test di Fase 9, perbaiki di Fase 10.

**Fase 0 · Fondasi** (selesai)
- `go mod init`, `.mise.toml`, `docker-compose` (TimescaleDB, Prometheus, Grafana), config loader, slog, kerangka Makefile/justfile.
- Migrasi awal: `candles` (hypertable), `instruments`, `orders`, `fills`, `positions`, `equity_snapshots`, `strategy_runs`.

**Fase 1 · Domain + Binance market data** (selesai)
- Tipe domain, interface `Exchange`, adapter Binance (spot + USDⓈ-M futures): instruments, klines REST, tickers 24h, WS kline.
- `cmd/datafetch`: download candle historis ke Timescale.
- Universe selector: top-N berdasarkan quote volume, dengan filter min volume, max spread, dan exclude stablecoin/leveraged token.

**Fase 2 · Indikator + Strategy + Backtester**
- Indikator streaming: EMA, SMA, RSI, MACD, ATR, Bollinger.
- Interface strategi + registry. Strategi contoh `emacross` (EMA cross + filter RSI, SL/TP berbasis ATR).
- `SimBroker` dengan fee maker/taker, slippage (bps), dan funding rate untuk futures.
- `cmd/backtest`: metrik total return, CAGR, max drawdown, Sharpe/Sortino, win rate, profit factor, jumlah trade. Equity curve diekspor ke CSV/DB.
- Dukungan walk-forward / out-of-sample split sederhana untuk menghindari overfitting.

**Fase 3 · Risk + OMS + Paper trading**
- Risk manager: sizing (% equity atau berbasis ATR), max leverage, max exposure per aset/exchange/total, max posisi terbuka, batas rugi harian dan drawdown (kill switch).
- OMS: lifecycle order, SL/TP/trailing, persist ke DB.
- `PaperBroker` dan `bot run` mode `paper` yang menyambungkan `marketdata.Feed.OnCandle` ke strategi.

**Fase 4 · Binance LiveBroker**
- Signed REST (HMAC), order endpoint (spot + futures), user data stream (listenKey + keepalive), set leverage/margin isolated, stop order native.
- Reconciliation saat startup dan berkala, `clientOrderId` idempoten, penanganan error/reject.
- Mode `testnet` dan `live` di `bot run` (live tetap dijaga validasi config; order mainnet baru dijalankan di Fase 11).

**Fase 5 · Telegram + Prometheus/Grafana**
- Notifikasi: entry/exit, SL/TP kena, error, kill switch, ringkasan harian.
- Perintah: `/status`, `/positions`, `/balance`, `/pnl`, `/pause`, `/resume`, `/stop`, `/killswitch`. Hanya `chat_id` di whitelist yang bisa memakainya.
- Metrics: equity, PnL, latensi order, WS reconnect, error rate, rate-limit usage. Dashboard Grafana dan alert rules.

**Fase 6 · Web dashboard**
- templ + HTMX: overview equity, posisi terbuka, riwayat trade, chart candle dengan marker entry/exit (lightweight-charts), daftar strategi dan statusnya, hasil backtest.
- Auth sederhana (password/token). Default bind ke localhost.

**Fase 7 · Bybit**
- Adapter untuk interface yang sama (spot + linear perp), termasuk endpoint testnet.

**Fase 8 · Strategi lanjutan**
- Grid / DCA (manajemen banyak order terbuka).
- Arbitrase: cross-exchange, dan spot-perp funding rate (basis/cash-and-carry).
- Market making: butuh WS order book L2, manajemen inventori, dan latensi rendah.

**Fase 9 · Testing (semua fitur)**
- Kerjakan seluruh "Backlog testing" dan semua butir di bagian "Verifikasi" di bawah.
- Unit test: indikator (dibandingkan nilai referensi), sizing/risk, kill switch, OMS, SimBroker (fee/slippage/funding), metrik backtest, strategi.
- Contract test adapter Binance dengan fixture respons REST/WS yang direkam, ditambah integration test opsional ke testnet dengan build tag `integration`. Bybit dan pengujian exchange nyata `xspread` ditunda sesuai kendala akses terbaru; test lokal yang sudah ada tetap dipertahankan.
- Golden test backtest `emacross`, lalu eksperimen backtest untuk menentukan parameter default.
- Uji end-to-end: paper run 24 jam, order sungguhan di Spot Testnet/Futures Demo Binance, reconciliation, Telegram, dan dashboard. Pengujian exchange nyata Bybit dan arbitrase lintas exchange ditunda.
- Hasil: daftar temuan (bug, perilaku salah, celah keamanan) dicatat di plan, diurutkan berdasarkan tingkat keparahan.

**Fase 10 · Fixing**
- Perbaiki semua temuan Fase 9. Prioritas: keamanan dana (order, risk, kill switch, secret) → integritas data → sisanya.
- Setiap perbaikan disertai test yang membuktikan bug-nya hilang. Ulangi test terkait sampai semuanya lolos.

**Fase 11 · Live mini**
- Hanya setelah Fase 10 bersih **dan** user menyetujui secara eksplisit.
- Live di Binance dengan modal sangat kecil dan batas risk ketat. Strategi baru naik ke tahap ini setelah lolos backtest dan minimal 1–2 minggu paper/testnet.
- Gunakan hosting VPS yang sudah diverifikasi saat paper/testnet Fase 9; tinjau kembali backup DB, heartbeat, dan konfigurasi khusus live.

## Catatan realistis
- **Modal < $500 + 10–20 pair + futures**: min notional per order (sekitar 5–100 USDT tergantung pair dan exchange) membuat modal cepat terbagi tipis. Saat live awal, sebaiknya batasi ke 2–5 pair aktif sekaligus. Universe 20 pair tetap dipakai untuk backtest dan screening.
- **Market making dan arbitrase cross-exchange** praktis tidak menguntungkan dengan modal < $500 karena fee tier dan saldo harus terpecah di beberapa exchange. Fitur ini tetap ada di roadmap, tapi diprioritaskan terakhir.
- Pastikan akses dan legalitas exchange dari Indonesia sudah kamu cek sendiri. Ini di luar cakupan kode.

## Verifikasi (dikerjakan di Fase 9)
- `go test ./...` untuk unit test indikator (dibandingkan nilai referensi), sizing/risk, pembulatan tick/lot, dan agregasi timeframe.
- Contract test adapter exchange dengan fixture respons REST/WS yang direkam. Integration test opsional ke testnet, dengan build tag `integration`.
- Golden test backtest: `emacross` di BTCUSDT pada rentang tetap harus menghasilkan metrik yang deterministik.
- Paper run 24 jam: tanpa panic, WS reconnect teruji (putuskan jaringan secara manual), tidak ada gap candle, dan state DB konsisten.
- Testnet: kirim order spot dan futures sungguhan. Cek SL/TP native terpasang di exchange, restart bot di tengah posisi, lalu pastikan reconciliation memulihkan state. Picu kill switch dengan batas rugi buatan.
- Telegram, `/metrics` Prometheus, dan dashboard web dicek manual.
- `golangci-lint run` bersih.
