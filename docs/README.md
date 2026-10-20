# Dokumentasi trading-bot

## Mulai dari sini

| Dokumen | Fungsi |
|---|---|
| [Handover Claude](HANDOVER-CLAUDE.md) | Titik lanjut development, status implementasi dan batas operasional |
| [Runtime managed](managed-runtime.md) | Config, kontrol, backtest, recovery dan migrasi multi-instance |
| [Tes lokal Plan 2](testing/plan2-local-2026-10-20.md) | Bukti pengujian/fixing dan keterbatasan |
| [Plan 2](plan-2.md) | Rencana aktif, fase implementasi, backlog pindahan dan status |
| [Rancangan multi-instance](multi-instance.md) | Akun → instance → strategi, pembagian dana, ownership dan kontrol |
| [Keputusan strategi 20 Oktober](strategy-decision-2026-10-20.md) | ETH momentum 4h sebagai kandidat paper, BTC harian pembanding |
| [Handoff VPS](handoff-vps.md) | Status server terakhir yang diverifikasi dan batas operasional |
| [Deployment](deployment.md) | Prosedur rilis CI/GHCR/digest, backup dan rollback |
| [Referensi ETH VPS](eth-paper-vps-reference.md) | Persiapan trial ETH yang belum dijalankan di VPS |
| [Bukti pengujian](testing/README.md) | Hasil pengujian dan sumber laporan |
| [Plan 1 — selesai/arsip](plan.md) | Riwayat fondasi dan hasil siklus pertama, bukan backlog aktif |

Per 20 Oktober 2026: Plan 1 ditutup; Plan 2 dimulai dengan
[audit P2-0 dan dua perbaikan reconciliation lokal](testing/p2-0-audit-2026-10-20.md).
Implementasi lokal P2-1–P2-6 tersedia; testing/fixing berjalan, visual browser
masih terbuka. Belum dirilis ke VPS. Bot testnet VPS sudah dihentikan, belum ada trial ETH baru atau live.
Penutupan plan tidak mengubah hasil tes menjadi lulus atau mengizinkan mainnet.
Status server di atas adalah checkpoint terakhir, bukan pemeriksaan real-time.
