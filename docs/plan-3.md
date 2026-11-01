# Plan 3 — Riset katalis dan hold berbasis tesis

> **Ditutup 1 November 2026.** Semua eksperimen dihentikan setelah
> [review akhir](testing/final-review.md): tidak ada edge, sebagian besar sinyal tidak pernah entry,
> AI tidak pernah mendapat event yang bisa dieksekusi. Analisis berikutnya di Plan 4.

Rencana aktif sejak 23 Oktober 2026. Plan 1 dan 2 sudah selesai. Pekerjaan sisanya ada di sini.
Status di bawah per **30 Oktober 2026**; verifikasi sebelum mengandalkannya.

## Tujuan

Bot membeli/menahan/menjual koin berdasarkan **tesis** dari berita dan data, bukan sinyal indikator saja.
Alurnya: daftar website crypto → Spider (scraping) → AI + data pasar menentukan buy/hold/sell →
validasi risiko → eksekusi **paper**. Proteksi (stop loss, penurunan besar → jual semua) berjalan
sendiri dan tidak bergantung pada AI.

## Status sekarang

| Hal              | Kondisi                                                                                                                         |
| ---------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| Pilot AI (P3)    | Aktif sejak 28 Oktober 22:53 WIB. Modal virtual 1.000 USDT, risiko 0,5%/trade, maksimal 2 posisi, maksimal 12 panggilan AI/hari |
| Hasil pilot      | Belum ada transaksi (fill); belum ada bukti untung/rugi                                                                         |
| Pembanding P2    | 6 instance/18 binding BTC/ETH/SOL sejak 22 Oktober. Dua EMA 1m rugi dan sudah di-pause; empat lain belum bertransaksi           |
| Riset tahap tiga | Enam kasus BTC/ETH (pullback 4h, EMA 15m/1h) aktif sejak 29 Oktober 23:14 WIB                                                   |
| Server           | `plesk-newest`; server lama `plesk` hanya cadangan rollback sampai ±5 November                                                  |
| Uang asli        | **Tidak diizinkan**                                                                                                             |

## Masalah yang diketahui

- Worker pilot sempat restart berulang dan mengirim spam Telegram. Pesan spam sudah dihapus dan
  dirilis 30 Oktober, tetapi **akar penyebab restart belum terbukti** (error asli dibuang sebelumnya).
- Alamat gateway AI lama gagal TLS; sudah dipindah ke gateway baru. Inference nyata lewat gateway baru
  belum terbukti sukses.
- Alert review lama yang invalid masih ada.

## Keputusan user

- Fokus awal pada token yang sudah diperdagangkan; memecoin termasuk. Presale dan token baru ditunda.
- Waktu jual bergantung kondisi tiap koin; acara selesai tidak otomatis jual.
- Penurunan signifikan menjual seluruh sisa posisi strategi ini (angka batas belum disepakati).
- Eksperimen P2 dibiarkan berjalan; pilot AI memakai modal dan ledger sendiri.

## Yang masih terbuka

1. Review pilot hari 3 (31 Oktober) dan hari 7 (4 November), serta evaluasi bersih setelah biaya.
2. Memastikan pilot stabil (akar restart) dan model AI memberi keputusan valid.
3. Uji pemulihan saat ada posisi/order aktif dan saat outage.
4. Lanjutan riset pembanding (target 44 instance/132 binding, baru 6/18 aktif).
5. Keputusan uang asli: hanya setelah bukti cukup dan persetujuan eksplisit user.
6. Ditunda: Futures, Bybit, arbitrase, presale/token baru.
