# Checkpoint lokal Plan 4

**29 Oktober 2026.** Riset P4-1 dan implementasi development P4-2. Tidak ada
uji fungsi, build, crawl CLI, koneksi RPC/API melalui aplikasi, inference,
Telegram, database, server, deploy atau transaksi yang dijalankan.

## Yang tersedia

- Riset referensi resmi dan matriks pilihan:
  [integrasi awal](../presale-integrations.md).
- CLI discovery/report terpisah dengan reuse archive serta pipeline Spider.
- Kandidat presale/pool/pair terpisah, identitas chain/kontrak atau venue,
  receipt time/revisi, bukti asli dan gap sumber.
- Verifier menahan data basi/unknown/konflik dan tidak pernah mengizinkan entry.
- Test source ditulis untuk batas 30 hari, identitas, jadwal presale/testnet,
  source gap, book, arsip rusak, trade ID 0, cursor/backlog pool, RPC error dan
  batas trust scanner.

## Pemeriksaan yang dilakukan

Review statis alur/callers dan format Go dengan gofmt; bukan kompilasi atau
kelulusan tes. Pemeriksaan diff whitespace bersih; sepuluh dokumen Markdown
yang berubah/baru tidak memiliki tautan lokal atau anchor putus. Dokumen dan
payload hasil Plan 1–3 tidak digunakan ulang sebagai bukti
strategi baru. Tidak ada perubahan remote Git atau VPS pada fase ini.

## Belum diverifikasi

- Build dan unit/contract tests baru maupun regresi repo; tidak dijalankan
  otomatis sesuai instruksi proyek.
- Crawl PinkSale nyata dengan binary pinned, robots dan HTML server-side;
  halaman yang membutuhkan JS dilaporkan sebagai gap, tidak dilewati.
- ABI selector/response dan registrasi pool terhadap RPC sebenarnya,
  finality/reorg, downtime/rate limit serta akses geografis Binance.
- Ketersediaan field risiko GoPlus, latensi/coverage/backfill, ukuran respons
  exchangeInfo dan kapasitas/retensi nyata.
- Seluruh AI, policy uang, simulasi/eksekusi, kontrol dan pilot Plan 4.

Perintah pengujian yang relevan kelak, **belum dijalankan**:

```sh
go test ./internal/launch ./cmd/launch
go test -race ./internal/catalyst ./internal/launch ./cmd/launch
```

Acceptance runtime P4-2 tetap terbuka sampai ada izin/tugas verifikasi dan
bukti hasilnya. Keberadaan kode bukan kelulusan gate.
