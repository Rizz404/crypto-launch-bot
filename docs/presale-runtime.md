# Discovery, verifikasi dan analisis Plan 4

**Implementasi development 29 Oktober 2026.** Pada 10 November, `report`,
`replay-paper` dan satu pass nyata `collect -source cex|dex` dijalankan di WSL
dan lulus setelah perbaikan. Pada 12 November `collect -source presale|curve`
juga lulus dengan Spider build lokal; `analyze` belum.
[Pilihan integrasi](presale-integrations.md) dan [checkpoint](testing/plan-4-local.md#verifikasi-12-november-2026)
menjelaskan sumber serta batas bukti. Bukan runtime transaksi atau pilot.

## Pass terjadwal di server

Sejak 14 November, pass `collect` keempat jalur lalu `report` berjalan di
`plesk-newest` tiap 4 jam (00:07, 04:07, … UTC, ±5 menit), terpisah dari
`ana-core` dan `ana-scout`.

| Item | Lokasi |
|---|---|
| Direktori | `/opt/crypto-launch` (root, 0700): `repo/`, `image`, `reports/` |
| Image | `crypto-launch:sha-<commit>`, build lokal dari [Dockerfile.launch](../Dockerfile.launch); binary Spider diambil dari image `ana-scout` yang sudah ada di server |
| Compose | Project `crypto-launch` ([compose.launch.yml](../deploy/compose.launch.yml)), read-only, 0,5 CPU, 512 MB |
| Arsip | Volume `crypto-launch_archive` (`/var/lib/launch/research` di container) |
| Jadwal | `crypto-launch-collect.timer` → [launch-collect.sh](../deploy/launch-collect.sh) |
| Laporan | `/opt/crypto-launch/reports/latest.json`; disimpan 30 hari, arsip tidak dipangkas |

Pass ini tanpa inference: `.env.launch` server tidak dibuat, dan `analyze`
tetap aksi manual. Update ke commit baru:

```sh
cd /opt/crypto-launch/repo && git fetch -q origin main && git checkout -q --detach origin/main
rev=$(git rev-parse --short HEAD)
docker build -f Dockerfile.launch --build-arg SPIDER_IMAGE=ghcr.io/rizz404/ana-scout:sha-c1ce8569c15eaa1685ad2488a9293ac041411150 --build-arg SPIDER_PATH=/usr/local/bin/spider -t crypto-launch:sha-$rev .
echo crypto-launch:sha-$rev > /opt/crypto-launch/image
```

Checkout memakai deploy key read-only `ana-core` (`core.sshCommand` repo).

## Perintah

CLI `cmd/launch` terpisah dari `bot` dan `catalyst`. Build melalui target build
yang sudah ada kini juga menyertakan `bin/launch`. Sejak 14 November archive,
lock dan journal jalan di Linux maupun Windows (`internal/catalyst/fs_*.go`);
binary Spider pinned tetap build Linux/container. Belum ada
image/deployment/daemon khusus Plan 4. Perintah berikut referensi; arsip uji
manual sebaiknya diarahkan ke direktori sementara dengan `-archive`.

```sh
./bin/launch -action report
./bin/launch -action collect -source all -limit 5 -spider-binary /path/to/spider
./bin/launch -action collect -source dex -limit 5
./bin/launch -action collect -source cex -limit 5
./bin/launch -action collect -source presale -spider-binary /path/to/spider
./bin/launch -action collect -source curve -limit 5
./bin/launch -action analyze -candidate ID_DARI_REPORT -policy configs/launch.policy.json -env-file .env.launch
```

`report` default hanya membaca arsip, tanpa jaringan. `collect` melakukan satu
putaran terbatas dan menyimpan bukti API/website, hasil dan gap. Flag `-archive`
default `var/launch`, diabaikan Git, terpisah dari `var/catalyst` serta seluruh
ledger. `report/collect` tidak membaca env atau memanggil AI. `analyze` membaca
credential hanya bila dijalankan eksplisit dan dapat memakai kuota/biaya
gateway. Tidak ada Telegram, wallet, order atau claim/refund finansial.

Sumber `presale` membaca dua kanal Telegram sebagai petunjuk tautan lalu
men-scrape halaman detail PinkSale; `curve` membaca token four.meme terbaru
lewat RPC dan hanya menghasilkan kandidat riset (`held`). `-limit` dibatasi
1–10 per pass.

Binary Spider tidak ada di repo dan belum ada image yang di-publish dengan nama
baru, jadi tiap perangkat harus membangunnya sendiri sekali (Rust di container,
beberapa menit). Go yang dipakai 1.27.2; bila Go bawaan lebih lama, set
`GOTOOLCHAIN=go1.27.2`.

```sh
podman build -f Dockerfile.spider -t localhost/crypto-launch-bot-spider:local .
cid=$(podman create localhost/crypto-launch-bot-spider:local /spider)
podman cp "$cid:/spider" "$HOME/.local/state/crypto-launch-bot/lv/spider"
podman rm "$cid"
"$HOME/.local/state/crypto-launch-bot/lv/spider" --version
```

Versi yang benar: `spider_cli 2.53.9-p3.1`.

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
Periksa `TB_CATALYST_BASE_URL`/`TB_LAUNCH_BASE_URL` sebelum memakai kuota:
gateway pindah ke `route-gateway.nexlabs.cloud` sejak 30 Oktober dan host lama
gagal TLS. Request yang gagal di transport tetap memakai slot harian.

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

Perilaku sumber yang teramati 10 November:

- CEX memakai `exchangeInfo` dengan `symbolStatus=TRADING`; daftar tanpa
  filter melebihi batas respons 4 MB. Bila trade ID 0 sudah tidak tersimpan,
  trade tertua yang tersisa menjadi fakta `oldest_retained_trade_at`; lebih
  tua dari 30 hari berarti ditolak, tanpa mengklaim waktu listing.
- Node publik BSC memangkas state ±1–2 menit di belakang finalized. DEX dan
  presale mengambil ulang blok finalized bila snapshot berumur lebih dari
  15 detik sebelum membaca tiap pool/sale.
- Daftar `/launchpads` PinkSale dirender JavaScript dan tidak memuat tautan
  sale; halaman detail sale dirender server. Sejak 12 November tautan pool
  diambil dari dua kanal Telegram, jadi gap
  `no_parseable_bsc_sale_pages_or_javascript_required` hanya muncul bila kedua
  kanal tidak memberi satu pun tautan BSC yang terbaca.

## Pass berulang

Untuk mengukur coverage, pakai satu arsip yang bertahan antar-restart, di luar
repo, dan jalankan serial. `/tmp` di WSL terhapus saat restart. Simpan sebagai
file `.sh` (shell WSL bawaan fish) lalu jalankan dengan `sh`:

```sh
export GOTOOLCHAIN=go1.27.2 GOFLAGS=-buildvcs=false
D=$HOME/.local/state/crypto-launch-bot/lv
mkdir -p "$D"
go build -o "$D/launch" ./cmd/launch
"$D/launch" -action collect -source presale -spider-binary "$D/spider" -archive "$D/research"
"$D/launch" -action collect -source dex -limit 5 -archive "$D/research"
"$D/launch" -action collect -source cex -limit 5 -archive "$D/research"
"$D/launch" -action collect -source curve -limit 5 -archive "$D/research"
"$D/launch" -action report -archive "$D/research" > "$D/report.json"
```

`$D/spider` adalah binary hasil build di atas. Ringkasan per kandidat:

```sh
jq -r '.reviews[] | [.candidate.kind, .status, .candidate.external_id, .candidate.state, (.reasons|join(","))] | @tsv' "$D/report.json"
```

Arsip itu lokal per perangkat dan tidak ikut Git; di perangkat lain coverage
dimulai dari arsip kosong. Cursor pool DEX dan rotasi simbol CEX tersimpan di
arsip, jadi arsip baru mengulang lima simbol CEX pertama.

Gunakan satu collector serial per arsip. Lock penulisan arsip menjaga record,
bukan koordinasi kuota lintas proses; proses paralel dapat menggandakan request.
TTL/cooldown, batas request dan kapasitas ada di [kontrak integrasi](presale-integrations.md).
Policy P4-3 tersedia; report memperlihatkan analisis expired/superseded dan
seluruh entry tetap mati. P4-4 wajib memverifikasi ulang fresh sebelum transaksi.
