# Simulasi dan adapter paper — P4-4

**29 Oktober 2026. Implementasi development, belum diuji/build/dijalankan.**
Mode yang tersedia adalah replay sintetis offline; tidak memiliki signer,
client order exchange, RPC submit, wallet atau jalur uang asli. Penerimaan
runtime P4-2/P4-3 tetap terbuka. Tidak memulai pilot.

## Buku bersama dan pemisahan data

Satu run memakai [policy paper yang disepakati](presale-analysis.md) dan
1.000 USDT virtual bersama ketiga jalur. Journal default `var/launch-paper`,
terpisah dari arsip riset `var/launch`, ledger P2/P3 dan seluruh database lama.
Mengganti run ID pada journal yang sudah terisi ditolak, bukan mengisi modal baru.

Setiap request memiliki ID dan hash payload. Journal menyimpan urutan,
hash record sebelumnya, policy hash dan clock fixture. Penulisan memegang
lock proses terpisah, mengulang state dari record immutable, memvalidasi
transisi serta konservasi dana, lalu append atomik melalui archive lama.
Gap/fork/checksum atau ID yang dipakai ulang dengan payload berbeda menahan
operasi. Tidak ada perubahan finansial persisten bila transisi gagal.

Kas tersedia, reservasi, cost basis inventori, principal terkunci, fees dan
realized gross dipisahkan. Invariant setiap transisi:

```text
kas + reservasi + cost basis inventori + principal terkunci + fees
= modal awal + realized gross
```

Profit terealisasi tidak berasal dari claim, vesting atau harga indikatif.
`Context` terpisah menghitung equity konservatif dari mark yang fresh; inventori
tanpa harga dan principal terkunci diberi nilai konservatif nol, ditandai
unknown, dan menahan exposure baru. Cost basis tetap ada untuk akuntansi.
Ini bukan klaim bahwa token tersebut pasti tidak bernilai.

`PaperExposure.mark_usdt/mark_known` memisahkan valuasi dari nominal risiko.
Data context lama tanpa mark masih dapat dibaca sebagai input riset, tetapi
tidak meloloskan exposure baru. Partial claim dapat menghasilkan bagian
tradable saat sisanya masih vesting; hanya quantity yang sudah diterima dan
tidak terreservasi yang dapat dijual. Validator P4-3 disesuaikan untuk itu.

## Adapter sintetis

| Jalur | Perilaku |
|---|---|
| CEX | Reuse `broker.Sim.MatchBook`: buy di ask/sell di bid, limit price, lot step, minimum notional dan maksimal 10% best-level depth. Konsumsi depth dicatat per snapshot/side agar quote sama tidak dipakai berulang. Partial fill/fee settle dari delta receipt kumulatif |
| DEX | Membutuhkan route quote eksplisit: aset, pool/kandidat, waktu/expiry, input/output, min-out, biaya/gas, likuiditas dan kemampuan menjual yang diasumsikan dalam fixture. Quote dikunci pada broadcast. Settlement menunggu receipt final; quote yang kedaluwarsa sesudah broadcast tidak membatalkan transaksi yang sudah pending |
| Presale | Kontribusi membuat principal terkunci, tanpa token tradable. Allocation/finalized proof memberi jumlah token, total unlock dan hak refund. Refund mengurangi principal, claim memindahkan bagian cost basis ke inventori sesuai proporsi; vesting berikutnya tidak menciptakan modal baru |

Semua input ekonomi dinormalisasi ke USDT secara **eksplisit dalam fixture**.
Tidak ada konversi BNB/token/gas atau komisi base-asset otomatis. Bila data
FX, fee, alokasi, route atau finality nyata belum tersedia, adapter prospektif
tidak boleh mengisi asumsi fixture sebagai fakta.

## Intent, receipt dan recovery

- `submit` memeriksa identitas, versi/revisi kandidat, umur/jadwal, policy,
  kas/exposure/fee ceiling, dedup kandidat, serta tesis. Reservasi dicatat
  sebelum simulasi broadcast atau match. Expiry maksimum 300 detik.
