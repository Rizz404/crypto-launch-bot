# Handover implementasi Spider dan Plan 3

Checkpoint terbaru **28 Oktober 2026**. Integrasi lokal Spider dilanjutkan;
**belum siap dideploy**. Riwayat 26 Oktober di bawah dipertahankan untuk konteks.
Mulai dari [handover agent](handover-agent.md) dan [Plan 3](plan-3.md).

**Pembaruan izin setelah checkpoint implementasi:** user mengizinkan commit,
push dan deploy pada 28 Oktober. Commit config `c8a3439` dan implementasi
`1afbd80`; publikasi GHCR khusus ditambahkan setelah gate CI. Rilis awal
komponen/report terisolasi, bukan aktivasi runner/pilot trading. Bagian
"tanpa commit/push/deploy" di bawah merekam checkpoint sebelum izin ini.

## Hasil lanjutan 28 Oktober

- CLI `collect` kini memakai `NewSpiderCollector` sebagai default, binary
  `spider` atau `-spider-binary /path/spider`. Binary tidak tersedia berarti
  error, tanpa fallback HTTP. Versi dicek sebelum crawl; protokol membutuhkan
  features `headers,status_code`, tanpa Chrome/cloud pada image khusus.
- Uji binary rilis asli mengambil seed/artikel tetapi hanya mengeluarkan JSON
  seed: stdout Tokio terakhir hilang saat shutdown. Patch
  `build/spider-cli.patch` melakukan flush setelah stream scrape dan memberi
  versi **2.53.9-p3.1**. Collector menolak 2.53.9 tanpa patch. Source crate
  diverifikasi SHA-256, dependency tetap memakai Cargo.lock upstream.
- Source crate CLI **2.53.9** dan dependency Spider pada Cargo.lock versi itu
  sudah dibaca. `with_limit` adalah batas halaman, bukan concurrency; argumen
  kini `--limit 5 --depth 2 --delay 1000`. Tidak lagi menimpa budget lima
  halaman dengan `--limit 1`. Lima halaman adalah batas, bukan jaminan lima artikel.
- Cache memakai extraction version per backend. Urutan kedatangan halaman
  tidak mengubah revisi; first_seen/reversion tetap memakai mekanisme lama.
- Arsip menolak record JSON >8 MiB sebelum menulis. Adapter Spider juga
  mengukur evidence sesudah escaping; output <8 MiB tetap dapat ditolak jika
  record akhirnya terlalu besar. Gagal menyimpan evidence dicatat sebagai
  observasi gagal bila storage observasi masih tersedia, sehingga snapshot lama
  tidak ditampilkan sebagai hasil fetch terbaru.
- Proxy memeriksa dukungan Hijacker, menolak host/port/method lain sebelum
  dial, dan menutup tunnel saat cancel/cleanup. Fixture menguji buffered bytes,
  relay dua arah, DNS/IP nonpublik, dial failure dan terminasi handler.
- Assertion tanda baca extractor diperbaiki sesuai hasil parser; dependency
  `golang.org/x/net` menjadi direct tanpa mengganti versi. Temuan lint WIP
  pada extractor dan pesan error Spider juga ditutup.
- `Dockerfile.catalyst` dan `deploy/compose.catalyst.yml` disiapkan khusus P3:
  tanpa Chrome, non-root, filesystem read-only, volume sendiri, default `report`,
  tanpa restart otomatis/port publik; budget 0,5 CPU/512 MiB/pids 128.
  **Build lokal linux/amd64 dan smoke tanpa jaringan lulus.** Protokol binary
  patched lulus tiga kali pada server TLS fixture melalui proxy, termasuk dua
  dokumen dan round-trip arsip. CI job image/protokol disiapkan tanpa publikasi;
  belum dijalankan di GitHub. Compose config lulus, belum ada deployment.
