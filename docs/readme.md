# Dokumentasi trading-bot

Folder ini untuk dibaca manusia. Semua trading masih **paper/virtual**; tidak ada izin uang asli.

| Ingin tahu | Baca |
|---|---|
| Rencana kerja dan status sekarang | [plan-4.md](plan-4.md) (Plan 3 ditutup: [plan-3.md](plan-3.md)) |
| Hasil riset strategi | [strategy-2.md](strategy-2.md) |
| Cara kerja, kontrol, deploy | [guide.md](guide.md) |

**Jangan baca `agents/` dan `testing/`.** Isinya catatan panjang untuk agent (handover, log, bukti
JSON, arsip). Tidak perlu dibaca manusia; semua yang penting sudah diringkas di tiga file di atas.

## Aturan dokumentasi

- Nama file/folder di `docs/` memakai kebab-case huruf kecil, tanpa tanggal.
- File di root `docs/` untuk manusia: pendek (±1 layar), status dan keputusan saja, tanpa log.
  Detail, riwayat, parameter dan bukti ditulis di `agents/` atau `testing/`.
- `agents/` netral antar-agent. Perbarui status terkini, hindari instruksi lama yang bertentangan,
  dan perbarui rujukan saat mengganti nama. Dokumen selesai masuk `agents/archive/`.
- Bukti JSON historis di `testing/` tidak ditulis ulang untuk merapikan narasi atau path lama.
- Ubah ringkasan di root `docs/` saat status penting berubah (pilot, keputusan, izin).
