# Checkpoint pengujian WIP pilot katalis

**28 Oktober 2026, handover sekitar 22.12 WIB.** Ini hasil beberapa checkpoint
lokal saat implementasi, **bukan penerimaan checkout final, CI rilis atau start
pilot**. [Handover utama](../catalyst-pilot-handover.md) memuat file, izin,
keputusan user dan pekerjaan lanjut. Kode baru masih modified/untracked.

## Pemeriksaan yang benar-benar dijalankan

| Perintah/area | Hasil dan cakupan |
|---|---|
| `go test -race ./internal/catalyst ./internal/exchange/binance` | Lulus setelah adapter market/ATR ditambahkan |
| `go test -race ./internal/catalyst ./cmd/catalyst ./scripts/catalyst-model-eval` | Lulus setelah event/date, budget, partial evidence awal; bukan model nyata v5 |
| `go test -race ./internal/broker ./internal/managed ./internal/catalyst ./cmd/catalyst ./scripts/catalyst-model-eval` | Lulus pada checkpoint book fills/quota; mencakup spread, depth bersama, lot rounding, partial/restart dan gap stop |
| `go test -race ./internal/catalyst ./cmd/catalyst ./internal/managed ./internal/broker ./internal/notify/telegram ./internal/exchange/binance` | Lulus pada checkpoint menengah; sesudahnya ada perubahan consumed events, HOLD, outcome, alias receipt dan test sender yang belum diuji ulang bersama |
| `go test -race ./internal/catalyst -run 'TestPilot' -count=1` | Lulus sesudah fixture diperbaiki agar restart MemoryStore tidak membuat saldo baru; pipeline mock source/market/model→entry→HOLD→partial exit→restart→flat/no reentry dan auth/health |
| `golangci-lint run ./internal/catalyst/... ./cmd/catalyst/... ./internal/broker/... ./internal/managed/... ./internal/notify/telegram/... ./internal/exchange/binance/...` | 0 issues pada checkpoint sebelum edit terakhir; bukan hasil lint final |
| PG lifecycle bertag integration, `-run '^TestPilotPostgresRestartAndLease$'` | Lulus, race, count=1, sekitar 2,514 detik; migrasi DB test, pipeline fixture yang sama, jurnal execution, restart runtime dan lease kedua ditolak |
| `CGO_ENABLED=0 go test -c ./internal/catalyst -o /tmp/catalyst-spider-tests/catalyst.test` | Lulus kompilasi untuk native binary test; tidak menjalankan seluruh unit test |
| Native container `-test.run '^TestSpiderBinary' -test.v -test.timeout 60s` | Lulus protocol (~2,12s) dan proxy denial (~0,01s), tanpa jaringan keluar; memakai binary Spider p3.1 image lokal lama dan test executable Go WIP yang di-mount |
| `git diff --check` saat handover | Tanpa output; pemeriksaan statis whitespace, bukan verifikasi fungsi |

DB test lokal dibuat sebagai container `catalyst-verify-20261028` memakai
TimescaleDB `2.30.2-pg17`, 1 CPU/1 GiB/pids128, port loopback sementara `37209`,
database `tradingbot_catalyst_test`. URL fixture tidak dicatat di sini agar
prosedur tidak menjadi rujukan credential; gunakan env
`TB_TEST_CATALYST_DATABASE_URL` khusus DB `_test` saat mengulang. Container
`--rm` dihentikan pada pause; tidak ada database test yang perlu di-resume.

Perintah native terakhir:

```sh
CGO_ENABLED=0 go test -c ./internal/catalyst -o /tmp/catalyst-spider-tests/catalyst.test
podman run --rm --network none --read-only --cpus 0.5 --memory 512m --pids-limit 128 --cap-drop all --security-opt no-new-privileges --tmpfs /tmp:size=16m,noexec,nosuid -v /tmp/catalyst-spider-tests:/test:ro -e TB_TEST_SPIDER_BINARY=/usr/local/bin/spider --entrypoint /test/catalyst.test localhost/trading-bot-catalyst:dev -test.run '^TestSpiderBinary' -test.v -test.timeout 60s
```

Fixture TLS memakai `--accept-invalid-certs` hanya pada invocation uji. Collector
produksi tidak memakai flag itu. Denial test menunjuk origin TLS loopback dan
proxy yang selalu 403: proxy menerima request, origin menerima nol request,
binary berhenti tanpa bypass/hang. Ini bukan seluruh matriks redirect, DNS,
TLS atau kontrak situs publik.

## Lanjutan 28 Oktober malam

| Perintah/area | Hasil dan cakupan |
|---|---|
| `go build ./...`, `go vet ./...` | Lulus pada checkout akhir lanjutan |
| `go test -race -count=1 ./...` | Lulus seluruh paket, termasuk test baru `TestSpiderArticleLinksSeedOrderSameHost` dan `TestSpiderScrapesSelectedArticles` (fake binary dua fase); tanpa tag integration |
| `golangci-lint run ./...` | 0 issues |
| Spider CLI image lokal, `crawl --output-links` ke Ethereum depth 2 vs 5 | Depth 2 tanpa artikel; depth 5 artikel bercampur kategori, urutan tidak stabil |
| `go run ./cmd/catalyst -action collect` dengan policy akhir, arsip scratchpad, `-env-file /dev/null` | Exit 0, ketiga sumber seed + 4 artikel terbaru; laptop, bukan jaringan VPS |

PG integration, native container test, image, Compose, CI dan VPS tidak dijalankan
pada putaran lanjutan.

## Failure yang sempat ditemukan dan ditutup pada checkpoint

- Nama helper test `writeInput` bertabrakan dengan fungsi produksi baru:
  fungsi produksi menjadi `writePilotInput`.
- Fixture restart awal membuat MemoryStore baru sehingga seluruh history
  hilang; factory test diperbaiki untuk membuka ledger yang sama. Ini bug
  fixture, bukan bukti bahwa recovery produksi kehilangan saldo.
- Aksi CLI `market` awal kurang import `net/http`; diperbaiki sebelum lint 0.
- Test PG awal menyebut journal operation `fill`; nama implementasi adalah
  `execution`. Assertion diperbaiki dan test PG kemudian lulus.

## Belum diverifikasi pada checkout akhir

- Semua test/race/vet/lint/build setelah seluruh perubahan terakhir, termasuk
  test baru `RunSender`, alias receipt pasca-event dan frontend baru.
- Image worker WIP, overlay Compose pilot dan CI baru. Regex CI native masih
  hanya menjalankan protocol test lama; perlu mencakup denial test.
- Worker `Pilot.Run` penuh dalam proses/container, model macet versus timer/
  feed, outage DB/lease, crash, health, SIGTERM dan recovery posisi/order aktif.
- Data Binance nyata pada adapter baru; source Ethereum homepage/depth dan
  penyelesaian robots Solana; gateway/prompt v5 dengan sumber + quote nyata.
- Ketepatan event verifier pada format/scope nyata, koreksi/cancel/negasi,
  mainnet versus testnet dan identitas event lintas berita.
- QA browser, notifikasi P3 nyata, backup/restore P3, retensi/kuota dalam operasi
  panjang, benchmark resource dan penerimaan prospektif 1/3/7 hari.

Tidak ada order sungguhan, inference v5 nyata, worker/trial pasar laptop,
commit/push kode WIP, atau deployment pilot baru. Hasil rilis komponen pukul
21.00 WIB tetap terpisah di [bukti rilis](catalyst-release.json).
