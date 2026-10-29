# Checkpoint lokal Plan 4

**29 Oktober 2026.** Riset P4-1, implementasi development P4-2 dan analisis/policy
P4-3 serta paper sintetis P4-4. Commit discovery **`108b3fa`**; perubahan
P4-3/P4-4 disertakan pada checkpoint commit berikutnya atas permintaan user
(lihat `git log` untuk hash terbaru). Tidak ada
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
- P4-3: schema/prompt, validator identitas/kutipan/lifecycle, policy dan helper
  sizing sesuai profil pilihan user, kuota durable WIB dan CLI analyze.
- Test source P4-3 mencakup override policy/size, missing/duplicate fields,
  quote palsu, expiry, consumed ID, locked exit/claim, HOLD/thesis, biaya/halt,
  dedup, kuota/error dan transport dengan schema kustom. Belum dijalankan.

## Pemeriksaan yang dilakukan

P4-4 menambahkan journal/adapters dan fixture data 18 langkah tiga jalur.
Data contoh dibentuk dengan serialisasi JSON/hash statis; bukan hasil eksekusi
simulator. Test source mencakup cash/cost/fee conservation, partial depth,
restart/idempotency, alokasi/vesting/refund, quote/min-out, timeout/expiry,
receipt gas gagal, finality/reorg dan journal fork. Semuanya belum dijalankan.

Review statis akhir P4-4: format Go dirapikan, diff whitespace bersih,
fixture JSON berlabel sintetis terbaca, 12 dokumen Markdown berubah/baru
tanpa tautan lokal/anchor putus. Payload bukti pengujian lama tidak berubah.

Review statis alur/callers dan format Go dengan gofmt; bukan kompilasi atau
kelulusan tes. Checkpoint P4-2 sebelumnya mencatat sepuluh dokumen dengan
tautan lokal/anchor valid. Lanjutan P4-3: diff whitespace bersih, JSON policy
terbaca dan sebelas dokumen Markdown berubah/baru tanpa tautan lokal atau
anchor putus. Dokumen dan
payload hasil Plan 1–3 tidak digunakan ulang sebagai bukti
strategi baru. Tidak ada perubahan remote Git atau VPS pada fase ini.

## Belum diverifikasi

- Build dan unit/contract tests baru maupun regresi repo; tidak dijalankan
  otomatis sesuai instruksi proyek. Wrapper transport gateway/kuota yang
  dipakai ulang juga memerlukan regression P3 sebelum rilis bersama.
- Crawl PinkSale nyata dengan binary pinned, robots dan HTML server-side;
  halaman yang membutuhkan JS dilaporkan sebagai gap, tidak dilewati.
- ABI selector/response dan registrasi pool terhadap RPC sebenarnya,
  finality/reorg, downtime/rate limit serta akses geografis Binance.
- Ketersediaan field risiko GoPlus, latensi/coverage/backfill, ukuran respons
  exchangeInfo dan kapasitas/retensi nyata.
- Kualitas model untuk prompt baru, dukungan schema/usage gateway, kuota dan
  penolakan adversarial pada inference nyata; policy P4-3 dipilih user tetapi
  kalkulasi serta test source belum diverifikasi lewat eksekusi.
- Eksekusi fixture dan recovery journal P4-4, API/receipt sumber nyata,
  bridge AI→paper serta kontrol/pilot Plan 4.

Perintah pengujian yang relevan kelak, **belum dijalankan**:

```sh
go test ./internal/launch ./cmd/launch
go test -race ./internal/catalyst ./internal/launch ./cmd/launch
```

Acceptance runtime P4-2–P4-4 tetap terbuka sampai ada izin/tugas verifikasi dan
bukti hasilnya. Keberadaan kode bukan kelulusan gate.
