> **ARSIP — 29 Oktober 2026.** Dokumen ini merekam proyek Plan 1–3.
> Roadmap aktif: [Plan 4](plan-4.md); arahan terkini: [handover agent](handover-agent.md).
> Pernyataan aktif, izin, jadwal, parameter dan instruksi resume di bawah berlaku
> pada checkpoint historisnya, bukan arahan melanjutkan pilot P3 atau transaksi.
> Pengarsipan tidak menghentikan proses lama. Lihat [indeks arsip](archive.md).

---

# Referensi paper ETH di VPS

**Pembaruan 21 Oktober 2026:** trial managed P2-9 yang dimulai
2026-10-20 16:24:45 UTC sudah dihentikan atas permintaan user pada 21 Oktober
20.30 WIB. Jangan resume; data/backup dipertahankan. Lihat
[keputusan riset terbaru](strategy-2.md) dan
[handoff VPS](handover-vps.md). Isi di bawah adalah riwayat persiapan.

Status 19 Oktober 2026: **belum di-deploy di VPS, tidak berjalan di laptop,
dan belum disetujui untuk live**. User meminta tidak menjalankan eksperimen ini
di laptop; VPS boleh dipakai bila tidak mengganggu instance yang sudah berjalan.
Jika belum siap, simpan referensi untuk sesudah pengujian VPS selesai.

## Hubungan dengan rencana aktif

[Plan 3](plan-3.md) adalah rencana aktif; Plan 1/2 menjadi arsip.
Audit/penerimaan lama diteruskan ke P3-O; P2-0 sampai P2-9 di bawah adalah
referensi historis. Pilot AI tidak otomatis mewarisi aturan ETH. Istilah project
Compose/instance kedua pada runbook ini menjelaskan isolasi deployment
eksperimen, berbeda dari [instance virtual dalam satu akun](multi-instance.md).
Jangan membuat beberapa proses trading pada wallet yang sama untuk meniru
fitur multi-instance; runtime managed sudah tersedia, backlog mengikuti Plan 3.

## Pembaruan keputusan 20 Oktober 2026

Rekomendasi setelah analisis adalah ETH momentum 4h untuk kandidat paper utama,
dengan BTC momentum harian sebagai pembanding. Baca
[keputusan strategi](strategy.md) untuk alasan dan syaratnya.
Belum ada trial baru dijalankan. Catatan 19 Oktober di bawah adalah riwayat
persiapan; status bot testnet sudah berhenti dan source lokal sudah di-commit,
belum menjadi rilis VPS.

## Keputusan saat ini

Uji lokal dihentikan atas permintaan user, termasuk container database khususnya.
Data dipertahankan di `var/paper-eth/` dan volume Podman
`trading-bot-eth-paper-data`. Ini bukan kegagalan strategi atau hasil akhir.
Helper lokal `scripts/eth-paper.py` diblokir dari start/resume/worker oleh marker
`var/paper-eth/disabled-by-user.json`. Jangan menghapus marker atau melanjutkan
eksperimen lokal tanpa instruksi baru dari user.

Pemeriksaan VPS read-only melalui `ssh plesk` menemukan sekitar 21 GiB RAM
available, 52 GiB disk kosong (88% terpakai), stack testnet masih berjalan,
dan baseline proyek lain `differences: []`. Kapasitas bukan hambatan utama.
Namun checkout VPS masih `f709429`, sedangkan `trendspot` dan konfigurasi riset
baru masih merupakan perubahan lokal yang belum dirilis. Workflow saat ini
hanya menerbitkan image pada `main`, bukan branch riset. Branch sendiri tidak
mengisolasi proses, database, jaringan, atau Telegram.

Dipilih opsi dokumentasi yang user izinkan: **tidak ada deployment baru sekarang**.
Tidak membuat branch, push, image, restart, migrasi database VPS, atau perubahan
konfigurasi instance berjalan. Jangan mengklaim eksperimen ETH sudah berjalan
di VPS. Riwayat startup lokal ada di
[bukti bootstrap/start](testing/eth-paper-start.json), bukan bukti trial
prospektif yang selesai. Timestamp penghentian ada di
`var/paper-eth/disabled-by-user.json`.

## Strategi yang dibekukan untuk uji berikutnya

- Satu pasangan **ETHUSDT**, Binance Spot dengan harga mainnet publik, order paper.
- Modal total **Rp2 juta** = 110,79104808331486 USDT pada kurs referensi Rp18.052.
  Konversi ini asumsi awal, bukan janji nilai rupiah tetap.
- Sinyal momentum **4h**: perubahan close selama 84 bar (>5%) dan close > EMA100.
- Exit pada momentum <=0 atau close < EMA100; stop awal 3 × ATR14;
  trailing berdasarkan persentase 3 × ATR14 / close saat entry; tanpa target tetap.
- Maksimum satu posisi, tanpa leverage/DCA/pyramiding. Posisi maksimum 75%,
  risk terencana 3% per transaksi, daily loss 8%, drawdown 30%, close on kill.
  Ini profil paper yang diuji, bukan persetujuan batas kerugian live.
- Fee standar 0,1% per sisi dan slippage 2bps. Jangan mengasumsikan diskon BNB.
- Detail lengkap: [referensi terstruktur](testing/eth-paper-reference.json)
  dan [konfigurasi strategi](../configs/paper.eth.yaml). Konfigurasi tersebut
  adalah template strategi; port, secret, koneksi DB, dan pembatasan deployment
  tetap harus disiapkan khusus VPS.

## Kapan dilanjutkan

