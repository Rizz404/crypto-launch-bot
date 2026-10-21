# Penutupan dini uji Spot Testnet — 20 Oktober 2026

User meminta menghentikan eksperimen karena hasil di bawah ekspektasi, lalu
menganalisisnya sebelum mencari strategi lain dan mengembangkan aplikasi.
Rencana akhir 21 Oktober 23.43 WIB dibatalkan. Jendela aktual: **18 Oktober
23.43.10 WIB–20 Oktober 18.52.43 WIB**, sekitar **43 jam 9 menit** dari target 72 jam.
Ini penutupan atas permintaan user, bukan kelulusan observasi tiga hari.

## Penghentian dan penyimpanan

- Compose project `trading-bot-paper`, bot `trading-bot-paper-bot-1` dihentikan
  dengan grace period 30 detik; exit **0**, run 9 tertutup normal `stopped`.
- Tujuh order yang kepemilikannya cocok dengan client ID dalam DB dibatalkan
  langsung di **Binance Spot Testnet**: enam SOL dan satu DOGE. Query ulang
  exchange: **0 order milik bot dan 0 order lain** tersisa.
- Inventori testnet dipertahankan, bukan dijual paksa. Di ledger run terakhir:
  SOL 0,468; XRP 368,1; DOGE 83. Saldo bawaan akun testnet tidak dilikuidasi.
- Pembatalan terjadi sesudah recorder bot berhenti. DB historis masih menampilkan
  tujuh order berstatus `new`; bukti exchange di `exchange_cleanup` adalah status
  final pembatalannya. Rekonsiliasi wajib sebelum resume; jangan menganggap order
  historis itu masih aktif. Posisi run 7/8 adalah snapshot sebelum restart, bukan
  inventori tambahan; analisis posisi aktif memakai run 9 saja.
- Timer observasi di-disable dan inactive. Timer backup tetap aktif; DB, DNS,
  Grafana, Prometheus tetap berjalan untuk penyimpanan dan pemeriksaan.
  Bot memakai `unless-stopped` dan tetap mati setelah stop eksplisit ini.
- Backup final: `/opt/trading-bot-paper/backups/backup-20261020T115448Z`,
  validasi daftar arsip/checksum berhasil. Dump dan konfigurasi rahasia tetap di
  VPS. Ini bukan uji restore baru.
- Pemeriksaan baseline sebelum/sesudah: 28 container dan lima service proyek
  lain, `differences: []`. Tidak ada redeploy, penggantian image atau mainnet.

## Hasil strategi

Angka berikut adalah **trade tutup yang tercatat**, bukan seluruh perubahan
nilai akun dan bukan return tahunan. Komisi aktual testnet tercatat nol.
Trade dapat berupa partial close; jumlahnya tidak sama dengan siklus posisi.

| Strategi | Trade tutup | Menang | PnL tercatat (USDT) | Sensitivitas setelah asumsi fee 0,1% per sisi |
|---|---:|---:|---:|---:|
| EMA cepat ETH 1m | 59 | 15 | −6,2977 | −56,9515 |
| EMA tren BTC 1h | 1 | 1 | +0,9038 | +0,0255 |
| Grid SOL | 100 | 85 | +84,9106 | +81,4048 |
| MM DOGE | 40 | 19 | +0,3876 | −1,1958 |
| DCA XRP | 0 | — | Belum terealisasi | — |

Sensitivitas memakai `0,001 × (notional entry + exit)` untuk trade tutup saja,
tanpa slippage tambahan atau perubahan sizing. Ini asumsi pembanding, bukan
verifikasi tarif exchange atau simulasi live lengkap. Fee pembelian inventori
terbuka belum masuk kolom tersebut.

Ekuitas akhir yang dilaporkan engine **10.082,4309 USDT**, terhadap modal virtual
10.000: **+82,4309 USDT / +0,8243%**, termasuk unrealized +2,8790. Jangan menyamakan
modal ini dengan trial ETH Rp2 juta yang belum dijalankan di VPS. Jumlah PnL
trade tutup +79,9043 tidak sepenuhnya cocok dengan balance gain +79,5519;
rekonsiliasi dan pembulatan perlu diaudit. Nilai ini belum layak dijadikan
proyeksi keuntungan uang asli.

Pemeriksaan independen arus kas fill (sell minus buy) ditambah inventori ledger
run 9 pada close candle terakhir 18.51 WIB memberikan **+82,3167 USDT**:
ETH −6,2977, BTC +0,9038, SOL +88,3820, XRP −0,6602, DOGE −0,0113.
Selisih dengan ekuitas final engine sekitar 0,1142 USDT; timestamp mark tidak
sama dan ada penyesuaian inventori saat rekonsiliasi. Belum terbukti bahwa seluruh
selisih berasal dari bug. Akun exchange juga memuat saldo testnet bawaan sehingga
saldo total koin tidak boleh langsung dianggap inventori strategi.

## Penilaian

**EMA ETH 1m tidak dilanjutkan sebagai kandidat utama.** Sudah rugi sebelum fee,
win rate trade tutup sekitar 25,4%, dan notional putaran 50.653,86 USDT menghabiskan
sekitar 50,65 USDT jika dikenakan asumsi komisi tersebut. Ini sejalan dengan
riset historis yang tersimpan di proyek. Ada 30 partial close beralasan stop loss,
14 take profit, dan 15 exit biasa; jangan membacanya sebagai 59 entry unik.

