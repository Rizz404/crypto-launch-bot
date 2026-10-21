# Dokumentasi trading-bot

Mulai dari [handover agent](handover-agent.md), lalu pilih panduan yang diperlukan.

| Dokumen | Isi |
|---|---|
| [Handover agent](handover-agent.md) | Status terbaru, batas kerja dan langkah lanjut untuk Codex/Claude/agent lain |
| [Plan 2](plan-2.md) | Rencana aktif, checklist dan kriteria penerimaan |
| [Strategi 2](strategy-2.md) | Keputusan terbaru: tarik trial ETH, kandidat dan protokol riset |
| [Strategi](strategy.md) | Keputusan awal ETH momentum 4h, parameter dan bukti historis |
| [Strategi VPS](strategy-vps.md) | Referensi rinci profil ETH dan persiapan trial terisolasi |
| [Runtime managed](managed-runtime.md) | Konfigurasi, kontrol, backtest dan recovery |
| [Multi-instance](multi-instance.md) | Model modal, ownership dan batas risiko |
| [Handover VPS](handover-vps.md) | Checkpoint server, akses dan batas operasional |
| [Deployment](deployment.md) | Rilis, backup, migrasi dan rollback |
| [Bukti pengujian](testing/readme.md) | Indeks laporan dan data pendukung |
| [Plan](plan.md) | Arsip siklus pertama; bukan backlog aktif |

Checkpoint 21 Oktober 2026: ETH paper VPS sudah dihentikan atas permintaan user;
modal virtual tetap 110,791048 USDT tanpa posisi/order/eksekusi. Data dan backup
tersimpan. Fokus kembali ke riset; tidak ada pengganti aktif atau izin mainnet.
Ini checkpoint terdokumentasi, bukan status server real-time.

## Aturan dokumentasi

- Nama file/folder di `docs/` memakai kebab-case huruf kecil, tanpa tanggal.
- Dokumen pertama tanpa `-1`; kelanjutan satu seri memakai `-2`, `-3`, dst.
  Topik berbeda tetap memakai nama deskriptif. Tanggal/periode ada di isi laporan.
- Dokumentasi umum ringkas: status, tindakan, batas dan tautan bukti. Plan dan
  strategi tetap rinci, termasuk alasan, parameter, risiko dan kriteria evaluasi.
- Handover netral antar-agent. Perbarui status terkini, hindari instruksi lama
  yang bertentangan, dan perbarui rujukan serta output script saat mengganti nama.
- Bukti JSON historis tidak ditulis ulang untuk merapikan narasi atau path lama.