Uji testnet VPS **dihentikan lebih awal pada 20 Oktober 2026 pukul 18.52 WIB**
atas permintaan user; target 21 Oktober pukul 23.43 WIB dibatalkan.
[Analisis penutupan](testing/testnet-stop/analysis.md) sudah tersedia.
User ingin mencari strategi lain dan mengembangkan aplikasi. ETH kini direkomendasikan sebagai kandidat paper utama sesuai
[keputusan 20 Oktober](strategy.md), tetapi trial belum dimulai.
Nilai kembali hasil dan keandalan aplikasi sebelum menyiapkan deployment berikutnya.
Tidak ada jadwal otomatis untuk memulai ETH, dan tidak ada polling AI.

Tanggal review 26 Oktober untuk trial lokal sebelumnya **dibatalkan**. Trial VPS
kelak memiliki tanggal mulai dan jendela evaluasi sendiri. Riwayat lokal yang
singkat tidak dihitung sebagai hari observasi trial VPS.

## Rencana deployment tanpa mencampur hasil

1. Catat baseline container/service dan kondisi instance lama sebelum perubahan.
   Periksa ulang kapasitas, pemakaian disk, port, subnet, dan batas sumber daya.
2. Rilis implementasi `trendspot` dan konfigurasi yang direview lewat GitHub/CI,
   mengikuti [prosedur deployment](deployment.md). Pin image ke digest dan source
   commit yang sama. Jangan salin binary/folder laptop atau memakai tag mutable.
   Branch riset boleh dibuat saat persiapan rilis; jika dipakai untuk image,
   workflow harus menerbitkan image branch secara eksplisit tanpa mengganti
   deployment lama.
3. Untuk paralel, siapkan project Compose terpisah, misalnya
   `trading-bot-eth-paper`, dengan checkout/direktori, database/volume, network,
   config, nama container, backup dan monitoring sendiri. Lokasinya ditentukan
   setelah review agar tidak melanggar batas proyek pada handoff VPS.
   Jangan menjalankan instance kedua dengan database testnet/paper lama:
   pemulihan sesi memilih run terakhir dan dapat menutup run yang dianggap
   terputus. Nama strategi/branch berbeda tidak cukup.
4. Paksa `app.mode: paper` dan `allow_live: false`; jangan memuat `.env.vps`
   instance lama. Harga publik tidak membutuhkan API key perdagangan. Database
   dan password dashboard baru dibuat khusus, jangan dicetak di log/chat.
5. Port kandidat loopback **18081** (web) dan **19101** (metrics), setelah cek
   kosong. DB tidak diekspos ke host/public. Resolver perlu disiapkan khusus
   karena DNS host VPS pernah memblokir Binance; jangan restart resolver lama
   atau menduplikasi subnet 172.31.253.0/29. Jangan ubah firewall/Docker daemon.
6. Batas awal bot: CPU 0,5, RAM 512 MiB; DB CPU 1, RAM 1 GiB; resolver dibatasi.
   Hindari menduplikasi Grafana/Prometheus bila belum perlu. Rotasi log dan backup
   data trial harus ikut disiapkan. Nilai lagi batas disk sebelum mulai.
7. Migrasi **database baru saja**. Prefill minimal 300 candle 4h sebelum membuat
   sesi; historical bars hanya mengisi indikator, bukan mengisi order masa lalu.
   Jika image tidak memiliki `datafetch` yang kompatibel, hentikan persiapan.
8. Telegram dimatikan pada instance kedua agar token yang sama tidak dipakai
   oleh dua long-poller. Jika user meminta notifikasi, siapkan jalur terpisah
   tanpa mengganggu bot Telegram lama; jangan kirim pesan tanpa otorisasi.
9. Verifikasi saldo awal, satu strategi/akun, feed aktual 1m dan 4h, warmup,
   order simulasi, persistence/restart, health/dashboard, dan baseline lama.
   Catat start time dan digest di bukti baru. Jangan menjalankan `down`, restart,
   migrasi, atau mengganti config pada project testnet lama untuk memasang ETH.

Jika memakai instance lama setelah tes selesai, tetap arsipkan hasil testnet
dan gunakan database trial ETH baru. Beralih mode saja pada DB lama dapat
melanjutkan saldo/posisi paper lama; itu bukan trial baru Rp2 juta.

## Penilaian

Review operasional pertama setelah tujuh hari **sejak start VPS yang sebenarnya**.
Periksa koneksi, data bolong, order/stop/trailing, fee, konsistensi saldo,
restart/OOM dan pemulihan. Gunakan parameter tetap dan catat semua transaksi.
Hasil harian yang sepi adalah normal: sekitar 37–38 transaksi tutup per tahun
pada pengujian ETH sebelumnya. Seminggu atau sebulan tidak membuktikan return
50% per tahun; ETH dipilih setelah melihat hasil beberapa koin dan kandidat 4h
ini gagal gerbang ketahanan semua lima koin. Tidak ada promosi otomatis ke live.

Untuk membatalkan deployment paralel kelak, stop **project ETH saja**; pertahankan
volume dan bukti. Hindari perintah global seperti prune atau down -v.

## Implementasi managed lokal

`configs/managed.eth-paper.yaml` mengekspresikan profil ETH-only pada ledger baru.
Belum di-deploy atau diaktifkan. Jangan mengganti config sesi lama lalu resume:
siapkan trial/DB baru setelah migrasi salinan, QA dan review P2-9. Baca
[panduan managed](managed-runtime.md) dan [handover](handover-agent.md).
