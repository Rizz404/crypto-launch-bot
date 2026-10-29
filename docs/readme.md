# Dokumentasi presale dan listing baru

**Checkpoint 29 Oktober 2026:** arah repository diganti melalui perombakan
dokumentasi. [Plan 4](plan-4.md) menjadi satu-satunya roadmap aktif. Belum ada
implementasi, pilot atau hasil pengujian strategi baru.

## Dokumen aktif

| Dokumen | Isi |
|---|---|
| [Plan 4](plan-4.md) | Fase implementasi dan gerbang penerimaan |
| [Handover agent](handover-agent.md) | Keputusan terbaru, kondisi kode dan batas tindakan |
| [Strategi presale/listing](presale-strategy.md) | Universe, discovery, tesis dan lifecycle |
| [Arsitektur](presale-architecture.md) | Alur, kontrak data, reuse dan kebutuhan integrasi |
| [Sumber data](presale-sources.md) | Protokol riset dan pemilihan chain/venue/provider |
| [Risiko](presale-risk.md) | Gate transaksi, dana terkunci dan kegagalan |
| [Evaluasi](presale-evaluation.md) | Simulasi, skenario pengujian dan penerimaan pilot/live |
| [Arsip](archive.md) | Seluruh panduan dan keputusan Plan 1–3 |
| [Bukti historis](testing/readme.md) | Laporan dan data pendukung lama |

Keputusan terbaru menggantikan penundaan presale/token baru pada Plan 3.
Instruksi resume, status aktif, jadwal, parameter dan izin dalam arsip berlaku
pada konteks historisnya; tidak menjadi perintah melanjutkan pilot P3.

## Aturan dokumentasi

- Nama file/folder kebab-case huruf kecil, tanpa tanggal. Tanggal ada di isi.
- Seri plan memakai nomor berurutan; topik lain memakai nama deskriptif.
- Bedakan rancangan, kode yang ada, hasil pemeriksaan dan kemampuan operasional.
- Handover netral antar-agent; keputusan aktif harus konsisten dengan Plan 4.
- Simpan dokumen historis di path lama. Jangan mengubah payload, angka,
  timestamp atau checksum bukti untuk menyesuaikan narasi baru.
- Laporan baru menggunakan lokasi terpisah dari hasil lama; jangan menimpa
  bukti dengan menjalankan ulang script reproduksi.
