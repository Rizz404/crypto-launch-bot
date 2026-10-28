# Checkpoint development lokal Plan 3

## Lanjutan Spider 28 Oktober

Perubahan lokal: perbaikan assertion extractor, Spider default CLI dengan
`-spider-binary`, version check, batas lima halaman, cache versi backend, batas
JSON arsip simetris dan pembatalan proxy. `golang.org/x/net` menjadi direct
tanpa upgrade. Temuan lint WIP extractor/pesan error juga diperbaiki.

Pemeriksaan komponen terkait:

- `go test -race ./internal/catalyst ./cmd/catalyst ./scripts/catalyst-model-eval`: lulus.
- `go vet ./internal/catalyst ./cmd/catalyst ./scripts/catalyst-model-eval`: lulus.
- `golangci-lint run ./internal/catalyst/... ./cmd/catalyst/... ./scripts/catalyst-model-eval/...`: 0 issues.
- `gofmt -l internal/catalyst cmd/catalyst scripts/catalyst-model-eval`: tanpa output.

Fixture baru memeriksa JSON stream/strict schema/duplicate keys, urutan stabil,
seed/budget/status/header wajib, URL salah, ekspansi JSON melewati batas arsip,
round-trip record diterima, penolakan record tanpa menulis file, failed
observation menonaktifkan snapshot lama, batas stdout, binary hilang/salah versi,
argumen subprocess dan tidak diwariskannya credential/proxy environment induk.
Cache tetap bekerja setelah binary fixture dihapus; cache HTTP tidak memenuhi
versi Spider. CLI gagal sebelum jaringan/arsip jika binary tidak tersedia.

Fixture proxy memakai loopback/net.Pipe: host/port/method salah ditolak sebelum
dial, Hijacker tidak tersedia tidak panic, DNS/IP loopback ditolak, dial error
menjadi 502, buffered bytes dan balasan diteruskan, cancel menutup koneksi serta
handler. Ini pengujian proxy Go, bukan bukti Spider asli selalu memakainya.

Putaran pertama tes baru gagal karena Host request CONNECT fixture tidak diset;
fixture dikoreksi, putaran berikut lulus. Lint mula-mula menemukan switch
extractor dan kapitalisasi pesan error; diperbaiki sebelum hasil akhir di atas.
Failure parser 26 Oktober ditutup tanpa mengubah parser untuk menambah spasi.

