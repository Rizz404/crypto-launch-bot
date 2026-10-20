# Keputusan strategi berikutnya — 20 Oktober 2026

## Hubungan dengan rencana aktif

Pengembangan aplikasi dan pekerjaan tersisa mengikuti [Plan 2](plan-2.md).
[Plan 1](plan.md) sudah selesai/arsip. Fitur pembagian modal antar-instance di
[rancangan baru](multi-instance.md) tidak mengubah modal trial ETH menjadi
Rp2 juta per instance. ETH tetap kandidat utama, BTC pembanding; tidak ada
kewajiban menjalankan keduanya atau membagi dana di antara keduanya.

## Keputusan dan status

Berdasarkan analisis backtest yang sudah tersedia dan penutupan dini testnet,
**kandidat utama untuk uji paper berikutnya adalah momentum mengikuti tren
ETHUSDT pada timeframe 4 jam**. Fokus satu koin dahulu dengan modal virtual
referensi **Rp2 juta total**, bukan Rp2 juta untuk setiap strategi yang berjalan
bersamaan. **Momentum harian BTCUSDT menjadi kandidat pembanding**, bukan
alokasi kedua yang otomatis dijalankan.

Ini mencatat rekomendasi yang dibahas dengan user. Permintaan pada sesi ini
adalah mendokumentasikan strategi; **belum memulai trial, deploy, atau live**.
Tidak ada tanggal mulai/jadwal otomatis. Bot testnet lama sudah berhenti pada
20 Oktober 2026 pukul 18.52 WIB. Larangan menjalankan trial di laptop tetap
berlaku; uji prospektif berikutnya direncanakan di VPS.

## Mengapa memilih ini

- Baseline EMA, grid, DCA, dan market making pada lima koin selama
  19 Oktober 2025–19 Oktober 2026 menghasilkan 20 kasus yang semuanya rugi.
  Pengujian EMA multi-tahun dan filter tren juga belum memberi edge konsisten.
- Pengujian kandidat awal memberi keuntungan kecil pada beberapa pasangan;
  pullback 4h rata-rata +1,05% untuk lima pengujian independen, belum memenuhi
  ekspektasi user. Itu bukan return portofolio bersama.
- Riset lanjutan 2.340 pekerjaan dan validasi eksekusi 1m sebanyak 360 pekerjaan
  menunjukkan ETH momentum 4h sebagai kandidat pertumbuhan yang menarik.
  Rekomendasi ini memakai profil `momentum-028-larger`, **bukan** profil
  `momentum-028-aggressive` dengan posisi maksimum 95% dan risiko 5%.
- Grid SOL menghasilkan laba testnet terbesar, tetapi riwayat backtest belum
  mendukung konsistensinya. Testnet 43 jam tanpa komisi tidak cukup untuk
  mengubahnya menjadi pilihan utama.
- EMA cepat ETH sudah rugi sebelum komisi; MM DOGE bermargin sangat tipis;
  DCA XRP belum menutup satu siklus. Ketiganya tidak diprioritaskan saat ini.

## Aturan ETH yang dibekukan

Implementasi: `trendspot`, mode `momentum`, pasangan **ETHUSDT**, Binance Spot,
long saja dan tanpa leverage. Identitas template: `eth-momentum028-larger-paper`.
Konfigurasi sumber: [configs/paper.eth.yaml](../configs/paper.eth.yaml).

| Komponen | Aturan |
|---|---|
| Candle sinyal | 4h yang sudah tutup; feed 1m untuk mengikuti harga/eksekusi |
| Momentum | `close_sekarang / close_84_bar_lalu - 1` (84 × 4h = 14 hari) |
| Entry | Momentum **>5%** dan close **> EMA100**; indikator harus siap |
| Exit sinyal | Momentum **<=0** atau close **< EMA100** |
| Stop awal | Close sinyal dikurangi **3 × ATR14** |
| Trailing | Persentase awal **3 × ATR14 / close sinyal entry**, mengikuti mekanisme engine |
| Target tetap | Tidak ada (`target_atr: 0`) |
| Posisi | Maksimum satu; tidak menambah posisi yang sudah terbuka |
| Pemanasan | Minimal 300 candle 4h sebelum sesi, tanpa order historis |

Trailing implementasi saat ini memakai persentase yang ditetapkan pada entry,
bukan menghitung ulang jarak 3 ATR terbaru pada setiap bar.
`exit_period: 20` dipertahankan dalam template, tetapi pada mode momentum exit
menggunakan momentum dan EMA, bukan breakout low 20 bar.

## Modal dan profil risiko paper

- Rp2.000.000 pada kurs referensi tetap Rp18.052/USDT =
  **110,79104808331486 USDT**. Kurs ini asumsi pembanding historis; konversi pada
  trial baru harus dicatat dan tidak menjanjikan nilai rupiah tetap.
- Risiko sizing terencana **3% ekuitas per transaksi**; posisi maksimum **75%**.
  Ukuran aktual juga dibatasi jarak stop, saldo dan aturan lot/minimum order.
- Maksimum leverage 1 dan satu posisi; `max_exposure_pct: 1`.
- Kill harian **8%**, drawdown **30%**, `close_on_kill: true`.
- Angka tersebut adalah profil simulasi/paper yang diuji, **belum batas live yang
  disetujui user**. Risiko sizing bukan jaminan rugi maksimum saat gap/slippage.

## Bukti hasil dan batas interpretasi

[Uji konfigurasi ETH referensi](testing/eth-reference-fees-2026-10-19/summary.json)
memakai modal Rp2 juta per tahun secara independen, komisi standar 0,1% per sisi
serta slippage dasar 2 bps:

