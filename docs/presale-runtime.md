# Discovery dan verifikasi Plan 4

**Implementasi development 29 Oktober 2026; belum diuji/dijalankan.**
[Pilihan integrasi](presale-integrations.md) dan [checkpoint](testing/plan-4-local.md)
menjelaskan sumber serta batas bukti. Bukan runtime transaksi atau pilot.

## Perintah

CLI `cmd/launch` terpisah dari `bot` dan `catalyst`. Build melalui target build
yang sudah ada kini juga menyertakan `bin/launch`. Jalankan pada Linux/WSL atau
container yang sesuai; archive reuse masih memakai syscall Unix. Belum ada
image/deployment/daemon khusus Plan 4. Perintah berikut referensi, tidak
dijalankan dalam pekerjaan ini.

```sh
./bin/launch -action report
./bin/launch -action collect -source all -limit 5 -spider-binary /path/to/spider
./bin/launch -action collect -source dex -limit 5
./bin/launch -action collect -source cex -limit 5
./bin/launch -action collect -source presale -spider-binary /path/to/spider
```

`report` default hanya membaca arsip, tanpa jaringan. `collect` melakukan satu
putaran terbatas dan menyimpan bukti API/website, hasil dan gap. Flag `-archive`
default `var/launch`, diabaikan Git, terpisah dari `var/catalyst` serta seluruh
ledger. Tidak ada pembacaan env privat, setup Telegram/AI/wallet, order, claim
atau aksi trading. Tindakan di luar `collect`/`report` ditolak.

## Data dan laporan

- `launch-raw`: URL, waktu pengambilan, request RPC bila ada, HTTP status,
  respons mentah dan error tersanitasi. Response non-200 tetap disimpan;
  tidak ada retry otomatis untuk rate limit atau kegagalan.
- `launch-web`: hasil Spider asli dan dokumen ekstraksi, memakai robots,
  proxy publik, batas ukuran/halaman dan versi pinned dari collector lama.
  Binary hilang/gagal tidak memicu fallback HTTP.
- `launch-batches`: observasi per sumber, kandidat, referensi bukti, gap
  serta cursor pool/simbol CEX untuk putaran berikutnya. Revisi lama
  immutable, checksum diverifikasi saat dibaca.
- Report menghitung first-seen dari history, memilih observasi terbaru,
  mempertahankan revisi dan menahan konflik identitas/jadwal ambigu. Gap
  pengambilan terbaru menahan penilaian lama; bukti basi tidak tampak fresh.

Satu aset dapat punya sale/pool/pair berbeda. Tidak ada penggabungan berdasarkan
ticker dan tidak ada first-seen bot yang dijadikan tanggal listing. Umur token
global belum diverifikasi oleh v1. Provider pool creation bukan trade pertama.

`accepted_for_research` berarti input riset lolos pemeriksaan tersedia,
`held` berarti bukti/gate belum cukup, dan `rejected` berarti alasan penolakan
seperti umur listing melewati batas atau indikator honeypot. Laporan selalu
`research_only=true` dan `entry_allowed=false`, termasuk hasil accepted.
Scope parsial dan unknown sengaja terlihat dalam reasons/sources.

## Batas

Scanner BSC belum mengikat semua state/ABI sale ke factory launchpad,
memetakan holder terkait, membuktikan kemampuan menjual, atau menentukan policy
likuiditas/exit. Karena itu DEX/presale tetap held kecuali ada alasan reject.
Mapping kontrak token Binance juga belum dibuat; ID venue tidak menjadi
klaim identitas lintas-chain. Cursor DEX mengejar pool baru sesudah bootstrap;
history sebelum bootstrap dan refresh semua kandidat lama belum dicakup.

Gunakan satu collector serial per arsip. Lock penulisan arsip menjaga record,
bukan koordinasi kuota lintas proses; proses paralel dapat menggandakan request.
TTL/cooldown, batas request dan kapasitas ada di [kontrak integrasi](presale-integrations.md).
P4-3/P4-4 harus menambahkan policy dan verifikasi ulang fresh tepat sebelum
transaksi; report ini tidak boleh digunakan sebagai izin entry.
