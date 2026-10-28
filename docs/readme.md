# Dokumentasi trading-bot

Mulai dari [handover agent](handover-agent.md), lalu pilih panduan yang diperlukan.

| Dokumen | Isi |
|---|---|
| [Handover agent](handover-agent.md) | Status terbaru, batas kerja dan langkah lanjut untuk Codex/Claude/agent lain |
| [Handover Spider](catalyst-spider-handover.md) | Arah website→Spider→AI, patch/protokol, rilis komponen VPS dan gate pilot |
| [Plan 3](plan-3.md) | Rencana aktif: katalis/AI, hold berbasis tesis, proteksi dan seluruh backlog Plan 2 |
| [Plan 2](plan-2.md) | Siklus multi-instance selesai/arsip; checklist dan bukti historis |
| [Strategi 2](strategy-2.md) | Keputusan terbaru: tarik trial ETH, kandidat dan protokol riset |
| [Strategi](strategy.md) | Keputusan awal ETH momentum 4h, parameter dan bukti historis |
| [Strategi VPS](strategy-vps.md) | Referensi rinci profil ETH dan persiapan trial terisolasi |
| [Runtime managed](managed-runtime.md) | Konfigurasi, kontrol, backtest dan recovery |
| [Runtime katalis](catalyst-runtime.md) | Komponen/CLI development Plan 3, replay fixture dan batas penerimaan pilot |
| [Multi-instance](multi-instance.md) | Model modal, ownership dan batas risiko |
| [Handover VPS](handover-vps.md) | Checkpoint server, akses dan batas operasional |
| [Deployment](deployment.md) | Rilis, backup, migrasi dan rollback |
| [Bukti pengujian](testing/readme.md) | Indeks laporan dan data pendukung |
| [Plan](plan.md) | Arsip siklus pertama; bukan backlog aktif |

Checkpoint development 28 Oktober: CLI collect kini Spider default dan race
test/vet/lint komponen lokal lulus. Image P3 dibangun/smoke; binary patched
2.53.9-p3.1 lulus TLS fixture/proxy terisolasi tiga kali. Egress adversarial/
situs publik dan pilot masih terbuka. **Seluruh CI dan deployment komponen/report
VPS lulus 28 Oktober 21.00 WIB**, source `2998f38`; worker/pilot belum aktif dan
P2 tidak direstart. [Bukti rilis](testing/catalyst-release.json).
Lanjutkan dari [handover Spider](catalyst-spider-handover.md).

Plan 2 ditutup 23 Oktober sebagai siklus; seluruh sisa pekerjaan ada di Plan 3.
Komponen collector/validator dan replay paper lokal tersedia; worker/pilot AI
belum aktif. [Evaluasi OmniRoute](testing/catalyst-model-evaluation.md) 26 Oktober
memilih Luna; combo/failover dan shadow prospektif masih terbuka. Presale/token baru menjadi
future reference; memecoin yang sudah diperdagangkan termasuk universe riset.

Checkpoint operasi P2 23 Oktober 20.52 WIB: rilis notifikasi `9c18962`
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
