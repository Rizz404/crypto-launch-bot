# Arsitektur presale dan listing baru

Rancangan [Plan 4](plan-4.md), 29 Oktober 2026. Kontrak di bawah adalah target
desain, belum schema Go/DB, API HTTP, CLI atau konfigurasi yang tersedia.

## Alur dan batas tanggung jawab

Spider untuk website serta adapter API/feed/on-chain → arsip bukti dan
deduplikasi → registry aset/sale/listing → verifier → snapshot pasar/on-chain
→ OmniRoute/AI → validator policy → intent eksekusi → adapter presale/DEX/CEX
→ rekonsiliasi/ledger → monitoring dan laporan.

Collector hanya membaca data. Analisis tidak memiliki akses signer/API trading.
Eksekutor menerima intent tervalidasi, memeriksa ulang kondisi sebelum I/O dan
menyimpan status eksternal. Loop proteksi dan rekonsiliasi tidak menunggu AI.
Konten eksternal dianggap data tak tepercaya; instruksi dalam halaman atau
hasil model tidak boleh mengubah konfigurasi maupun memberi akses tool.

## Kontrak data minimum

| Kontrak | Isi dan pemisahan yang diperlukan |
|---|---|
| Identitas aset | ID internal, chain/network dan kontrak/mint bila tersedia, mapping venue/simbol beserta bukti; ticker hanya label |
| Kandidat | Aset atau identitas sementara, jenis presale/listing, ID sale/venue/pool, jadwal, umur token/listing, first-seen dan status lifecycle |
| Bukti | ID/checksum, URL/provider, waktu publikasi bila tersedia, fetched/first-seen, revisi, konten asli dan gap pengambilan |
| Hasil verifikasi | Referensi kandidat/bukti, fakta yang terkonfirmasi, konflik, unknown, waktu pemeriksaan dan masa berlaku |
| Snapshot | Harga, book/quote/rute, likuiditas dan data on-chain dengan waktu observasi, network/block serta status finality bila relevan |
| Keputusan AI | Aksi kontekstual, target kandidat, tesis/kontra-argumen/invalidation, kutipan, snapshot/policy version, expiry dan hasil validasi |
| Intent dan hasil eksekusi | ID idempoten, aksi/jalur, jumlah/biaya maksimum, reservasi, order ID/tx hash, status submit/unknown/confirmed/failed serta settlement |

Gunakan waktu UTC untuk fakta/jadwal tersimpan dan WIB untuk tampilan; hari
akuntansi/kuota ditetapkan eksplisit saat policy dibekukan. Unknown tetap
unknown. Identitas sementara boleh untuk riset, tetapi entry memerlukan
identitas terverifikasi sesuai adapter. Jadwal listing bukan waktu fetch.

ID sale/listing berbeda dari ID aset dan ID berita. Satu berita yang disalin
tidak menghasilkan intent ganda; listing aset yang sama di venue berbeda
tetap event berbeda. Semua keputusan mengacu versi bukti/policy tertentu.
Schema final, precision/unit, TTL dan finality dipilih setelah P4-1.

## Kode saat ini dan reuse

Inspeksi statis checkout 29 Oktober, tanpa tes/runtime baru:

| Komponen yang ada | Batas dan pekerjaan Plan 4 |
|---|---|
| Go, collector Spider, ekstraksi/cache/arsip dan proxy dalam `internal/catalyst` | Kandidat reuse ingest; cakupan sumber baru, kapasitas dan freshness belum terbukti |
| Adapter OmniRoute, strict schema dan validator | Schema saat ini memakai symbol/event; perlu identitas lintas-chain, lifecycle dan evaluasi prompt baru |
| Policy catalyst dan input pasar Binance | Policy memakai daftar simbol USDT; bukan discovery dinamis atau registry sale/pool |
| Managed ledger, reservasi/checkpoint dan broker Sim | Kandidat reuse pencatatan; fill candle/book tidak memodelkan alokasi/vesting, AMM/gas/finality |
| Runner paper, dashboard/health dan Telegram | Ada kode P3, bukan penerimaan operasional Plan 4; UI/laporan perlu dana terkunci dan hasil per jalur |
| Presale/DEX execution dan signer | Belum ada integrasi Plan 4; perlu adapter, batas kewenangan dan rekonsiliasi baru |

Panduan kode lama ada di [arsip](archive.md). Dokumen checkpoint P3 bukan bukti
checkout sekarang sudah diuji atau dirilis untuk Plan 4.

## Durability dan isolasi

Simpan intent/reservasi sebelum submit. Timeout sesudah submit menghasilkan
status unknown yang direkonsiliasi lewat order ID/tx hash, bukan retry buta.
Catat kontribusi, alokasi, claimable, terkunci/vesting dan inventori tradable
secara terpisah; perpindahan antarstatus tidak menciptakan modal baru.

Plan 4 kelak memakai konfigurasi, DB/ledger, wallet dan budget terisolasi dari
eksperimen lama. Perubahan shared code membutuhkan regression relevan sebelum
rilis, tanpa otomatis deploy ulang P2/P3. Rancangan signer tidak menyimpan key
di prompt, arsip bukti, source atau log. Tidak ada perubahan runtime/DB pada
tahap dokumentasi ini.