**EMA BTC belum cukup bukti.** Hanya satu trade, dan penutupan dipicu masalah
linked protective order. Sampel ini tidak membuktikan edge.

**Grid SOL menghasilkan laba terbesar dalam jendela ini**, tetapi bergantung
pada kondisi harga testnet. Masih ada inventori, dan pengisian partial membuat
100 trade tutup berbeda dari jumlah order sell filled. Enam buy tersisa sebelum
pembatalan. Belum diperiksa satu per satu apakah setiap partial fill mendapat
order balasan dan level yang tepat; keuntungan testnet belum meluluskan audit itu.

**DCA XRP mencapai 24 buy**, nominal total 478,41715 USDT, posisi 368,1 XRP,
rata-rata entry 1,2996934. Close terakhir 1,2979 memberikan unrealized sekitar
−0,6602. Tidak ada siklus take profit 3% selesai. Jeda buy pertama–kedua sekitar
100 menit setelah dua restart, lalu sekitar 60–61 menit; implementasi restart
memulai penantian satu interval baru. Jadi ini bukan bukti jadwal persis setiap
jam. Setelah buy ke-24 tidak ada pembelian tambahan. Strategi masih tanpa stop.

**MM DOGE marginnya tipis dan pencatatannya perlu diperiksa.** Buy order dibatalkan
719 kali dan sell 51 kali; ada 80 fill, tetapi 40 trade tutup saja. Fill bersih
10.192−10.104 = 88 DOGE, sedangkan posisi akhir 83 DOGE. Lima WARN posisi ditutup
di luar bot masing-masing mencatat DOGE 1. Ini cocok secara jumlah dengan
penyesuaian lima DOGE, tetapi perlu audit rekonsiliasi untuk menjelaskan penyebab
dan pengakuan PnL. PnL trade positif kecil tidak sama dengan laba arus kas plus
inventori, yang mendekati nol dan akan tergerus komisi.

## Operasional dan pekerjaan aplikasi berikutnya

Ada **521 sampel observasi**, semuanya `problems: []`, dari 18 Oktober 23.43 WIB
hingga 20 Oktober 18.49 WIB. Run terakhir tidak restart dan tidak OOM. Ini bukti
ketersediaan proses/feed menurut pemeriksaan script, bukan bukti bebas cacat
transaksi. Log yang masih tersedia dari container terakhir memuat:

- 23 `cancel failed`; periksa race pembatalan pasangan OCO dan status terminal.
- Dua `protective orders failed, closing position` pada ETH; audit penolakan
  protection, fallback exit dan kemungkinan fill terlambat. Penyebab belum
  dipastikan dari bukti yang disaring.
- Satu pasangan BTC expired lalu bot menutup posisi lewat market.
- Lima penyesuaian posisi DOGE di luar bot; audit kuantitas, dust dan akuntansi.
- 12 `telegram poll failed`, satu `telegram send failed`.
- Dua reconnect WS, serta satu peringatan XRP tanpa stop sesuai desain DCA.

Log Compose tersedia untuk container rilis terakhir; angka di atas tidak
mengklaim seluruh log dari container lama run 7/8 ikut terarsip. Rentang ekuitas
DB 9.979,02–10.736,07 dan drawdown tercatat sekitar 6,11% perlu diverifikasi
terhadap lonjakan harga/mark testnet sebelum dipakai sebagai ukuran risiko.
Kolom exchange equity berisi `binance-testnet:spot`, sedangkan order/fill memakai
`binance-testnet`; filter yang keliru akan menghasilkan equity kosong.

Urutan kerja berikutnya yang disiapkan untuk pengembangan:

1. Audit proteksi OCO, fill terlambat dan penutupan posisi; buat reproduksi dengan
   payload testnet sebelum memperbaiki engine/adapter.
2. Audit rekonsiliasi saldo, dust DOGE, realized PnL dan perubahan mark; pisahkan
   modal strategi dari saldo testnet bawaan di laporan.
3. Tambahkan prosedur akhir eksperimen yang membatalkan order milik sesi,
   mencatat status exchange final, dan membedakan stop untuk resume vs selesai.
4. Nilai kandidat strategi baru pada data mainnet historis dengan fee/slippage,
   split waktu dan pembanding buy-and-hold. Bekukan aturan sebelum uji baru.
   Referensi ETH momentum 4h tetap calon riset, belum dipilih atau dijalankan.

Belum ada strategi baru dipasang. Keputusan sekarang adalah mengakhiri trial,
menyimpan pelajaran, lalu mengerjakan riset strategi dan perbaikan aplikasi.

## Bukti

- [evidence.json](evidence.json): agregat order/fill/trade, posisi, observasi,
  pembatalan exchange dan saldo testnet terpilih; tanpa kredensial.
- [supplement.json](supplement.json): 3.003 equity snapshot run 7–9,
  pengelompokan exit dan error dengan isi sensitif disaring.
- [summary.json](summary.json): perhitungan arus kas dan sensitivitas komisi.
- Bukti lengkap DB/config: backup final di VPS, tidak disalin ke dokumentasi.
