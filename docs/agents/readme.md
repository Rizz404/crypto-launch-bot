# Docs untuk agent

Untuk Codex, Claude dan agent lain. Manusia tidak perlu membaca folder ini; ringkasannya ada di
[docs/readme.md](../readme.md) (`plan-3.md`, `strategy-2.md`, `panduan.md`).

Versi **lengkap** dari dokumen ringkas itu ada di sini: [plan-3.md](plan-3.md), [strategy-2.md](strategy-2.md)
dan panduan rinci ([runtime managed](managed-runtime.md), [runtime katalis](catalyst-runtime.md),
[multi-instance](multi-instance.md), [deployment](deployment.md)). Saat status penting berubah
(pilot, keputusan, izin), perbarui juga ringkasan di root `docs/`.

## Urutan baca

1. [Handover agent](handover-agent.md): status terbaru, server, batas kerja, langkah lanjut.
2. [Handover VPS](handover-vps.md): checkpoint rilis, akses, jadwal review, batas operasional.
3. [Plan 3](plan-3.md): rencana aktif dan backlog.
4. Sesuai tugas:
   - [Handover pilot katalis](catalyst-pilot-handover.md): riwayat pilot AI P3.
   - [Handover Spider](catalyst-spider-handover.md): website → Spider → AI, patch dan protokol.
   - [Deployment](deployment.md), [runtime managed](managed-runtime.md),
     [runtime katalis](catalyst-runtime.md).
   - [Bukti pengujian](../testing/readme.md).
   - [Arsip](archive/plan.md): Plan 1, Plan 2, Strategi, Strategi VPS (historis).

## Aturan keras

- Verifikasi checkout, waktu dan runtime sebelum bekerja; dokumen ini adalah checkpoint, bukan status real-time.
- Server `plesk-newest` juga menjalankan project Plesk lain milik user. Jangan ganggu; ikuti batas di handover.
- Hanya paper/virtual. Tidak ada izin transaksi uang asli.
- Jangan menyalakan stack di `plesk` (rollback saja); dua poller Telegram dan dua penulis ledger dilarang.
- Bukti JSON di `testing/` tidak ditulis ulang; path lama di payload adalah provenance.
- Jangan commit/push/deploy tanpa izin user.
