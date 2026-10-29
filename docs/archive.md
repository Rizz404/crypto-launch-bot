# Arsip Plan 1–3

**Diarsipkan sebagai roadmap/panduan proyek lama pada 29 Oktober 2026.**
[Plan 4](plan-4.md) adalah satu-satunya roadmap aktif;
[handover agent](handover-agent.md) memuat arahan terkini.

Lokasi lama dipertahankan agar rujukan dan script tetap bekerja. Penanda arsip
ditambahkan pada panduan, sedangkan payload dan laporan hasil pengujian lama
dipertahankan. Status, jadwal, angka, izin, checklist dan instruksi resume
di bawah penanda merupakan catatan historis, bukan perintah operasi saat ini.
Pengarsipan bukan bukti semua pengujian lulus atau eksperimen telah berhenti.

## Rencana dan strategi

| Dokumen | Konteks |
|---|---|
| [Plan 1](plan.md) | Siklus awal bot trading dan testnet |
| [Plan 2](plan-2.md) | Platform managed/multi-instance dan riset paper |
| [Plan 3](plan-3.md) | Katalis/AI berbasis event dan backlog lama; tidak dilanjutkan otomatis |
| [Strategi](strategy.md) | Keputusan awal ETH momentum |
| [Strategi 2](strategy-2.md) | Penarikan trial dan protokol riset lama |
| [Strategi VPS](strategy-vps.md) | Profil ETH dan persiapan trial |

## Panduan teknis dan handover

| Dokumen | Konteks |
|---|---|
| [README lama](legacy-readme.md) | Quickstart, strategi dan perintah proyek lama; link disesuaikan dari root ke folder docs |
| [Handover agent lama](handover-agent-legacy.md) | Salinan handover sebelum pivot, termasuk invariant dan batas historis |
| [Handover pilot katalis](catalyst-pilot-handover.md) | Checkpoint kode/tes P3 dan pekerjaan tersisa |
| [Handover Spider](catalyst-spider-handover.md) | Riwayat collector, binary dan rilis komponen |
| [Runtime katalis](catalyst-runtime.md) | Kontrak/CLI P3; bukan quickstart Plan 4 |
| [Runtime managed](managed-runtime.md) | Konfigurasi, ledger, kontrol dan recovery lama |
| [Multi-instance](multi-instance.md) | Modal/ownership dan batas risiko lama |
| [Handover VPS](handover-vps.md) | Snapshot operasi; tidak diverifikasi ulang saat pivot |
| [Deployment](deployment.md) | Prosedur rilis lama; bukan izin deploy baru |

## Bukti dan reproduksi

[Indeks bukti historis](testing/readme.md) mempertahankan tautan ke laporan,
JSON, manifest dan ringkasan Plan 1–3. Hasil lama tidak membuktikan kelayakan
presale/DEX/CEX Plan 4. Jangan menulis ulang payload atau menjalankan ulang
script untuk mengganti hasil. Referensi path dalam payload tetap provenance
historis. Perintah reproduksi dalam dokumen tidak dijalankan oleh pivot ini.

## Kelanjutan yang tidak otomatis

Tidak ada migrasi modal, ledger, T0, database atau proses hanya karena pivot.
Tidak ada pemindahan seluruh backlog P3 ke roadmap baru. Gunakan kembali
komponen hanya setelah memeriksa kecocokan dan batasnya sesuai
[arsitektur aktif](presale-architecture.md). Status server aktual serta
kewajiban posisi/order lama harus diperiksa tersendiri bila ada tugas operasi
yang diizinkan; jangan menyimpulkannya dari pengarsipan docs.