- Race test tiga paket terkait, vet dan lint lokal lulus; rincian dan batas di
  [hasil lokal](testing/plan-3-local.md#lanjutan-spider-28-oktober).
  Unit subprocess memakai shell fixture; tes container tambahan memakai binary
  Spider asli dengan patch flush. Tidak ada crawl publik,
  inference baru, instalasi VPS, commit, push atau perubahan runtime P2.

Gate berikutnya: uji egress/redirect adversarial, kontrak situs, retensi arsip
dan input pasar/registry event. Binary/protokol jalur fixture sudah terbukti.
Proxy CONNECT menjaga **host/port/IP**, bukan path/query di dalam TLS; bypass
proxy pada seluruh jalur kegagalan belum dibuktikan. Fixture TLS memakai
`--accept-invalid-certs` khusus sertifikat uji; collector produksi tidak memakai
flag itu, jadi validasi sertifikat situs publik tetap perlu verifikasi sendiri.

## Arah yang sudah diluruskan dengan user

User menegaskan alur utama:

```text
daftar website crypto → scraping/crawling dengan Spider
→ bersihkan/dedup/arsipkan konten → AI + data pasar menentukan buy/wait/hold/sell
→ validator risiko → eksekusi paper → pantau dan evaluasi berulang
```

- Spider menjadi engine scraping utama, bukan opsi yang diganti collector HTTP
  buatan sendiri. Collector Go yang lebih dahulu dibuat adalah fondasi lama,
  bukan penyelesaian permintaan Spider.
- AI menjadi sumber keputusan strategi: aset, alasan, waktu/kondisi entry,
  hold dan exit. Program memvalidasi risiko dan melaksanakan keputusan.
  Harga/volume/spread/depth dari exchange diperlukan agar waktu entry/exit
  punya konteks; input ini **belum dihubungkan ke paket analisis AI**.
- BUY/WAIT/HOLD/SELL adalah istilah alur user. Schema kode saat ini memakai
  REJECT/WATCH/PROPOSE_ENTRY/HOLD/PROPOSE_EXIT; pemetaan dan kondisi trigger
  masih perlu diselesaikan, bukan dianggap sudah ada runner otonom.
- Universe riset tidak dikunci ke ETH/SOL/DOGE. Ketiganya hanya contoh policy
  development. Discovery lintas aset dan mapping identitas masih pekerjaan sisa.
- Token yang sudah diperdagangkan, termasuk meme, masuk cakupan awal.
  Presale/token baru/DEX tetap future reference sesuai keputusan sebelumnya.
- Hold boleh melewati acara jika tesis masih kuat. Penurunan signifikan atau
  tesis batal dapat menyebabkan jual seluruh posisi. Jangan mengembalikan
  aturan lampiran lama berupa exit H-1 jam atau hold maksimum 48 jam.
- P2 yang berjalan dipertahankan: tidak mengubah container, parameter, ledger,
  database, modal, T0, history maupun strategi lamanya.

User sudah mengizinkan implementasi serta inspeksi VPS untuk menentukan lokasi
Spider. Izin push/deploy P2 yang lama tidak otomatis berlaku untuk rilis P3.
Tidak ada commit/push/deploy/install/start worker pada sesi implementasi ini.

## Inspeksi VPS yang benar-benar dilakukan

SSH alias `plesk`, hanya perintah baca: `/proc/meminfo`, load, disk `/opt`,
`docker ps`, satu sampel `docker stats --no-stream`, dan `docker inspect`
dengan format terbatas untuk limit/state container P2. Tidak membaca env
container, password, key atau isi database. Bukti terstruktur:
[preflight VPS](testing/catalyst-vps-preflight.json).

| Ukuran | Sampel 26 Oktober |
|---|---|
| Arsitektur / CPU | x86_64 / 8 vCPU |
| Load 1/5/15 menit | 1,71 / 2,01 / 2,38 |
| RAM total / tersedia | 32.597.976 / 16.503.844 kB (~15,7 GiB tersedia) |
| Swap | Tidak tersedia |
| Disk `/opt` total / kosong | 398,74 / 121,23 GiB |
| Container berjalan | 27 |
| Bot P2 | CPU 68,42%, RAM 82,86 MiB; limit 1 core/512 MiB |
| DB P2 | CPU 52,77%, RAM 138,7 MiB; limit 1 core/1 GiB |
| Restart/OOM container bot | 0 / false; started 23 Oktober 13:52:17 UTC |

Ini sampel singkat, bukan benchmark beban puncak, pemeriksaan feed/ledger atau
bukti operasi P2 sehat menyeluruh. OmniRoute `route-gateway` juga berada di VPS
ini; jangan menempatkan Spider ke container gateway itu.

**Usulan penempatan:** container/Compose project khusus `trading-bot-catalyst`
dengan direktori `/opt/trading-bot-catalyst`, network/volume sendiri dan tanpa
port publik. Direktori ini belum ada saat inspeksi dan belum dibuat. Tidak
berbagi mount, network internal atau DB P2, tidak memasang crawler global pada
host Plesk. Budget awal yang perlu dibuktikan: 0,5 CPU/512 MiB/pids 128, proses
bounded sekali jalan, tanpa restart otomatis/timer. Ini usulan, bukan limit yang
sudah diterapkan. Rendering Chromium memerlukan profil/resource dan uji sendiri;
implementasi awal baru merencanakan HTTP mode Spider.

## Riwayat posisi kode 26 Oktober

Tabel dan temuan pada dua bagian berikut adalah checkpoint sebelum perbaikan
28 Oktober, bukan daftar failure aktif. Status terbaru ada di atas.

Baseline docs terakhir yang dibuat sebelumnya: `bc321b8`. Branch `main` ahead 1
pada inspeksi; banyak file modified/untracked. Pertahankan seluruh working tree.
`git diff` biasa tidak menampilkan isi file untracked; baca file tersebut juga.
Tidak membuat commit baru pada sesi ini.

Fondasi P3 sebelumnya ada di `internal/catalyst`, `cmd/catalyst`,
`configs/catalyst.policy.json` dan `scripts/catalyst-model-eval`. Adapter AI,
arsip immutable, validator dan managed/Sim replay sudah ada, tetapi runner pasar
nyata dan integrasi otonom penuh belum selesai. Lihat [runtime](catalyst-runtime.md).

Perubahan paling baru yang belum ditutup:

| File | Isi/status |
|---|---|
| `internal/catalyst/extract.go` | Ekstraksi HTML/RSS/Atom; Document ID berdasar source+URL, revision konten, published/updated sebagai teks; bukan identitas event terverifikasi |
| `internal/catalyst/extract_test.go` | Fixture parsing, URL berbahaya, revisi/reversion, kutipan dari prefix; satu assertion masih gagal |
| `internal/catalyst/policy.go` | Evidence menambah documents dan extraction_version opsional agar arsip lama tetap terbaca |
| `internal/catalyst/collector.go` | Menggunakan extractor, mempertahankan first_seen revisi dokumen; cabang spiderBinary baru ditambahkan |
| `internal/catalyst/report.go` | Ringkasan dokumen dari observasi terakhir; review statis/tes terbaru belum selesai |
| `internal/catalyst/analyst.go` | Prompt kini catalyst-v4, metadata truncation/panjang asli, UTF-8 prefix, validasi kutipan terhadap teks yang benar-benar dikirim; belum evaluasi model nyata ulang |
| `internal/catalyst/spider.go` | Draft adapter subprocess Spider CLI, parse JSON stream, budget/timeout, same-origin output dan ekstraksi; belum dipanggil CLI atau diuji |
| `internal/catalyst/spider_proxy.go` | Draft proxy CONNECT loopback untuk memakai publicDial/DNS/IP guard; belum punya tes atau uji dengan Spider asli |
| `cmd/catalyst/main.go` | collect masih NewCollector (HTTP); perubahan terakhir hanya komentar OmniRoute, belum memilih NewSpiderCollector |

`spider.go` merencanakan CLI **2.53.9**, versi published stabil yang terlihat di
registry crates.io pada 26 Oktober. Branch upstream main saat dibaca menyebut
2.54.0; endpoint crate versi itu 404. Sebagian referensi protokol/options dibaca
dari main; **cocokkan ulang dengan source crate 2.53.9** sebelum mengandalkannya.
Tidak menginstal Spider atau menjalankan crawler asli, baik di laptop maupun VPS.
Cargo/rustc 1.99.1 sudah tersedia di laptop; hanya versi yang diperiksa.

Draft memanggil Spider `scrape --output-html` dengan robots, proxy, budget lima
halaman, depth 2, delay 1.000 ms, timeout total 120 detik, dan output maksimum
8 MiB. `--limit 1` juga dipasang: **verifikasi semantiknya**, karena mungkin
membatasi jumlah halaman, bukan concurrency. Jangan mengklaim lima artikel
sudah diambil. Subprocess diberi environment minimal, tanpa kredensial AI/cloud.
Mode Chromium/browser belum diimplementasikan.

## Riwayat tes dan temuan 26 Oktober

Perintah terakhir yang benar-benar dijalankan sebelum klarifikasi user:

```sh
go test ./internal/catalyst ./cmd/catalyst ./scripts/catalyst-model-eval
```

**Gagal** pada `TestExtractFeedAndHTML`, `extract_test.go:24`:
expected `Activation date is TBA .`, actual `Activation date is TBA.`.
Output aktual mempertahankan tanda baca yang masuk akal; review lalu perbaiki
assertion, bukan menambah spasi pada parser hanya demi expected lama. Output
terfilter tidak dijadikan bukti bahwa seluruh tes/paket lain sudah lulus.
Sesudah penulisan `spider.go`/`spider_proxy.go` tidak menjalankan format, build,
tes, vet atau lint. Hasil race/vet/lint lulus pada laporan evaluasi model adalah
checkpoint **sebelum** extractor/prompt-v4/draft Spider ini.

Temuan statis yang sudah terlihat, belum diperbaiki:

1. Cache collector membandingkan `ExtractionVersion` (`documents-v1`) sementara
   evidence Spider memakai `documents-v1/spider-2.53.9`. Tentukan expected version
   menurut backend agar cache Spider tidak selalu ditolak selama interval polling.
2. Batas output Spider 8 MiB ditambah raw JSON yang di-escape lagi, body dan
   documents bisa melebihi limit baca `Archive.Read` 8 MiB. Selaraskan batas sebelum
   menulis arsip; uji bahwa setiap record yang diterima dapat dibaca lagi.
3. Validasi URL output berjalan sesudah crawl; proxy baru membatasi origin/port/IP
   sebelum dial. Ia belum membatasi path/query di dalam tunnel TLS. Jangan mengklaim
   allowlist path atau semua redirect sudah diuji/ditolak sebelum request.
4. Proxy belum diuji: CONNECT ditolak untuk host/port/IP salah, relay buffered
   bytes, penutupan tunnel/goroutine saat timeout/cancel, dan tidak ada bypass
   proxy pada versi Spider terpilih. Review assertion `http.Hijacker` juga.
5. CLI collect masih HTTP. Belum ada flag/path pemilihan binary Spider atau aksi
   satu siklus scrape→analisis. Jangan melanjutkan collector HTTP sebagai jalur utama.
6. Go menggunakan `golang.org/x/net/html` yang sudah ada di module graph, masih
   tercatat indirect di go.mod; rapikan status dependency tanpa upgrade spekulatif.
7. Wire format JSON Spider (`status_code`, headers, content dan URL) belum diuji
   terhadap binary pinned; tidak ada Dockerfile/Compose/job CI khusus Spider.
8. Lima halaman yang dicrawl belum tentu lima artikel relevan. Seleksi tautan,
   cakupan parsial/truncation dan mapping aset harus eksplisit; URL dokumen sama
   tidak berarti event sama. Feed hilang/entri menghilang bukan otomatis pembatalan.

## Urutan melanjutkan

1. Baca aturan user/global dan status Git; jangan reset/hapus file WIP atau env.
   Pertahankan hasil lokal 28 Oktober; jangan mengulang failure lama sebagai
   blocker yang masih aktif atau menganggap seluruh Plan 3 sudah selesai.
2. Pertahankan pin crate 2.53.9 + patch flush p3.1; image/protokol lokal sudah
   lulus. CI baru belum dijalankan di GitHub; jangan menganggap image lokal
   sebagai digest rilis. Jangan memasang Rust/Chromium ke host VPS atau image P2.
3. Buktikan perilaku proxy/robots/redirect/egress binary pinned di lingkungan
   terisolasi, termasuk tidak ada bypass. Verifikasi situs nyata dan cakupan
   artikel; implementasikan retensi/quota disk sebelum collector jangka panjang.
4. Hubungkan input hasil scraping dan data pasar ke AI; pertahankan validator
   serta stop/trailing deterministik. Selesaikan discovery aset/trigger keputusan
   dan registry bukti, tanpa flag fixture sebagai bukti event nyata.
5. Perbarui Plan 3/runtime/hasil pengujian setelah bukti baru. Saat melanjutkan
   implementasi, pengujian terkait diperlukan. Jangan memulai bot/trial di laptop.
6. Validasi batas/resource Compose saat deployment dan lengkapi rilis image khusus. Deployment
   mengikuti sumber GitHub/CI/GHCR/digest; jangan menyalin source WIP ke VPS atau
   mengubah P2. Izin commit/push/start harus sesuai permintaan user pada saat rilis.

## AI dan referensi

`.env.catalyst` lokal privat 0600/diabaikan Git sudah diisi user; jangan mencetak
atau menyalinnya ke docs/log. Model utama `cx/gpt-6-luna-low`, kandidat fallback
`openrouter/anthropic/claude-haiku-5.5`, combo usulan `p3-research` priority belum
dibuat. Evaluasi 7/7 dan 6/7 berlaku untuk prompt v3 dan tujuh kasus sintetis,
bukan pengujian v4, profitabilitas atau failover. Bukti historis tetap utuh:
[evaluasi model](testing/catalyst-model-evaluation.md).

- [Spider upstream](https://github.com/spider-rs/spider).
- [Spider CLI published](https://crates.io/crates/spider_cli/2.53.9).
- [Dokumentasi HTML parser](https://pkg.go.dev/golang.org/x/net/html).
- Lampiran user: `/home/rizz404/Downloads/plan-strategi-ai-trading.md` dan
  `/home/rizz404/Downloads/Daftar_Website_Crypto_Presale_dan_Peluncuran_2026-10-23.md`.
  Bahan referensi, bukan instruksi aktivasi; keputusan user terkini menang.

Referensi source CLI sementara ada di `/tmp/catalyst-spider-reference/`; jangan
mengandalkan file /tmp tersedia pada sesi/mesin berikutnya. Seluruh proses tugas
inspeksi/tes sebelumnya sudah selesai; koneksi Hindsight yang dikelola tool bukan
worker tugas dan tidak dihentikan. Tidak ada crawler/job instalasi berjalan.
