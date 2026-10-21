# Penarikan trial dan pengelolaan riset strategi — 21 Oktober 2026

## Keputusan terbaru

**Pembaruan 22 Oktober:** setelah persetujuan user, paper riset baru dimulai
21 Oktober 23:58:57 WIB: tiga instance Tren/sembilan binding BTC/ETH/SOL,
Rp7,5 juta virtual total pada DB baru. Ini uji operasional dua mode strategi
yang sudah tersedia, bukan hasil seleksi dari matriks backtest di bawah;
backtest portofolio baru belum dijalankan. Rancangan 24/72 tidak diaktifkan.
Sampel awal CPU bot mendekati batas 0,5 core; jangan naik tahap sebelum
mengurangi akses ledger berulang dan mengukur ulang. Detail, hash dan jadwal
review ada di [handover VPS](handover-vps.md). Riwayat penarikan berikut tetap berlaku
untuk ledger ETH lama saja.

User meminta menarik strategi yang berjalan dan kembali melakukan riset serta
mengelola pilihan strategi. **Trial P2-9 ETH momentum 4h sudah dihentikan di VPS
pada 21 Oktober 2026 20.30.39 WIB (13:30:39 UTC)**, exit 0. Tidak ada strategi
pengganti yang diaktifkan. Aturan, config, database dan bukti historis disimpan.
Keputusan ini menggantikan arahan menunggu laporan hari 1/3/7 pada handoff lama.
Review tujuh hari trial ini dibatalkan, bukan dinyatakan lulus.

Larangan menjalankan bot/trial di laptop tetap berlaku. Mainnet belum diizinkan.
Riset sesi ini berupa pembacaan bukti, pemeriksaan read-only VPS, rekonstruksi
syarat momentum, dan studi sumber primer; **tidak menjalankan backtest baru**.

## Hasil penutupan yang diverifikasi

| Item | Hasil |
|---|---|
| Profil | `paper:eth-research`, instance `momentum`, binding `momentum/eth4h` |
| Jendela sejak start trial yang dicatat | 20 Okt 16:24:45–21 Okt 13:30:39 UTC, sekitar 21 jam 6 menit; bukan uptime tanpa gangguan |
| Kas awal dan akhir | 110,79104808331486 USDT; perubahan 0 USDT / 0% |
| Holding / order / eksekusi | Kosong pada ledger; arsip order/eksekusi juga kosong |
| Status proses / run terakhir | Container exited 0; run terakhir `stopped` |
| Observasi | 256 sampel, satu masalah `metrics` pada 21 Okt 03:23:33 UTC |
| Restart yang ditemukan | Container mulai lagi 21 Okt 03:23:55 UTC, bersamaan dengan banyak proyek lain; penyebab belum dibuktikan |
| Log WARN yang tersedia sejak start trial | 3 `telegram poll failed`, 1 `persist managed shutdown`; belum diaudit akar sebabnya |
| Backup final | `backup-20261021T133042Z`; dump dan config privat tetap di VPS |
| Timer | Observe disabled/inactive; backup tetap active |
| Layanan proyek | DB, DNS, Grafana dan Prometheus tetap running |
| Proyek lain | Snapshot tepat sebelum/sesudah penghentian identik |

Baseline historis 17 Oktober sudah berbeda **sebelum** tindakan sesi ini:
20 container lain tercatat restart sekitar 03:23:55 UTC pada 21 Oktober.
Jangan menuliskan `differences: []` untuk baseline historis tersebut atau
mengatribusikan restart itu ke penghentian trial ini.

Tidak ada posisi/order yang perlu dijual atau dibatalkan. Ini penghentian proses
dengan data untuk resume dipertahankan, bukan penghapusan/arsip instance melalui
kontrol managed. Status instance tersimpan masih `running`; **itu bukan bukti
proses hidup dan bukan izin resume**. Jangan start ulang, memakai ledger ini
untuk eksperimen baru, mengubah bootstrap, atau menandai selesai lewat SQL manual.

