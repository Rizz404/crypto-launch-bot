# Panduan singkat

## Cara kerja

- Satu akun/mode punya satu **wallet** (modal). Wallet dibagi ke **instance** (portofolio virtual);
  tiap instance punya satu atau lebih **binding** (strategi + simbol). Instance bukan container.
- Alokasi dan transfer atomik, tidak mengulang kredit. Cap strategi bukan saldo tambahan.
  Setoran/transfer bukan profit.
- Buy mengunci uang + buffer fee. Jual hanya koin milik binding itu; tidak ada short Spot.
- Restart memulihkan state, lalu rekonsiliasi dengan exchange. Entry ditahan bila data tidak cocok.
- Secret hanya lewat environment, tidak di YAML/laporan. Paper tidak butuh key trading.

## Kontrol

Lewat dashboard web atau Telegram. Menu samping web: **Trading bot** (Ringkasan, Instance & strategi,
Posisi & order, Riwayat; halaman awal setelah login), **Trading manual** (segera hadir), **Akun trading**
(pindah akun paper/testnet/live), **Ganti password**, **Keluar**.

| Aksi | Efek |
|---|---|
| Pause | Tahan entry baru; exit/proteksi tetap jalan |
| Resume | Lanjut, hanya jika state sehat |
| Cancel | Batalkan order entry, tidak menjual posisi |
| Flatten | Jual seluruh inventori scope (butuh konfirmasi) |
| Finish | Selesaikan order, simpan atau tutup inventori |
| Kill | Hentikan entry sampai di-reset |
| Archive | Tutup instance yang sudah kosong |

Telegram: `/status`, `/balance`, `/pnl`, `/positions`, `/orders`, `/strategies`, `/pause`, `/flatten`,
`/finish`, `/stop`. Aksi dana minta `confirm`. Ringkasan harian pukul 07.00 WIB; setiap transaksi
dilaporkan bila level notifikasi `info`.

Dashboard memakai Basic Auth dan hanya bind loopback. Jangan buka port ke publik.

## Halaman Trading manual (Plan 4, ditunda dari menu)

Belum ditautkan di menu sampai bisa memakai pair bebas; alamat `/trade` masih bekerja.

`/trade`: chart candle live, order book, transaksi pasar, form order manual, serta posisi, order
terbuka, riwayat dan saldo. Switcher akun di kanan atas (badge PAPER / TESTNET / **LIVE merah**).

- Order manual hanya lewat binding strategi `manual` di sebuah instance; modal dan batas risikonya
  ikut instance. Market buy bisa diberi stop loss/take profit. Bot bisa memperkecil jumlah order.
- Posisi bot bisa ditutup dan order entry bot bisa dibatalkan dari tabel. Order proteksi (SL/TP)
  tidak bisa dibatalkan satu per satu; tutup posisinya.
- Login email + password. Akun (satu pemilik) disimpan di database, password di-hash PBKDF2, bukan di env.
  Selama belum ada akun, `/login` mengarah ke `/register`, yang meminta **kode setup sekali pakai** dari log bot
  (`docker logs ana-core-bot-1 | grep setup_code`). Setelah akun dibuat, register tertutup.
- Form **Tambah strategi** (Instance & strategi, instance harus dijeda): Momentum, Tren pullback, EMA cross, DCA,
  Grid (satu pair, butuh batas bawah/atas dan jumlah level) dan Manual. Pair bisa beberapa, dipisah koma.
  Strategi baru aktif setelah bot di-restart; salah ketik pair membuat bot menolak start.
- Ganti password di `/account` (tombol **Akun** di header); sesi lain ikut keluar. Aksi akun live meminta
  password akun lagi.
- Data pasar: satu websocket Binance per jaringan untuk semua tab; browser tidak memakai kuota API.

## Menjalankan dan deploy

- **Jangan menjalankan bot atau trial di laptop.** Runtime ada di server `plesk-newest`
  (`/opt/ana-core` untuk P2 + riset, `/opt/ana-scout` untuk pilot AI).
- Rilis: commit → GitHub Actions → image GHCR dengan digest tetap → server. Tidak pernah `latest`.
- Server juga berisi proyek Plesk lain milik user; jangan ubah service, port, firewall atau Docker host.
- Backup harian otomatis (±03:15 UTC untuk core, 03:45 UTC untuk pilot), disimpan di server saja.
  Restore hanya ke database salinan. Rollback tidak mengembalikan database.
- Trial baru memakai database/wallet baru dengan modal eksplisit, bukan melanjutkan saldo lama.

## Perintah lokal (pengembangan)

```sh
make test               # unit test
make test-integration   # butuh `make up`
make lint
make generate           # setelah mengubah *.templ / input.css / app.js
./bin/catalyst -action report    # laporan keputusan/sumber AI (baca saja)
```

Pengujian dan perubahan kode dilakukan lewat agent; detail teknis ada di `agents/`.
