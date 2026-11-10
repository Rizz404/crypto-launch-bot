# Dokumentasi presale dan listing baru

**Checkpoint 10 November 2026:** [Plan 4](plan-4.md) tetap satu-satunya
roadmap aktif repo ini. Repo `Rizz404/trading-bot` (lanjutan dari commit
dasar yang sama, aktif sampai 8 November) sudah di-merge: UI bersama,
website trading, marketview Binance, perbaikan catalyst dan penghapusan
Bybit ikut masuk. Plan 4 milik repo itu (website trading) disimpan sebagai
[website trading](trading-website.md) dengan tahap `TW-*` agar tidak bentrok
dengan tahap `P4-*` presale.

## Dokumen aktif

| Dokumen | Isi |
|---|---|
| [Plan 4](plan-4.md) | Fase implementasi dan gerbang penerimaan |
| [Handover agent](handover-agent.md) | Keputusan terbaru, kondisi kode dan batas tindakan |
| [Strategi presale/listing](presale-strategy.md) | Universe, discovery, tesis dan lifecycle |
| [Arsitektur](presale-architecture.md) | Alur, kontrak data, reuse dan kebutuhan integrasi |
| [Sumber data](presale-sources.md) | Protokol riset dan pemilihan chain/venue/provider |
| [Integrasi awal](presale-integrations.md) | Hasil P4-1, referensi resmi, kontrak v1 dan gap |
| [Discovery/report](presale-runtime.md) | CLI P4-2, arsip dan status riset tanpa transaksi |
| [Analisis/policy](presale-analysis.md) | P4-3: profil disetujui, schema/validator, kuota dan metode evaluasi |
| [Simulasi paper](presale-paper.md) | P4-4: journal, adapter fixture tiga jalur, settlement dan recovery |
| [Checkpoint P4](testing/plan-4-local.md) | Implementasi dan hasil verifikasi |
| [Risiko](presale-risk.md) | Gate transaksi, dana terkunci dan kegagalan |
| [Evaluasi](presale-evaluation.md) | Simulasi, skenario pengujian dan penerimaan pilot/live |
| [Website trading](trading-website.md) | Hasil impor trading-bot: dashboard, akun multi-mode, order manual |
| [Panduan web/kontrol](guide.md) | Cara kerja wallet/instance/binding dan kontrol dashboard |
| [Arsip](archive.md) | Seluruh panduan dan keputusan Plan 1–3 |
| [Bukti historis](testing/readme.md) | Laporan dan data pendukung lama |

`agents/` berisi dokumen detail dari trading-bot (handover, runtime, deploy,
arsip Plan 1–3). Status VPS di sana adalah checkpoint repo itu, bukan hasil
pemeriksaan dari repo ini. Instruksi resume, jadwal, parameter dan izin dalam
arsip berlaku pada konteks historisnya; tidak menjadi perintah operasi.

## Aturan dokumentasi

- Nama file/folder kebab-case huruf kecil, tanpa tanggal. Tanggal ada di isi.
- Seri plan memakai nomor berurutan; topik lain memakai nama deskriptif.
- Bedakan rancangan, kode yang ada, hasil pemeriksaan dan kemampuan operasional.
- Handover netral antar-agent; keputusan aktif harus konsisten dengan Plan 4.
- Simpan dokumen historis di path lama. Jangan mengubah payload, angka,
  timestamp atau checksum bukti untuk menyesuaikan narasi baru.
- Laporan baru menggunakan lokasi terpisah dari hasil lama; jangan menimpa
  bukti dengan menjalankan ulang script reproduksi.