Bukti: [penghentian](testing/p2-9-stop.json) dan
[pemeriksaan trial](testing/p2-9-review.json). Backup memvalidasi daftar
arsip dan membuat checksum; tidak ada restore drill baru pada penutupan ini.

## Mengapa tidak ada transaksi

Strategi mensyaratkan momentum 84 candle 4h (14 hari) **>5%** dan close > EMA100.
Perhitungan ulang dari candle 4h yang tersimpan memberi:

| Candle tutup (WIB) | Momentum 14 hari |
|---|---:|
| 21 Okt 03.00 | −9,44% |
| 21 Okt 07.00 | −9,20% |
| 21 Okt 11.00 | −6,39% |
| 21 Okt 15.00 | −6,12% |
| 21 Okt 19.00 | −4,25% |

Semua gagal syarat momentum, sehingga pemeriksaan EMA tidak diperlukan untuk
menjelaskan tidak adanya entry pada lima candle tersebut. Ini pemeriksaan syarat
perlu dari data tersimpan, bukan replay lengkap feed, warmup atau engine.
Riwayat sekitar 37–38 trade tutup setahun juga menunjukkan profil ini memang
jarang bertransaksi. Tidak ada entry selama 21 jam **belum membuktikan strategi
rugi atau kehilangan edge**. Trial ditarik karena keputusan user untuk kembali
riset, bukan karena bukti statistik kegagalan strategi.

## Apa yang sudah dan belum didukung riset lama

- [Baseline 20 kombinasi](testing/backtest/summary.txt): semuanya rugi
  pada tahun 2025–2026 dengan parameter yang diuji. Tidak menjadi alasan memilih
  lagi EMA cepat, grid, DCA atau MM sebagai default.
- [Kandidat pullback 4h](testing/candidates/summary.txt): BTC +2,41%
  dengan 14 penutupan dan ETH +2,35% dengan 20 penutupan pada tahun tersebut.
  Hasil tipis; keduanya bahkan tidak lebih sering tutup transaksi daripada ETH
  momentum referensi. Tidak boleh dijanjikan sebagai solusi aktivitas harian.
- [ETH momentum referensi](testing/eth-reference-fees/summary.json):
  +34,38%, +57,81%, +8,80% pada tiga simulasi tahunan independen; maksimum
  drawdown masing-masing 10,50%, 12,54%, 11,37%. Bukan modal tersambung tiga tahun
  dan ETH dipilih setelah melihat hasil lintas koin.
- [Validasi 360 pekerjaan](testing/high-return-validation-analysis.json):
  tidak ada kandidat lolos gerbang target 50% historis; `momentum-037-larger`
  harian lolos penyaringan ketahanan heuristik, bukan bukti keunggulan statistik.
  Rata-rata kelompok yang positif juga tidak menjamin tiap koin positif:
  BTC pada validation stress sekitar −0,25%.
- 2.340 pekerjaan riset dan 360 validasi bukan ribuan eksperimen independen.
  Ada pencarian banyak parameter, koin berkorelasi, periode yang sudah dilihat,
  serta perbedaan warmup dan model eksekusi antar laporan. Jangan membandingkan
  return kandidat dengan sizing 25% dan 75% sebagai bukti kualitas sinyal semata.

Target historis user sekitar 50% per tahun tetap dicatat sebagai **aspirasi yang
belum terbukti konsisten**. Jangan mengejar target dengan menaikkan leverage,
risiko, atau frekuensi tanpa bukti tambahan.

## Daftar strategi untuk dikelola

Daftar ini adalah prioritas riset, bukan keputusan alokasi modal atau deployment.

