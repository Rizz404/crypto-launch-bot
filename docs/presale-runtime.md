# Discovery, verifikasi dan analisis Plan 4

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
./bin/launch -action analyze -candidate ID_DARI_REPORT -policy configs/launch.policy.json -env-file .env.launch
```

`report` default hanya membaca arsip, tanpa jaringan. `collect` melakukan satu
putaran terbatas dan menyimpan bukti API/website, hasil dan gap. Flag `-archive`
default `var/launch`, diabaikan Git, terpisah dari `var/catalyst` serta seluruh
ledger. `report/collect` tidak membaca env atau memanggil AI. `analyze` membaca
credential hanya bila dijalankan eksplisit dan dapat memakai kuota/biaya
gateway. Tidak ada Telegram, wallet, order atau claim/refund finansial.

## Analisis AI eksplisit

[Policy/schema P4-3](presale-analysis.md) membekukan profil yang disepakati user.
Template credential aman: [.env.launch.example](../.env.launch.example);
file `.env.launch` terisi diabaikan Git. `TB_LAUNCH_BASE_URL`, `TB_LAUNCH_MODEL`
dan `TB_LAUNCH_API_KEY` dari environment proses mengungguli key yang sama
dalam file. Jangan menulis key ke config JSON atau membagikannya di chat.

Untuk memakai file gateway lama tanpa menyalin key, flag eksplisit
`-env-file .env.catalyst` mendukung `TB_CATALYST_BASE_URL/MODEL/API_KEY`
sebagai fallback; namespace `TB_LAUNCH_*` yang tersedia diprioritaskan.
Tidak ada fallback pembacaan file lama otomatis dan token Telegram tidak dibaca.

Tanpa `-context`, analisis memakai konteks awal virtual 1.000 USDT tanpa posisi,
bukan mengambil saldo/ledger lama. `-context /path/paper-context.json` menerima
snapshot lengkap/fresh termasuk `day_wib`, equity, day-start, peak, kas tersedia,
positions dan consumed IDs. File parsial tidak diisi diam-diam dari saldo awal.
Konteks ini untuk riset; integrasi ledger otoritatif tetap P4-4.

Kandidat harus sudah ada di arsip dan policy harus cocok versi yang disetujui.
Command tidak collect otomatis. Sebelum/sesudah inference, arsip dan status
kandidat diperiksa; respons malformed, kutipan palsu, tool call, identitas
salah, override risiko dan aksi lifecycle yang tidak tersedia ditolak.

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
- `launch-ai-inputs`/`launch-ai-reviews`: packet persis yang dikirim dan hasil
  model/usage/validasi, tanpa headers/key. Marker dedup dan `daily-budget`
  bertahan setelah restart; gagal tidak mengembalikan slot.
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

## Replay paper terisolasi

P4-4 menambahkan aksi offline `replay-paper` dan `paper-report` dengan journal
default `var/launch-paper`, berbeda dari arsip riset. Keduanya tidak membaca
credential atau memanggil AI/jaringan. Fixture harus menyatakan sintetis dan
policy yang cocok; referensi transaksi/proof diberi prefix `fixture:`.
[Kontrak serta contoh](presale-paper.md) menjelaskan batas simulasinya.
Hasil fixture tidak menjadi saldo pilot dan tidak dipakai otomatis sebagai
context oleh `analyze`; ledger lama juga tidak diimpor.

## Batas integrasi

Scanner BSC belum mengikat semua state/ABI sale ke factory launchpad,
memetakan holder terkait, membuktikan kemampuan menjual, atau menentukan policy
likuiditas/exit. Karena itu DEX/presale tetap held kecuali ada alasan reject.
Mapping kontrak token Binance juga belum dibuat; ID venue tidak menjadi
klaim identitas lintas-chain. Cursor DEX mengejar pool baru sesudah bootstrap;
history sebelum bootstrap dan refresh semua kandidat lama belum dicakup.

Gunakan satu collector serial per arsip. Lock penulisan arsip menjaga record,
bukan koordinasi kuota lintas proses; proses paralel dapat menggandakan request.
TTL/cooldown, batas request dan kapasitas ada di [kontrak integrasi](presale-integrations.md).
Policy P4-3 tersedia; report memperlihatkan analisis expired/superseded dan
seluruh entry tetap mati. P4-4 wajib memverifikasi ulang fresh sebelum transaksi.
