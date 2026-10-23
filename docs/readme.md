# Dokumentasi trading-bot

Mulai dari [handover agent](handover-agent.md), lalu pilih panduan yang diperlukan.

| Dokumen | Isi |
|---|---|
| [Handover agent](handover-agent.md) | Status terbaru, batas kerja dan langkah lanjut untuk Codex/Claude/agent lain |
| [Plan 3](plan-3.md) | Rencana aktif: katalis/AI, hold berbasis tesis, proteksi dan seluruh backlog Plan 2 |
| [Plan 2](plan-2.md) | Siklus multi-instance selesai/arsip; checklist dan bukti historis |
| [Strategi 2](strategy-2.md) | Keputusan terbaru: tarik trial ETH, kandidat dan protokol riset |
| [Strategi](strategy.md) | Keputusan awal ETH momentum 4h, parameter dan bukti historis |
| [Strategi VPS](strategy-vps.md) | Referensi rinci profil ETH dan persiapan trial terisolasi |
| [Runtime managed](managed-runtime.md) | Konfigurasi, kontrol, backtest dan recovery |
| [Multi-instance](multi-instance.md) | Model modal, ownership dan batas risiko |
| [Handover VPS](handover-vps.md) | Checkpoint server, akses dan batas operasional |
| [Deployment](deployment.md) | Rilis, backup, migrasi dan rollback |
| [Bukti pengujian](testing/readme.md) | Indeks laporan dan data pendukung |
| [Plan](plan.md) | Arsip siklus pertama; bukan backlog aktif |

Plan 2 ditutup 23 Oktober sebagai siklus; seluruh sisa pekerjaan ada di Plan 3.
Strategi AI belum diimplementasikan/diaktifkan. Presale/token baru menjadi
future reference; memecoin yang sudah diperdagangkan termasuk universe riset.

Checkpoint operasi terbaru 23 Oktober 20.52 WIB: rilis notifikasi `9c18962`
pada paper pembanding tahap dua, enam instance/18 binding BTC/ETH/SOL. T0 tetap
22 Oktober 19:58:45 WIB, modal awal 747,83957456237531 USDT virtual dan CPU1.
Review user 23/25/29 Oktober pukul 19:58:45 WIB; ledger lama dipertahankan.
Detail dan batas penerimaan ada di [handover VPS](handover-vps.md).
Ini checkpoint terdokumentasi, bukan status server real-time atau izin uang asli.

## Aturan dokumentasi

- Nama file/folder di `docs/` memakai kebab-case huruf kecil, tanpa tanggal.
- Dokumen pertama tanpa `-1`; kelanjutan satu seri memakai `-2`, `-3`, dst.
  Topik berbeda tetap memakai nama deskriptif. Tanggal/periode ada di isi laporan.
- Dokumentasi umum ringkas: status, tindakan, batas dan tautan bukti. Plan dan
  strategi tetap rinci, termasuk alasan, parameter, risiko dan kriteria evaluasi.
- Handover netral antar-agent. Perbarui status terkini, hindari instruksi lama
  yang bertentangan, dan perbarui rujukan serta output script saat mengganti nama.
- Bukti JSON historis tidak ditulis ulang untuk merapikan narasi atau path lama.
