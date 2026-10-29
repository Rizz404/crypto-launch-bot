# Review tujuh hari pembanding P2

Snapshot baca-saja VPS **29 Oktober 2026 20:34:29 WIB**, sekitar 36 menit
sesudah jadwal review tujuh hari. T0 tetap **22 Oktober 19:58:45 WIB**;
angka berikut bukan rekonstruksi tepat pada batas tujuh hari.
[Bukti numerik dan 214 fill](research-stage-two-review.json).

## Hasil

Modal **747,83957456 USDT** menjadi **740,40558347 USDT**: rugi
**7,43399109 USDT (0,9941%)**. Seluruh holding bernilai kuantitas nol,
pending event dan order nonterminal nol; akun tidak halted/attention.
Enam instance/18 binding masih running. Tidak ada perubahan konfigurasi,
parameter, modal, database, T0, image atau proses VPS pada review ini.

| Instance | Modal USDT | PnL net USDT | Return | Komisi USDT | Fill beli/jual | Fill jual untung/rugi |
|---|---:|---:|---:|---:|---:|---:|
| Tren Rp500 ribu | 27,697762 | 0 | 0% | 0 | 0/0 | 0/0 |
| Tren Rp2 juta | 110,791048 | 0 | 0% | 0 | 0/0 | 0/0 |
| Tren Rp5 juta | 276,977620 | 0 | 0% | 0 | 0/0 | 0/0 |
| EMA dasar 1m | 110,791048 | −4,242513 | −3,8293% | 3,390376 | 63/63 | 6/57 |
| EMA tren 1m | 110,791048 | −3,191478 | −2,8806% | 2,383502 | 44/44 | 3/41 |
| EMA tren 4h | 110,791048 | 0 | 0% | 0 | 0/0 | 0/0 |

Outcome dihitung per fill jual, bukan menganggap setiap partial fill sebagai
trade independen. Realized binding sudah mencakup biaya entry/exit: komisi
**5,77387876 USDT tidak dikurangkan lagi**. Biaya setara **77,67%** dari rugi
net akun. Jika komisi tercatat ditambahkan kembali, hasil tetap negatif
**−1,66011233 USDT** (baseline −0,852137; filter tren −0,807976).
Ini atribusi aritmetika; slippage dan waktu eksekusi tetap ada.
Diskon biaya saja belum membuktikan edge positif.

Filter tren mengurangi jumlah fill dan rugi nominal, tetapi kedua EMA 1m
masih negatif. Empat instance tanpa transaksi belum bisa dinilai unggul atau
gagal; return nol berasal dari tidak ada exposure, bukan bukti profitabilitas.
Khusus tiga ukuran modal Tren, belum ada bukti pengaruh ukuran terhadap hasil.

## Operasi belum lulus

- Bot P2 masih source `9c18962`, start 23 Oktober 20:52 WIB, restart 0 dan
  OOM false. Proses aktif tidak membuktikan data pasar lancar.
- Candle 1m BTC/ETH/SOL terakhir open **20:27 WIB**, umur **450 detik** pada
  snapshot. Karena candle ditulis sesudah penutupan, ukuran ini mencakup
  durasi candle; tetap jauh di atas toleransi observer 240 detik.
- Timestamp harga ledger ketiganya **20:03 WIB**, sekitar 31,5 menit sebelum
  snapshot. Ledger mark lebih tertinggal daripada candle tersimpan. Karena
  holding nol, equity snapshot ini berasal dari kas, bukan harga stale tersebut.
- Log 24 jam terakhir memuat **140 baris cocok `Pong timeout`** dalam 941
  baris yang dibaca (batas 20.000). Ini hitungan baris, bukan 140 outage unik.
- Sebanyak **1.186 dari 1.894** sampel observer mencatat
  `candle_freshness_or_gap`. Rentang 23 Oktober 07:01 WIB–29 Oktober
  20:32 WIB; dimulai sebelum rilis notifikasi malam 23 Oktober, bukan cakupan
  penuh tujuh hari atau persentase downtime. Sampel terakhir masih bermasalah.
- HTTP P2 `/healthz` tetap **200 `ok`** saat pemeriksaan terpisah sesudah
  snapshot. Health proses tersebut tidak cukup untuk menilai kelayakan feed.

Gangguan feed membatasi interpretasi performa strategi, terutama pembanding
1m. Gejala konsisten dengan catatan 28 Oktober; penyebab websocket/backpressure
belum dibuktikan. Audit alur baca/control frame, callback sinkron dan antrian
consumer sebelum memilih patch. Jangan menyimpulkan penyebab hanya dari log.

## Keputusan dan pekerjaan lanjut

1. **Review tujuh hari sudah dilakukan; penerimaan operasi belum lulus.**
   P3-O3 tetap terbuka untuk durasi/sampel lanjutan dan gangguan feed.
2. Usulkan pause entry dua EMA 1m setelah persetujuan user; status saat
   snapshot flat, tetapi harus diperiksa lagi saat aksi benar-benar dilakukan.
   Review ini belum menonaktifkan instance atau menghentikan proses.
3. Prioritaskan diagnosis/perbaikan feed dan health yang mencerminkan freshness.
   Perubahan source, CI dan rollout P2 merupakan pekerjaan lanjutan terpisah;
   jangan merestart untuk menutupi gejala atau mengubah T0/modal/history.
4. Sesudah feed stabil, catat batas periode baru untuk perbandingan prospektif.
   Pertahankan ledger/history lama; jangan mencampur hasil parameter baru dengan
   hasil ini. Periode tanpa trade perlu diperpanjang sesuai protokol Strategi 2.
5. P3 tetap terpisah. Inspeksi kali ini hanya membuktikan container source
   `1779878` running/restart 0/OOM false dan health runtime/feed 200; belum
   menilai riset, kuota AI, posisi atau hasil hari pertamanya. Review P3 hari 1
   dijadwalkan **29 Oktober 22:53 WIB**, sesudah snapshot ini.

Pekerjaan P3 yang masih terbuka: backup/restore DB dan arsip, observer/alert,
QA browser serta review hari 1/3/7. Mulai dari operasi pilot yang sudah aktif;
jangan mengulang bootstrap atau memakai instruksi pause/WIP lama sebagai status kini.

## Metode dan batas pemeriksaan

Wallet, arsip execution dan candle dibaca dalam satu transaksi PostgreSQL
`REPEATABLE READ READ ONLY`, database `tradingbot_research_stage_two`, wallet
`paper:research-stage-two`. Docker inspect/logs, health dan file observer
dibaca terpisah sesudahnya. Angka PnL/komisi direkonsiliasi dengan Python
Decimal: kas minus funded = jumlah realized binding = jumlah delta realized
214 fill, dan komisi binding = total fee fill. Arsip execution kosong.

Tidak menjalankan suite tes, build, bot lokal, inference baru, deployment,
restart, simulasi outage, restore backup atau uji penerimaan Telegram/browser.
Tidak ada izin uang asli dari hasil review ini.