| Periode | Return standar | Drawdown maksimum | Trade tutup |
|---|---:|---:|---:|
| 19 Okt 2023–19 Okt 2024 | +34,38% | 10,50% | 38 |
| 19 Okt 2024–19 Okt 2025 | +57,81% | 12,54% | 37 |
| 19 Okt 2025–19 Okt 2026 | +8,80% | 11,37% | 38 |

Hasil ini bukan pertumbuhan modal tersambung tiga tahun. ETH dipilih setelah
melihat hasil lintas koin/periode; data tersebut **bukan holdout murni**.
Kandidat 4h gagal gerbang ketahanan lintas lima koin. Memilihnya berarti
memprioritaskan potensi pertumbuhan untuk riset, bukan menyatakan kandidat ini
lebih kokoh dari semua alternatif. Target user sekitar **50% per tahun tetap
belum terbukti dapat dicapai konsisten**; jangan menurunkannya diam-diam.

Pembanding `momentum-037-larger` menggunakan momentum **14 hari >0**, close di
atas **EMA50 harian**, stop/trailing 3 ATR. Kandidat ini lolos penyaringan
heuristik lintas koin. BTC menghasilkan +3,02%, +1,36%, +7,06% pada tiga periode
tahunan yang sama. Hasil lebih kecil itu menjadikannya pembanding ketahanan,
bukan bukti target user sudah terpenuhi. Memilih BTC setelah melihat hasilnya
juga memiliki bias seleksi. Jangan membagi modal atau menjalankan keduanya
bersamaan tanpa merancang dan menguji portofolio bersama.

## Biaya: cara menilai koin

Trading Spot internal Binance tidak membayar gas Ethereum pada setiap order.
Jadi memilih SOL/XRP karena biaya transfer blockchain murah tidak otomatis
menurunkan komisi trading bot. Nilai pasangan dari hasil bersih setelah
komisi, spread, slippage, frekuensi transaksi dan batas minimum order.

Pada pemeriksaan 20 Oktober 2026, [tabel tarif Binance](https://www.binance.com/en/fee/trading)
menampilkan tarif regular standar 0,1% maker/taker dan 0,075% dengan pembayaran
BNB sesuai kelayakan. Ini referensi bertanggal, bukan tarif akun yang telah
dikonfirmasi. Sebelum trial/live, periksa komisi akun dan simbol aktual;
model BNB harus memperhitungkan saldo, pembelian dan perubahan nilainya.
Tidak perlu memperdagangkan BNB sebagai strategi agar bisa memakai pembayaran
komisi BNB. Menggunakan limit order juga tidak otomatis memberi fee lebih rendah
pada tarif regular standar yang sama untuk maker dan taker.

Baseline tetap **0,1% per sisi + slippage 2 bps**, dengan stress biaya dan
sensitivitas 0,075% sebagai pembanding. Diskon tidak boleh menjadi syarat agar
strategi tampak menguntungkan. Sensitivitas ETH referensi tahun terakhir hanya
naik dari +8,80% menjadi +9,97%; penghematan biaya tidak mengubahnya menjadi 50%.
Pajak, transfer dan perubahan kurs belum dimodelkan dalam hasil historis itu.

## Urutan sebelum menjalankan uji berikutnya

1. Audit dua kegagalan order pelindung ETH, lifecycle OCO/fill terlambat dan
   rekonsiliasi lima DOGE dari [penutupan testnet](testing/testnet-stop-2026-10-20/analysis.md).
   Pastikan pencatatan saldo/PnL dan prosedur cancel order akhir eksperimen benar.
2. Siapkan rilis GitHub/CI dan image dengan digest tetap; source lokal sudah
   di-commit hingga `9c05d7b` sebelum dokumen ini, belum di-push pada checkpoint itu.
   Periksa ulang status rilis; jangan menganggap source tersebut sudah di VPS.
3. Ikuti [runbook ETH VPS](eth-paper-vps-reference.md): database trial baru,
   modal baru, isolasi resource/secret, baseline proyek lain, paper dan
   `allow_live: false`. Jangan melanjutkan ledger testnet/lokal lama sebagai trial baru.
4. Catat waktu mulai aktual, commit/digest, hash config, biaya, saldo awal dan
   indikator siap. Bekukan aturan; perubahan berikutnya mendapat versi trial baru.
5. Review operasi pertama setelah tujuh hari aktual: feed, proteksi, restart,
   persistence, saldo dan fee. Frekuensi historis sekitar 37–38 trade setahun;
   satu minggu sepi bukan bukti gagal dan satu minggu untung bukan bukti edge.
6. Nilai strategi dari jumlah transaksi dan beragam kondisi pasar yang memadai,
   bandingkan dengan tunai, buy-and-hold, BTC harian serta skenario biaya stress.
   Belum ada tanggal kelulusan atau ambang promosi live yang disepakati.

## Rujukan sesi mendatang

Baca dokumen keputusan ini dahulu untuk pilihan strategi; gunakan
[Plan 2](plan-2.md) untuk progres dan [handoff VPS](handoff-vps.md) untuk status server.
Bukti tambahan: [baseline](testing/backtest-2026-10-19/summary.txt),
[kandidat awal](testing/candidates-2026-10-19/summary.txt),
[riset lanjutan](testing/research-analysis-2026-10-19.json),
[validasi dan batas interpretasi](testing/high-return-validation-analysis-2026-10-19.json),
[referensi ETH terstruktur](testing/eth-paper-reference-2026-10-19.json).
Jika keputusan berubah, perbarui status dan alasan dengan tanggal; jangan
menimpa bukti historis atau menganggap rekomendasi ini sebagai deployment aktif.
