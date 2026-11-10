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
| [Plan 1](agents/archive/plan.md) | Siklus awal bot trading dan testnet |
| [Plan 2](agents/archive/plan-2.md) | Platform managed/multi-instance dan riset paper |
| [Plan 3](plan-3.md) / [detail](agents/plan-3.md) | Katalis/AI berbasis event; ditutup 1 November ([review akhir](testing/final-review.md)) |
| [Strategi](agents/archive/strategy.md) | Keputusan awal ETH momentum |
| [Strategi 2](strategy-2.md) / [detail](agents/strategy-2.md) | Ringkasan dan protokol riset lama |
| [Strategi VPS](agents/archive/strategy-vps.md) | Profil ETH dan persiapan trial |

## Panduan teknis dan handover

| Dokumen | Konteks |
|---|---|
| [README lama](legacy-readme.md) | Quickstart, strategi dan perintah proyek lama; link disesuaikan dari root ke folder docs |
| [Handover agent lama](handover-agent-legacy.md) | Salinan handover sebelum pivot, termasuk invariant dan batas historis |
| [Handover pilot katalis](agents/catalyst-pilot-handover.md) | Checkpoint kode/tes P3 dan pekerjaan tersisa |
| [Handover Spider](agents/catalyst-spider-handover.md) | Riwayat collector, binary dan rilis komponen |
| [Runtime katalis](agents/catalyst-runtime.md) | Kontrak/CLI P3; bukan quickstart Plan 4 |
| [Runtime managed](agents/managed-runtime.md) | Konfigurasi, ledger, kontrol dan recovery lama |
| [Multi-instance](agents/multi-instance.md) | Modal/ownership dan batas risiko lama |
| [Handover VPS](agents/handover-vps.md) | Snapshot operasi; tidak diverifikasi ulang saat pivot |
| [Deployment](agents/deployment.md) | Prosedur rilis lama; bukan izin deploy baru |
| [Handover trading-bot](agents/handover-agent.md) / [indeks agent](agents/readme.md) | Handover repo Rizz404/trading-bot sampai 8 November, diimpor lewat merge |
| [Panduan web/kontrol](guide.md) | Cara kerja wallet/instance/binding dan kontrol dashboard trading-bot |

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
