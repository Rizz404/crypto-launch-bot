# Penarikan trial dan pengelolaan riset strategi — 21 Oktober 2026

## Keputusan terbaru

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
2. **Daftarkan aturan persisnya.** Mulai dengan R-01/02/03 yang sudah ada; tautkan
   config/parameter dan hash. Jangan grid search baru atau melonggarkan ambang
   5% hanya agar bot terlihat bekerja. R-03 terdiri dari dua kasus koin,
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
- [ ] Audit restart bersama pukul 10.23 WIB dan WARN shutdown sebelum trial baru.
- [ ] Sepakati trade-off aktivitas, return dan drawdown; bekukan spesifikasi
  perbandingan, tempat eksekusi serta periode data sebelum backtest berikutnya.
- [ ] Jalankan evaluasi terkontrol, lalu perbarui daftar di atas dengan hasilnya.