| ID | Kandidat / koin | Status sekarang | Pertanyaan berikutnya |
|---|---|---|---|
| R-01 | ETH momentum 4h, aturan 20 Oktober | Trial ditarik; simpan sebagai referensi | Apakah hasil setelah biaya dan risiko sepadan dengan periode menunggu? |
| R-02 | BTC momentum harian `momentum-037-larger` | Pembanding ketahanan | Apakah return kecil/drawdown lebih rendah cukup berguna dibanding tunai dan buy-and-hold? |
| R-03 | Pullback 4h BTC dan ETH, aturan kandidat lama | Kandidat pembanding | Apakah bertahan lintas periode, biaya stress dan sizing yang disamakan? |
| R-04 | Breakout harian, aturan kandidat lama | Cadangan riset | Ada alasan baru untuk menaikkan prioritas di luar hasil lama yang tipis? |
| R-05 | EMA cepat / MM / DCA / grid lama | Tidak diprioritaskan | Buka lagi hanya dengan hipotesis yang menjelaskan kelemahan sebelumnya |

BTC/ETH didahulukan untuk memperkecil ruang pencarian dan memakai data yang
sudah tersedia, bukan klaim keduanya memiliki tarif termurah. BNB/XRP/SOL lama
bisa dipakai sebagai pemeriksaan generalisasi; memilih pemenangnya sesudah
melihat hasil wajib dicatat sebagai seleksi tambahan. Tarif akun/pasangan
belum diverifikasi ulang pada sesi ini.

## Protokol riset berikutnya

1. **Bekukan pertanyaan dan ukuran keberhasilan sebelum menjalankan pekerjaan.**
   Laporkan pertumbuhan bersih, maksimum drawdown, turnover/fee, jumlah entry dan
   siklus tertutup, persentase waktu tunai, serta median dan jeda terlama tanpa
   entry. Preferensi user tentang aktivitas minimum dan batas drawdown untuk
   trial baru belum ditetapkan; jangan menggantinya dengan angka buatan.
2. **Daftarkan aturan persisnya.** R-01/02/03 menjadi referensi; perluasan yang
   diminta user dicatat pada bagian matriks di bawah. Tautkan config/parameter
   dan hash sebelum eksekusi. Jangan melonggarkan ambang 5% hanya agar bot
   terlihat bekerja. R-03 terdiri dari dua kasus koin,
   bukan dua alokasi otomatis. Setiap perubahan sinyal mendapat versi baru.
3. **Samakan perbandingan.** Modal Rp2 juta total per simulasi alternatif,
   tanggal/data/warmup sama, feed eksekusi 1m, tanpa leverage. Bandingkan aturan
   dengan profil risiko bersama yang dibekukan dahulu; profil lama boleh
   direproduksi terpisah. Jangan menjumlahkan akun simulasi independen sebagai
   satu portofolio. Pakai CLI `portfolio-backtest` untuk modal bersama bila
   alokasi banyak binding memang akan diuji.
4. **Jujur tentang data yang sudah dilihat.** Semua periode lama yang dipakai
   seleksi tetap data pengembangan/replikasi. Audit provenance, duplikat, gap,
   batas UTC dan warmup sebelum replay; gunakan walk-forward kronologis tanpa
   tuning pada fold evaluasi. Data lama yang baru diunduh belum otomatis holdout
   murni. Bukti prospektif memerlukan aturan beku dan data setelah pembekuan.
5. **Biaya dan pembanding lengkap.** Baseline model 0,1% per sisi + 2 bps;
   stress model 0,2% + 10 bps, dicatat sebagai asumsi, bukan tarif akun. Samakan
   perlakuan biaya beli/jual akhir, spread/slippage dan inventori terbuka.
   Bandingkan tunai 0%, buy-and-hold BTC/ETH dan profil ETH lama pada periode,
   modal, biaya dan exposure yang dijelaskan. Pajak/kurs yang belum dimodelkan
   harus tampak di laporan.
6. **Nilai stabilitas, bukan kasus return tertinggi.** Tampilkan seluruh hasil
   dan kegagalan, kontribusi trade terbesar, kondisi naik/turun/sideways dan
   sensitivitas kecil parameter. Sampel kecil diberi status belum cukup bukti;
   mengganti parameter setelah melihat fold berarti eksperimen baru.
7. **Putuskan status secara eksplisit.** `kandidat → aturan beku → dievaluasi →
   paper / ditolak / belum cukup bukti`. Kandidat paper memerlukan hasil bersih
   yang bertahan pada biaya stress, alasan keunggulan dibanding benchmark
   (return atau pengurangan drawdown), sampel memadai, dan risk/trade-off yang
   disepakati. Tidak ada kandidat otomatis lolos karena paling tinggi di daftar.
