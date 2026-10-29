# Dokumentasi trading-bot

Mulai dari [handover agent](handover-agent.md), lalu pilih panduan yang diperlukan.

**Terbaru — 29 Oktober 2026, 22:23 WIB:** patch feed/health P2, schema/UI P3
serta helper/timer P3 sudah commit/push/CI/deploy pada **`d30b93b`**.
[Laporan rilis dan batas bukti](testing/paper-operations-release.md).
Dua EMA 1m tetap pause, saldo/T0/parameter sama. Feed awal membaik; P3 masih
memiliki alert review WATCH lama yang invalid meskipun runtime/feed sehat.

| Dokumen | Isi |
|---|---|
| [Handover agent](handover-agent.md) | Status terbaru, batas kerja dan langkah lanjut untuk Codex/Claude/agent lain |
| [Handover pilot](catalyst-pilot-handover.md) | Riwayat implementasi/pause/resume hingga start, pilihan user dan pekerjaan tersisa |
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

Rencana aktif repo ini tetap **Plan 3**. Klarifikasi user 29 Oktober:
Plan 4 yang sempat muncul dari memori perangkat Windows adalah proyek lain
dengan repository GitHub berbeda; tidak berlaku untuk trading-bot ini.
Plan 2 ditutup sebagai siklus pada 23 Oktober; backlog-nya tetap di Plan 3.

Pilot P3 memakai paper 1.000 USDT, maksimal 12 panggilan AI/hari WIB dan DB/wallet
sendiri. Sumber Ethereum/Anza/Dogecoin sudah dikoleksi pada start; detail rilis,
digest dan batas bukti di [handover VPS](handover-vps.md). Aktivasi
observer/alert permanen, backup/restore flat dan QA Chromium sudah selesai.
Alert review invalid tetap dipertahankan. Review hari 1
pada **29 Oktober 22.53 WIB**, hari 3/7 pada 31 Oktober/4 November.

P2 tetap enam instance/18 binding BTC/ETH/SOL, T0 **22 Oktober 19:58:45 WIB**.
Review terbaru memakai snapshot 29 Oktober 20:34 WIB; dua EMA 1m merugi,
empat instance lain belum bertransaksi. Dua EMA 1m sudah pause pada 21:10 WIB;
P2 kemudian dirilis ulang pada 22:23 WIB; parameter/modal/T0 dan ledger
tetap dipertahankan.
[Riset biaya 12 kasus](testing/strategy-cost-review/readme.md) selesai; kandidat
baru belum diaktifkan. Tidak ada izin uang asli.

## Aturan dokumentasi

- Nama file/folder di `docs/` memakai kebab-case huruf kecil, tanpa tanggal.
- Dokumen pertama tanpa `-1`; kelanjutan satu seri memakai `-2`, `-3`, dst.
  Topik berbeda tetap memakai nama deskriptif. Tanggal/periode ada di isi laporan.
- Dokumentasi umum ringkas: status, tindakan, batas dan tautan bukti. Plan dan
  strategi tetap rinci, termasuk alasan, parameter, risiko dan kriteria evaluasi.
- Handover netral antar-agent. Perbarui status terkini, hindari instruksi lama
  yang bertentangan, dan perbarui rujukan serta output script saat mengganti nama.
- Bukti JSON historis tidak ditulis ulang untuk merapikan narasi atau path lama.