Source rilis [CLI](https://crates.io/crates/spider_cli/2.53.9) dan
[Spider](https://crates.io/crates/spider/2.53.9) diunduh/dibaca, bukan diinstal.
SHA-256 crate CLI `bc25ae89df2472723dd93fe5b941a05c423c4b3741cfe7f2b7d4301b8f45e4b0`;
crate Spider `fd745f8618f4081c53e38ca89adb5ff02f19a9e1ef3dbc87095b8e5824354775`.
Cargo.lock CLI memuat Spider 2.53.9. `with_limit` mengganti budget wildcard;
karena itu `--budget *,5 --limit 1` draft ternyata hanya satu halaman.
Fixture protokol dibuat dari source, **bukan capture binary asli**.

Manifest Compose lulus `CATALYST_IMAGE=localhost/trading-bot-catalyst:dev podman compose -f deploy/compose.catalyst.yml config`;
ini tidak membuat/start stack Compose. Build lokal linux/amd64 berhasil dengan
`podman build -f Dockerfile.catalyst -t localhost/trading-bot-catalyst:dev .`.
Smoke versi Spider, `catalyst -h` dan default `report` lulus dengan
`--network none --read-only`. Image ID, perintah dan batas tersedia di
[bukti terstruktur](catalyst-spider-verification.json).

Tes binary rilis asli 2.53.9 **gagal**: server menerima robots/seed/artikel,
tetapi stdout hanya berisi seed; JSON halaman terakhir hilang. Patch
`build/spider-cli.patch` melakukan flush stdout setelah stream scrape dan
memberi versi `2.53.9-p3.1`. Source crate tetap checksum di atas dan lock
upstream. Collector mewajibkan versi patched; rilis tanpa patch ditolak.

`TestSpiderBinaryProtocol` pada image patched **lulus tiga kali**, setiap kali
robots dan dua halaman fixture diterima, diparse dan round-trip arsip lulus.
Perintah: `podman run --rm --network none --read-only --cpus 0.5 --memory 512m --pids-limit 128 --cap-drop all --security-opt no-new-privileges --tmpfs /tmp:size=16m,noexec,nosuid -v /tmp/catalyst-spider-tests:/test:ro -e TB_TEST_SPIDER_BINARY=/usr/local/bin/spider --entrypoint /test/catalyst.test localhost/trading-bot-catalyst:dev -test.run '^TestSpiderBinaryProtocol$' -test.v -test.count 3 -test.timeout 90s`.
Test executable dikompilasi dengan `CGO_ENABLED=0 go test -c ./internal/catalyst -o /tmp/catalyst-spider-tests/catalyst.test`.
Ini binary Spider asli dengan patch kecil, bukan fixture subprocess shell.
`--accept-invalid-certs` hanya pada invocation test karena sertifikat TLS
fixture self-signed; collector produksi tetap memvalidasi sertifikat. Guard
proxy fixture memetakan origin example.org ke server loopback terkendali;
tidak ada DNS publik, request situs publik atau order trading.

CI job image/protokol ditambahkan terpisah dari job publikasi P2, tanpa publish
image P3; belum ada run GitHub. Tidak ada tes penuh repo/integrasi DB, crawl
publik, evaluasi AI v4, worker/trial laptop, inspeksi/deploy VPS, pesan Telegram,
commit atau push pada checkpoint ini. Gate berikutnya: egress/redirect
adversarial, kontrak situs, retensi/quota dan input pasar/event; P3-1 belum
diterima penuh. Dua halaman fixture bukan benchmark kapasitas pilot.

## Checkpoint handover Spider 26 Oktober

Tes setelah penambahan extractor/metadata/prompt-v4:
`go test ./internal/catalyst ./cmd/catalyst ./scripts/catalyst-model-eval`
**gagal** pada `TestExtractFeedAndHTML`: expected `Activation date is TBA .`,
actual `Activation date is TBA.`. Sesudah draft `spider.go`/`spider_proxy.go`
belum ada format/build/tes/vet/lint baru. Hasil lulus di bagian berikut berlaku
untuk checkpoint sebelumnya. [Handover Spider](../catalyst-spider-handover.md)
mencatat failure dan temuan statis untuk ditutup.

Inspeksi VPS hanya baca, tanpa instalasi/crawl/deploy: [preflight](catalyst-vps-preflight.json).

## Evaluasi model dan pemeriksaan 26 Oktober

User mengisi key dan mengizinkan inference nyata untuk pemilihan model.
[Evaluasi model](catalyst-model-evaluation.md) dan
[JSON teredaksi](catalyst-model-evaluation.json) mencatat tiga putaran, tujuh
kasus sintetis per model. Env memilih Luna; combo/failover belum diuji.
Pada checkpoint evaluasi ini prompt `catalyst-v3`, memakai strict JSON Schema;
kode WIP terbaru v4 belum dievaluasi ulang.

Pemeriksaan sesudah perubahan env/prompt/schema/evaluator:

- `go test -race ./internal/catalyst ./internal/config ./cmd/catalyst ./scripts/catalyst-model-eval`: lulus.
- `go vet ./internal/catalyst ./internal/config ./cmd/catalyst ./scripts/catalyst-model-eval`: lulus.
- `golangci-lint run ./internal/catalyst/... ./internal/config/... ./cmd/catalyst/... ./scripts/catalyst-model-eval/...`: 0 issues.

Tidak menjalankan worker, order, deployment atau mengubah P2. Build penuh,
DB integration, sumber berita nyata dan shadow prospektif tidak diuji pada sesi ini.
Status key/model kosong dan tes belum berjalan di bagian berikut adalah checkpoint
setup awal yang telah digantikan hasil di atas.

## Pembaruan env OmniRoute 26 Oktober

File `.env.catalyst` lokal disiapkan 0600 dan diabaikan Git, key/model kosong,
base URL HTTPS `https://route-gateway.fbe.co.id/v1`. Pemeriksaan HTTPS tanpa
key ke `/v1/models` mendapat 401. Pemeriksaan statis memastikan file privat
diabaikan Git, template boleh dilacak dan tidak ada key pada template.

CLI menambahkan `-env-file` untuk `analyze`, memakai parser dotenv yang sudah
ada (diekspor sebagai `config.ReadDotEnv` tanpa perubahan perilaku parser).
Test konstruksi adapter dari file, prioritas environment proses dan override
key kosong ditambahkan tanpa HTTP request. **Tes/build/runtime perubahan env
ini belum dijalankan**; hasil lulus 23 Oktober di bawah tetap bukti historis.
Tidak ada inference atau perubahan layanan P2.

## Checkpoint awal 23 Oktober

23 Oktober 2026. Baseline dokumentasi di-commit sebagai `bc321b8`.
Development komponen baru belum di-commit pada checkpoint ini. User meminta
koneksi/model AI ditunda untuk fokus development. [Plan 3](../plan-3.md) dan
[kontrak runtime](../catalyst-runtime.md) mencatat cakupan serta pekerjaan sisa.

## Hasil yang dijalankan

| Pemeriksaan | Hasil |
|---|---|
| `go test -race ./...` | Lulus; termasuk paket lama/P2, komponen catalyst dan CLI baru |
| `go test -race ./internal/catalyst ./cmd/catalyst` | Lulus sesudah tambahan kasus hold setelah acara dan pending expiry |
| `go vet -tags integration ./...` | Lulus analisis statis; bukan menjalankan tes DB integration |
| `golangci-lint run` | 0 issues |
| `make build` | Lima binary lokal, termasuk catalyst; tidak menjalankan bot |

Seluruh pengujian catalyst memakai transport/inference dan data pasar fixture.
Replay menggunakan MemoryStore dan inbox sementara, tanpa koneksi PostgreSQL,
exchange atau provider model. Hasil bukan performa perdagangan atau kelulusan
trial prospektif. Suite lama dapat memakai hasil cache Go jika tidak berubah.

## Bukti perilaku yang diperiksa

- JSON field asing, key duplikat, trailing content, nilai invalid dan payload
  berlebih ditolak. Policy membatasi symbol/source dan URL.
- Kutipan palsu, sumber nonprimer, tanggal tanpa bukti, observasi masa depan,
  simbol salah dan tesis tidak lengkap ditolak untuk entry/HOLD.
- Arsip menerima penulisan konkuren tanpa overwrite dan mendeteksi checksum
  yang berubah. Versi sumber dan reversion mempertahankan first-seen.
- Fetch gagal tidak memakai snapshot sukses lama sebagai data terbaru;
  polling minimum/cache menahan request ulang. Robots dan IP nonpublik diuji.
- Request model tidak memiliki tools; JSON hasil dan served-model dicatat.
  Input sama dalam satu jendela ditahan sebelum request kedua; respons
  terpotong/tool call/HTTP error ditolak. Secret fixture tidak muncul di arsip
  error/model report. Tidak ada kredensial nyata di fixture.
- Entry gagal untuk attestation kosong/dicabut, quote basi/masa depan,
  spread lebar, book sisi jual tipis, halt/exposure penuh, venue/policy salah
  atau keputusan kedaluwarsa. Sizing mencakup fee/slippage dalam cap.
- Cap memecoin mencakup biaya dan menolak sleeve yang sudah penuh.
- Managed/Sim membuka posisi, menaikkan trailing mengikuti harga, memulihkan
  checkpoint pada runtime baru dan menjual seluruh posisi ketika trailing
  tersentuh. Order akhir terminal; dana reserved nol dan event tidak dibeli ulang.
- Pencabutan event dan review kedaluwarsa menyebabkan full exit. HOLD sesudah
  acara dengan bukti primer baru memperpanjang review; bukti lama tidak cukup.
- Entry pending yang kedaluwarsa dibatalkan sebelum fill fixture berikutnya.
- `NewPaper` menolak wallet beridentitas P2 tanpa mengubah state sumber.
- CLI default report tidak membuat arsip, membuka DB atau mengakses jaringan;
  analyze tanpa konfigurasi dan replay tanpa fixture gagal dengan jelas.

Perbaikan selama development: sizing awal belum menyisakan fee pada cap
binding sehingga ledger menolak entry. Cap kini dihitung setelah buffer biaya;
regression entry/memecoin memakai jumlah yang benar-benar dapat direservasi.

## Belum dibuktikan

- URL/konten/robots sumber publik, JavaScript atau extraction seluruh artikel.
- Koneksi 9router, model/combo, kualitas model nyata, tarif dollar dan shadow
  prospective; urusan AI nyata ditunda user.
- Registry attestation tepercaya, pencabutan event dari sumber dan normalisasi
  tanggal/konflik. Flag verified pada fixture bukan verifikasi otomatis.
- Adapter ATR14 15m, book/depth/anchor nyata serta model fill order book.
- PostgreSQL pilot, migrasi/lease/restart proses pilot, storage outage, partial
  fill/gagal exit/dust pada integrasi baru. Regression lama tetap bukti asal,
  bukan bukti seluruh integrasi catalyst sudah memenuhi kontrak tersebut.
- Dashboard/Telegram/metrics AI, worker/scheduler, disk quota/retensi, workload
  VPS, container smoke/cross-build CI dan deployment.

Tidak ada push/deploy, perubahan layanan/config/ledger P2, trial laptop atau
transaksi uang asli. Inspeksi VPS sesi ini hanya membaca daftar container
untuk mencari layanan 9router; tidak memeriksa ulang kebenaran ledger P2.