8. **Trial berikutnya terpisah.** VPS saja; DB/wallet trial baru, satu modal total,
   baseline proyek lain, backup, commit/digest/config hash dan waktu mulai.
   Bedakan kesehatan aplikasi dari performa strategi. Parameter tetap selama
   observasi; ketiadaan trade dinilai terhadap distribusi jeda historis dan
   kondisi sinyal. Tidak ada jadwal memulai trial atau promosi live otomatis.

Belum ada backtest baru dijadwalkan/dijalankan atau keputusan final pengganti
ETH. Pengujian berat tetap perlu pekerjaan tersendiri dengan tempat eksekusi
yang sesuai batas laptop/VPS; permintaan riset ini tidak mengizinkan live.

## Perluasan riset multi-instance — 21 Oktober 2026

User meminta banyak kombinasi koin top 20, strategi, modal dan timeframe,
termasuk beberapa strategi dalam satu instance, dengan batas beban VPS/API.
Ini memperluas daftar riset awal, bukan izin mainnet atau resume ledger lama.
**Status: rancangan kapasitas dan matriks; belum membuat antrean runnable,
mengimplementasikan strategi baru, menjalankan backtest atau deploy trial.**

### Model dan jumlah eksperimen

Satu koordinator wallet, banyak instance virtual, maksimal tiga binding per
instance pada rancangan paper ini. Binding adalah satu konfigurasi strategi;
bisa mengamati beberapa koin. Tidak membuat satu container, koneksi DB pool,
WebSocket atau Telegram long-poller per instance. Satu modal instance dipakai
bersama; cap binding tidak menciptakan uang baru.

Riset dipisahkan menjadi penyaringan sinyal individual, pengujian portofolio
bermodal bersama, lalu paper bertahap. Seluruh kombinasi tidak dihidupkan
bersamaan. Jumlah di bawah adalah plafon rencana, bukan pekerjaan selesai.

### Universe kandidat top 20