- `broadcast` hanya merekam referensi `fixture:` dan route bila DEX. Tidak
  mengirim transaksi. `unknown` menahan reservasi dan exposure baru; tidak
  retry buta, melepas dana karena timeout, atau mengganti transaction ID.
- Receipt adalah counter kumulatif untuk satu order. Event ID yang sama
  tidak dibukukan ulang; counter regressed, inventori berlebih, salah ID,
  fee/min-out/price limit yang dilanggar ditolak.
- Pending/confirmed on-chain belum memindahkan principal atau token. Receipt
  final dengan block proof fixture menyelesaikan sekali. Transaksi on-chain
  tidak diperlakukan sebagai partial order CEX; gas gagal tetap dibayar.
- Reorg sebelum final mengembalikan status unknown tanpa membukukan hasil.
  Reorg setelah settlement final menyalakan `attention`, menahan entry baru
  dan mempertahankan history sampai audit; tidak membalikkan dana sembarangan.
- Expiry menghentikan matching lokal, tetapi dana menunggu ack terminal.
  CEX late acknowledgment dapat menyelesaikan fill terdahulu. Restart/replay
  memakai journal yang sama, bukan saldo atau counter baru.
- Alokasi/unlock tidak boleh mundur dari hasil yang sudah settled. Refund
  excess harus selesai sebelum claim pertama; perubahan alokasi sesudah claim
  dan refund sesudah claim belum didukung, sehingga ditahan untuk rekonsiliasi.
- `mark` dengan harga fresh dapat menandai `exit_required` saat menyentuh stop.
  Pemantau otomatis dan pengiriman exit independen AI tetap P4-5; flag ini
  menahan entry baru dan bukan bukti stop sudah terisi.

Fee budget per posisi berasal dari ceiling biaya entry yang diketahui; aksi
claim/refund/exit berikutnya memakai sisa budget itu. Biaya yang naik di atas
ceiling perlu review, tidak otomatis mengambil modal tambahan. Satu posisi
hanya memiliki satu aksi outstanding, dan position ID tidak dipakai ulang.

## Perintah dan fixture contoh

Referensi berikut **belum dijalankan**. Runtime memakai Linux/WSL/container
karena archive dan lock memakai syscall Unix.

```sh
./bin/launch -action replay-paper -fixture configs/launch.paper.fixture.json -paper-archive var/launch-paper
./bin/launch -action paper-report -run-id example-three-paths -paper-archive var/launch-paper
```

[Fixture contoh](../configs/launch.paper.fixture.json) mempunyai 18 langkah
sintetis: CEX buy/sell, DEX swap/exit, lalu presale batal dan refund. Label
`fixture_only=true` wajib. Kandidat, kutipan pasar, fees dan proof semuanya
buatan; harga/perhitungan tidak mewakili peluang atau hasil pasar nyata.
Test source mencantumkan saldo yang diharapkan dari data itu, bukan hasil
pengujian yang sudah lulus. Menjalankan ulang tape yang sama seharusnya
idempoten; test ini belum dieksekusi.

Fixture memerlukan versi, run ID, policy lengkap dan steps. Operasi yang
diizinkan: submit, broadcast, unknown, match, receipt, allocation dan mark.
Payload yang tidak sesuai operasi ditolak. Maksimal 1.000 langkah/16 MiB
input; journal tetap tunduk kuota 256 MiB dan batas record archive.

`assumed_verified=true` hanya menyatakan asumsi skenario sintetis. Flag itu
tidak berasal dari AI atau collector dan tidak meningkatkan status held
P4-2. Tidak ada bridge otomatis AI → order, quote provider live, ABI transaksi
PinkSale, atau signer dalam implementasi ini. Wire adapter sumber nyata dan
penerimaannya harus diselesaikan sebelum pilot; dukungan live tetap P4-6.

Replay journal penuh saat setiap transisi sesuai fixture terbatas, belum
dibuktikan untuk daemon panjang. Tidak ada compaction, migrasi DB, perubahan
VPS, push atau otomatisasi Telegram pada fase ini. User kemudian meminta
commit checkpoint P4-3/P4-4; hal itu tidak mengubah status tes/runtime di atas.