Snapshot [CoinGecko](https://www.coingecko.com/) dibaca 21 Oktober 2026:

| Koin | Peringkat snapshot | Pasangan kandidat |
|---|---:|---|
| BTC | 1 | BTCUSDT |
| ETH | 2 | ETHUSDT |
| BNB | 4 | BNBUSDT |
| XRP | 5 | XRPUSDT |
| SOL | 7 | SOLUSDT |
| TRX | 8 | TRXUSDT |
| ZEC | 10 | ZECUSDT |
| DOGE | 12 | DOGEUSDT |
| LINK | 16 | LINKUSDT |
| ADA | 17 | ADAUSDT |
| NEAR | 19 | NEARUSDT |

Ini kandidat, **belum verifikasi listing/izin akun/likuiditas Binance**. Kandidat
masuk hanya jika Spot USDT berstatus TRADING, filter akun/instrumen terpenuhi,
data cukup dan kualitas spread/volume memadai. Stablecoin dan aset tokenisasi
dikecualikan; anggota top 20 lainnya belum dimasukkan karena ketersediaan/data
belum diverifikasi. Catat alasan setiap pengecualian, jangan mengganti dengan
koin di luar top 20 diam-diam. Bekukan snapshot saat job dibuat; perubahan
peringkat tidak otomatis mengubah eksperimen yang sedang berjalan.

Universe sekarang pada data lampau tetap memiliki bias seleksi/survivorship.
Penyaringan eksploratif ini bukan studi historis top 20 pada setiap tanggal.

### Matriks penyaringan

| Dimensi | Pilihan |
|---|---|
| Koin | Maksimum 11 kandidat di atas, setelah pemeriksaan kelayakan |
| Enam strategi satu koin | Momentum, pullback tren, channel breakout, EMA lambat sebagai kontrol, squeeze breakout, mean reversion khusus sideways |
| Timeframe sinyal | 1h, 4h, 12h, 1d; eksekusi historis 1m |
| Modal virtual per alternatif | Rp500 ribu, Rp2 juta, Rp5 juta, Rp10 juta |
| Strategi ketujuh | Rotasi momentum lintas universe + pilihan tunai; evaluasi harian/mingguan |
| Periode replikasi | Tiga jendela tahunan 19 Okt 2023–2024, 2024–2025, 2025–2026 |
| Biaya | Baseline 0,1%/sisi + 2 bps; stress 0,2%/sisi + 10 bps |

Enam strategi × 11 koin × empat timeframe × empat modal = **1.056 konfigurasi**.
Rotasi: dua jadwal × empat modal = **8 konfigurasi**, memakai satu universe
bersama, bukan dikali 11 koin lagi. Total **1.064 konfigurasi individual/rotasi**
× tiga periode × dua biaya = **6.384 pekerjaan backtest**.

Portofolio multi-strategi di bawah menambah 24 konfigurasi × tiga periode × dua
biaya = **144 pekerjaan**, sehingga plafon total **6.528 pekerjaan**. Jumlah
berkurang bila koin/data tidak layak. Strategi belum siap tidak diganti dengan
strategi lama bernama baru dan tidak dihitung selesai. Setiap job gagal/skipped
tetap ada dalam laporan beserta alasannya.

Momentum/pullback/channel breakout tersedia sebagai mode `trendspot`; EMA
tersedia sebagai `emacross`. Rotasi, squeeze dan sideways mean reversion
**belum tersedia** dan memerlukan aturan persis, implementasi serta regression
test sebelum matriks ini runnable. Parameter per timeframe harus ditulis
eksplisit (durasi lookback bisa berbeda); perubahan timeframe bukan jaminan
sinyal yang sama. Tidak membuka pencarian ambang/risk tambahan di batch ini.

Ketiga tahun sudah terlihat dalam riset lama: hasilnya replikasi/eksplorasi,
bukan holdout murni. Bekukan sinyal, periode, risiko, benchmark dan kriteria
seleksi sebelum job pertama; evaluasi prospektif memakai data setelah pembekuan.
Catat seluruh hasil, bukan hanya pemenang dari ribuan percobaan. Empat modal
bukan empat bukti sinyal independen; tujuannya mengukur efek minimum order,
pembulatan dan keterbatasan modal. Gunakan risk bersama yang sama persentasenya,
tanpa leverage; jangan membuat modal besar terlihat bagus dengan risk berbeda.

### Portofolio dan target paper

Enam susunan awal berikut × empat modal = **24 instance**, masing-masing tiga
binding = **72 binding maksimum**. Susunan ini usulan yang harus dibekukan dan
diuji sebagai portofolio; kelulusan sinyal individual tidak otomatis meloloskan
portofolio. Tidak mengganti koin berdasarkan pemenang tahun evaluasi.

| Susunan | Binding 1 | Binding 2 | Binding 3 |
|---|---|---|---|
| Tren | Momentum BTC 4h | Momentum ETH 4h | Pullback SOL 4h |
| Breakout | Squeeze BTC 4h | Squeeze ETH 4h | Channel breakout BNB 4h |
| Sideways | Mean reversion BTC 4h | Mean reversion ETH 4h | Mean reversion XRP 4h |
| Pullback | Pullback BTC 4h | Pullback ETH 4h | Pullback LINK 4h |
| Rotasi dan tren | Rotasi universe mingguan, data 1d | Momentum BTC 1d | Pullback ETH 4h |
| Campuran | Momentum BTC 1d | Squeeze ETH 4h | Mean reversion DOGE 1h |

Semua instance paper memakai satu sumber dana virtual eksplisit. Jika seluruh
24 aktif, jumlah alokasi = 6 × (Rp0,5 + Rp2 + Rp5 + Rp10 juta) = **Rp105 juta
virtual**, tanpa cadangan tambahan. Itu modal laboratorium, bukan modal riil
user dan bukan simulasi satu akun Rp2 juta. Laporan tiap alternatif terpisah;
return 24 instance tidak dijumlahkan untuk mengklaim hasil modal Rp2 juta.
Skenario akun riil Rp2 juta kelak perlu seleksi dan backtest alokasi sendiri.

Pool instance berbagi cash, dengan urutan keputusan deterministik dan batas
posisi/exposure instance. Perbandingan komposisi harus mencatat order yang
tidak terdanai. Pada modal Rp500 ribu jangan memaksa tiga posisi serentak atau
menaikkan size agar lolos minimum. Contoh kurs historis Rp18.052/USDT memberi
27,70 USDT; cap posisi 25% hanya 6,92 USDT sebelum fee/pembulatan. Filter
notional aktual dapat membuat order tidak layak; catat sebagai keterbatasan,
bukan sinyal rugi atau alasan mengakali filter exchange.

### Kapasitas yang benar-benar diperiksa

Inspeksi SSH read-only pada **21 Oktober 2026 sekitar 21.53 WIB**:

| Item | Hasil snapshot |
|---|---|
| CPU terlihat oleh proses | 8 logical CPU |
| Load average 1/5/15 menit | 2,55 / 2,31 / 1,78 |
| RAM total / available | 31.833 / 20.060 MiB (sekitar 31,1 / 19,6 GiB) |
| Swap | Tidak ada |
| Disk root | 399 GiB total, 137 GiB available, 66% terpakai menurut `df -h` |
| DB bot | 127,4 MiB / limit 1 GiB |
| Grafana | 428,7 MiB / limit 512 MiB |
| Prometheus / DNS bot | 52,21 / 17,79 MiB |

Snapshot tidak membuktikan kapasitas pada beban puncak. Banyak proyek lain
berbagi host; tidak ada layanan yang diubah atau dimulai pada inspeksi ini.

`Runtime.Record` dan `Tick` masih melintasi seluruh binding. `syncView`,
`checkpoint`, `PGStore.Load/Update` membaca/mengurai agregat state; update yang
tidak mengubah state pun melakukan transaksi/pemeriksaan. Jadi feed bersama
mengurangi API, tetapi tidak membuat biaya CPU/DB per binding menjadi nol.
Kompaksi dan throttle mark 10 detik bukan bukti runtime mampu 72 binding.

### Batas pekerjaan dan kenaikan bertahap

**Antrean offline VPS:** satu job aktif, satu logical CPU, RAM maksimum 2 GiB,
nice 10/IO priority rendah, maksimal dua unduhan arsip bersamaan, cache data
bersama. Batas CPU hanya 1/8 kapasitas logical host, bukan jaminan beban layanan
lain tidak berubah. Berhenti menjadwalkan job baru pada disk free <50 GiB,
RAM available <8 GiB, load 5 menit >6 selama lima menit, atau gangguan layanan
yang bertepatan dengan riset. Tidak menjalankan worker riset di laptop.

Anggaran data/output awal **20 GiB termasuk ekstraksi sementara dan cadangan
hasil**; hitung ukuran arsip/rencana ekstraksi sebelum mengunduh. Jangan
menyimpan salinan dataset per modal/strategi. Timeout awal per job 15 menit,
tanpa retry tanpa batas; catat kegagalan, checkpoint dan kemampuan resume.
Worker antrean lama masih khusus lokal, lima koin dan jumlah job tetap: **perlu
adaptasi VPS/matriks baru**, bukan langsung menjalankan worker arsip.

Tidak ada ETA terukur untuk batch ini. Ilustrasi 6.528 × 30 detik = 54,4 jam
CPU job serial, × 120 detik = 217,6 jam (sekitar 9,1 hari), belum termasuk
unduhan/persiapan. Ini skenario, bukan benchmark. Ukur batch kalibrasi VPS
sebelum menetapkan ETA; jangan memakai kecepatan backtest lama dengan resolusi
dan batas CPU berbeda. Cache 1m tiga tahun untuk 11 koin sekitar 17,4 juta bar,
tetapi replay seluruh matriks membaca ulang banyak bar; ukuran RAM tidak boleh
ditaksir dari ukuran CSV saja.

**Paper VPS:** mulai **3 instance / 9 binding / maksimal 3 koin**, lalu naik
ke 6/18, 12/36, dan paling banyak 24/72. Tahan tiap tahap minimal 24 jam operasi
normal termasuk satu restart terkontrol sebelum menambah; ini kelulusan operasi,
bukan validasi profit. Koordinator bot tetap satu, bukan 24 container. Batas
awal bot 0,5 CPU/512 MiB dan DB 1 CPU/1 GiB mengikuti manifest sekarang; jangan
menaikkan limit atau menambah DB/monitoring hanya agar target jumlah terpenuhi.
Awali tanpa backtest paralel; jalankan bersamaan hanya setelah headroom diukur.

Gerbang sebelum naik: tidak ada OOM/restart tak terencana, feed gap, backlog
bertumbuh atau attention keuangan; CPU bot rata-rata <70% kuotanya, RAM <70%
limit dan proyeksi pertumbuhan DB/WAL/backup dalam anggaran disk. Ukur p95/p99
durasi event dan lag, termasuk ledakan saat candle tutup bersamaan. Angka
persentase ini kebijakan konservatif yang diusulkan, belum guard otomatis.
Jika gagal, tetap di tahap terakhir yang terbukti sehat. Hentikan worker
offline dahulu; sekadar pause entry paper tidak mengurangi jalur fan-out/DB.
Jika perlu stop paper, simpan state/order simulasi untuk recovery dan catat
akhir tahap, jangan kill/restart loop atau menghapus hasil.

### Perhitungan feed dan batas Binance

Dengan maksimal 11 koin dan lima series unik (1m eksekusi + 1h/4h/12h/1d),
batas atas **55 stream**, satu combined connection pada implementasi yang
memecah tiap 200 stream. Pada update kline sekitar dua detik, estimasi rata-rata
27,5 event/detik; burst candle tutup bisa lebih tinggi. Untuk 72 binding,
fan-out mentah sekitar **1.980 kunjungan binding/detik**, ditambah Tick tiap
detik dan akses DB/checkpoint. Angka ini hitungan jalur, bukan ukuran throughput.
Rata-rata service event harus lebih cepat dari sekitar 36 ms agar antrean tidak
terus tumbuh pada 27,5 event/detik; target headroom sekitar separuhnya masih
harus dibuktikan. Paper portofolio yang hanya memakai 1h/4h/1d membutuhkan
lebih sedikit series; subscription aktual harus dihitung dari config.

[Dokumentasi WebSocket Binance](https://github.com/binance/binance-spot-api-docs/blob/master/web-socket-streams.md)
menyebut 1.024 stream/koneksi, 300 percobaan koneksi per lima menit per IP,
dan lima pesan masuk ke server per detik (ping/pong/control, **bukan** batas
lima update harga yang diterima). Koneksi memiliki masa hidup 24 jam.
Batas internal usulan: maksimal dua control frame/detik termasuk cadangan pong,
dan enam upaya reconnect per lima menit total dengan exponential backoff/jitter;
sesudah itu cooldown, bukan loop reconnect. Angka internal ini belum guard
yang sudah diverifikasi di kode.

Paper memakai data publik tanpa trading key; order simulasi **nol request order
exchange**. Backtest memakai arsip resmi/cache, bukan REST per kombinasi.
Warmup 55 series × satu request ≤1.000 bar × weight 2 = **110 weight** bila
satu halaman cukup. Ini warmup terbatas, bukan unduh tiga tahun candle 1m;
request tambahan untuk gap/backfill/instrumen tetap dihitung.

Anggaran REST riset usulan: maksimum **min(300 weight/menit, 10% limit aktual)**
dan maksimal satu request/detik, dengan pemantauan pemakaian seluruh IP.
Ambil limit dari `exchangeInfo`, amati `X-MBX-USED-WEIGHT-*`, sisakan ruang
untuk klien lain dan jangan menganggap API key berbeda memberi kuota IP baru.
Limiter Spot kode sekarang 4.800 weight/menit per client, bukan limiter global
host dan **belum** menerapkan anggaran konservatif ini. Verifikasi limiter,
cooldown, reconnect dan retry sebelum download/feed diperluas.

[Aturan REST Binance](https://github.com/binance/binance-spot-api-docs/blob/master/rest-api.md#limits)
mewajibkan backoff pada 429; patuhi `Retry-After` dan hentikan permintaan
selama jendela itu. Pada 418 hentikan trafik riset Binance sampai ban berakhir
dan penyebab ditinjau; jangan merotasi key/IP untuk melewati limit. 403/WAF juga
tidak boleh di-retry rapat. Risiko ban IP tetap ada walau paper, dan kepatuhan
rate limit bukan jaminan akun tidak pernah dibatasi karena sebab lain.
Tidak ada rencana testnet massal atau transaksi live dari matriks ini.

### Urutan pekerjaan berikutnya

1. Audit restart/WARN trial lama dan cek ulang kapasitas serta listing/data.
2. Bekukan spesifikasi sinyal/risk/komposisi, manifest job dan kriteria seleksi;
   implementasikan tiga strategi yang belum tersedia beserta regression test.
3. Adaptasi antrean offline/resource guard dan limiter bersama; review semua
   batas di atas. Catat mana hard limit, metrik terukur dan asumsi.
4. Rilis lewat CI/digest seperti biasa; kalibrasi batch kecil di VPS dan perbarui
   ETA/ukuran data sebelum meneruskan antrean. Tidak menyalin source laptop ke VPS.
5. Selesaikan backtest individual dan bermodal bersama; hanya kandidat yang
   lolos biaya stress/benchmark dengan bukti cukup masuk tahap paper baru.
6. Trial baru memakai DB/wallet/config hash sendiri, tahap 3→6→12→24 instance.
   Kurangi target bila beban tidak lolos; tidak ada kewajiban memenuhi 24.

Matriks backtest, hard guard baru dan antrean belum dijalankan. Sampel operasional
3/9 dari trial baru tercatat di handover VPS; tidak membuktikan throughput
backtest atau keamanan target 24 instance.

## Sumber primer yang dibaca ulang

- [Moskowitz, Ooi & Pedersen, Time Series Momentum (2012)](https://www.aqr.com/Insights/Research/Journal-Article/Time-Series-Momentum):
  bukti momentum lintas kontrak futures/forward; dasar hipotesis, bukan validasi
  konfigurasi Spot crypto atau lookback 14 hari kita.
- [Rozario dkk., A Decade of Evidence of Trend Following Investing in Cryptocurrencies (2020)](https://arxiv.org/abs/2009.12155):
  penelitian trend following crypto; konteks historisnya berbeda. Angka return
  paper tidak dipakai sebagai target atau proyeksi bot ini.
- [Bailey dkk., The Probability of Backtest Overfitting (2015)](https://www.davidhbailey.com/dhbpapers/backtest-prob.pdf):
  banyak percobaan pada data yang sama meningkatkan risiko temuan palsu;
  memotivasi pencatatan semua percobaan dan pemisahan evaluasi dari seleksi.
  Kita belum menghitung PBO/CSCV pada hasil proyek ini.

## Tindak lanjut terbuka

- [x] Tarik proses trial, simpan data dan backup, hentikan observasi trial.
- [x] Catat saldo, order/eksekusi/arsip, gangguan observasi dan syarat momentum.
- [x] Tetapkan daftar kandidat dan protokol evaluasi tanpa mengaktifkan pengganti.
- [x] Audit restart bersama pukul 10.23 WIB dan WARN shutdown: journal menunjukkan
  service Docker dihentikan dan DB memutus koneksi; pemicu administratif belum ditelusuri.
- [ ] Sepakati trade-off aktivitas, return dan drawdown; bekukan spesifikasi
  perbandingan, tempat eksekusi serta periode data sebelum backtest berikutnya.
- [ ] Jalankan evaluasi terkontrol, lalu perbarui daftar di atas dengan hasilnya.
