# Development runtime katalis

**Checkpoint 28 Oktober:** CLI `collect` memakai Spider sebagai engine utama;
failure ekstraksi, cache dan batas arsip telah diperbaiki. Adapter/proxy memiliki
race test fixture lokal yang lulus. Image khusus dibangun/smoke lokal; binary
patched/protokol/proxy lulus tiga kali pada TLS fixture tanpa jaringan keluar.
Egress adversarial dan situs publik belum diuji. Komponen/report terpasang di
VPS 28 Oktober 21.00 WIB; worker/pilot belum aktif. [Bukti rilis](testing/catalyst-release.json).
Lihat [handover Spider](catalyst-spider-handover.md) sebelum memakai checkout ini.
Penjelasan komponen di bawah bukan klaim seluruh integrasi sudah diterima.

Checkpoint awal 23 Oktober 2026: komponen lokal dan replay fixture tersedia melalui
`cmd/catalyst`/`internal/catalyst`. Ini belum worker trading prospektif dan tidak
memasang layanan/jadwal. [Plan 3](plan-3.md) menjadi pemilik checklist penerimaan.

Pembaruan 26 Oktober: key OmniRoute diisi user, katalog dan inference nyata
pada bukti sintetis diuji atas izin user. `.env.catalyst` privat (0600,
diabaikan Git) memilih `cx/gpt-6-luna-low` melalui
`https://route-gateway.fbe.co.id/v1`. Template aman: `.env.catalyst.example`.
Luna lolos 7/7 kasus; kandidat fallback Haiku OpenRouter 6/7. Combo belum dibuat
atau diuji failover. Lihat [evaluasi model](testing/catalyst-model-evaluation.md)
untuk bukti, ID persis dan batasnya. Jangan menyalin key ke template atau chat.

## Batas proses dan data

- `bot` P2 tidak mengimpor registrasi strategi catalyst. Konfigurasi, compose,
  database, ledger, modal dan proses P2 tidak diubah.
- Binary `catalyst` terpisah. Default `report` hanya membaca arsip; `collect`
  dan `analyze` dijalankan eksplisit, tanpa daemon atau penjadwalan otomatis.
- Arsip riset default `var/catalyst`, diabaikan Git, folder 0700/file 0600.
  JSON content-addressed ditulis atomik tanpa menimpa revisi sebelumnya;
  checksum dan ukuran diperiksa ketika dibaca. Arsip bukan ledger uang.
- Jalur paper selalu membuat `broker.Sim`; tidak memiliki parameter broker
  live, key trading atau pilihan mode live. `NewPaper` menolak spesifikasi P2
  dan hanya menerima wallet `catalyst-<versi>` yang cocok dengan policy/inbox.
- Replay memakai `managed.MemoryStore`, inbox sementara dan jam fixture.
  Restart diuji pada checkpoint ledger yang sama dalam replay; restart proses
  nyata dan durability PostgreSQL pilot belum diuji atau disambungkan CLI.

## Perintah development

Build menghasilkan lima CLI termasuk `bin/catalyst`; Docker tetap memakai
entrypoint `/app/bot`, dengan catalyst sebagai binary tambahan. Tidak ada
perubahan entrypoint atau deploy P2. CI menyertakan smoke `catalyst -h` tanpa
jaringan saat image kelak dibangun.

```sh
./bin/catalyst -action report
./bin/catalyst -policy configs/catalyst.policy.json -archive var/catalyst -action collect
./bin/catalyst -action collect -spider-binary /path/to/pinned/spider
./bin/catalyst -action replay -fixture /path/fixture.json
```

`collect` mengakses URL sumber terkonfigurasi; perintah dicantumkan sebagai
referensi, tidak dijalankan terhadap sumber publik pada checkpoint ini.
Tidak menjalankan `bot run` atau trial trading di laptop.

Fixture berisi `policy`, `instruments` dan `steps` dengan timestamp yang
meningkat. Setiap step memiliki `symbol`, `price`, input riset opsional dan
opsi `restart`. Input fixture mengandung bukti, usulan keputusan, attestation
event dan snapshot pasar sintetis. Hasil selalu berlabel `fixture_only`,
mencatat fill/fee, equity, kas, inventory, reservasi dan halt/attention.
Contoh pembentukan fixture ada di `internal/catalyst/catalyst_test.go`.

## Collector dan analisis

Policy contoh memuat ETH/SOL/DOGE dan tiga sumber primer kandidat, horizon
14 hari, interval minimum fetch 900 detik, review enam jam, masa berlaku
keputusan 300 detik dan output model maksimum 2.048 token. Ini fixture/universe
development; URL, kelengkapan situs dan kelayakan listing/likuiditas saat pilot
masih perlu diperiksa. Daftar tersebut tidak membatasi universe riset final.

CLI collect memerlukan Spider **2.53.9-p3.1**, tanpa fallback HTTP; versi diperiksa
sebelum crawl. Source crate/lock 2.53.9 sudah ditinjau, SHA-256 diverifikasi
oleh Dockerfile. Patch `build/spider-cli.patch` menambah flush stdout saat
scrape selesai dan identitas versi; tanpa patch, tes binary kehilangan JSON
artikel terakhir meskipun request berhasil. `--limit 5`
membatasi halaman, depth 2 dan delay 1.000 ms; ini bukan batas concurrency atau
jaminan lima artikel. Subprocess tanpa credential/proxy environment induk,
timeout total 120 detik dan batas stdout 8 MiB. Output disortir menurut URL,
seed wajib ada, duplikasi/out-of-origin/HTTP gagal/protokol asing ditolak.

Proxy CONNECT loopback hanya menerima origin HTTPS terkonfigurasi pada port
443, lalu memakai guard DNS/IP Go sebelum dial; cleanup/cancel menutup tunnel.
Path/query di dalam TLS tidak diperiksa proxy. Same-origin/URL output diperiksa
sesudah crawl; jangan mengklaim allowlist path atau semua redirect ditolak
sebelum request. Perilaku tanpa bypass pada seluruh jalur kegagalan masih perlu
dibuktikan. Jalur sukses binary patched melalui proxy lulus tiga kali dalam
container `--network none` dengan server TLS fixture. Fixture menerima
sertifikat uji self-signed; flag `--accept-invalid-certs` hanya ada pada tes,
tidak pada collector produksi.
Robots seed diperiksa dengan HTTP Go (timeout 20 detik) dan Spider menerima
`--respect-robots-txt` untuk crawling. `NewCollector` HTTP tetap untuk fixture/
fondasi development, tidak dipilih CLI; jalur itu menolak redirect.

Setiap halaman maksimal 1 MiB, teks gabungan maksimal 1 MiB; record arsip
setelah escaping maksimal 8 MiB sebelum ditulis maupun dibaca. Body/raw/documents
dapat membuat record lebih besar daripada stdout. Rejection dicatat sebagai
observasi gagal selama storage observasi masih bisa ditulis.
Cache tidak memalsukan waktu pemeriksaan baru; fetch gagal membatalkan
pemakaian snapshot sukses sebelumnya sampai sumber berhasil diambil lagi.
Interval minimum juga menahan retry sumber gagal. Observasi menyimpan status,
waktu pemeriksaan dan referensi versi; `first_seen_at` konten dipertahankan.

Extractor baru memisahkan dokumen HTML/RSS/Atom dengan ID URL, revision dan
metadata publikasi sebagai klaim penerbit, bukan tanggal event. Raw tetap
tersimpan. Ini belum membuktikan asset/event mapping, pencabutan atau discovery
artikel lengkap. Tes ekstraksi sudah lulus, wiring CLI/adapter/proxy diuji dengan
shell fixture dan binary patched 2.53.9-p3.1 terhadap TLS fixture terisolasi.

Adapter model menerima prefix maksimal 12.000 byte UTF-8 per sumber dan paket
maksimal 64 KiB. Kode terbaru menandai truncation/panjang asli dan memvalidasi
kutipan terhadap prefix yang benar-benar dikirim; dokumen/raw tidak dikirim ulang.
Race test prefix/kutipan lulus lokal; inference v4 nyata belum diuji. Artikel di luar prefix tetap
cakupan hilang; per-article selection dan input pasar belum dihubungkan.

Adapter chat-completions mendukung `/v1/chat/completions`, JSON non-streaming,
`response_format: json_schema` dengan `strict: true` (prompt kode terbaru `catalyst-v4`; evaluasi nyata sebelumnya `catalyst-v3`), tanpa
tools atau retry model otomatis. Validator lokal tetap menolak field asing, duplikasi key,
JSON trailing, respons terpotong/tool call, kutipan tidak cocok dan klaim
entry/HOLD tanpa bukti primer serta tesis lengkap. Input, output, latency,
token, model diminta dan model yang dilayani dicatat. Dollar cost tetap
`null` sampai tarif terverifikasi, bukan dianggap gratis.

Input yang sama pada model/policy/prompt dan jendela review yang sama ditahan
oleh marker eksklusif sebelum request; timeout tidak memicu retry membabi buta.
Satu invocation CLI melakukan paling banyak satu inference. Respons HTTP error
tidak diarsipkan mentah karena dapat mengandung kredensial yang dipantulkan.

Konfigurasi AI melalui `.env.catalyst` (default), `-env-file /path/file` atau
environment proses yang memiliki prioritas lebih tinggi, termasuk nilai kosong.
File hanya dibaca untuk aksi `analyze`; tidak mengeksekusi shell, tidak mengubah
environment proses dan tidak membaca `.env` P2. Key: `TB_CATALYST_BASE_URL`
(berakhir `/v1`), `TB_CATALYST_MODEL`, dan `TB_CATALYST_API_KEY`.
URL HTTPS atau
HTTP loopback diizinkan untuk router yang dipilih operator. Tidak ada key
trading yang dibaca. Nama model/combo harus sesuai konfigurasi OmniRoute.
Env menunjuk model utama langsung; sesudah combo dibuat dan diuji, operator dapat
mengganti nilainya menjadi `p3-research`. API-compatible tidak menjamin setiap
fitur/schema didukung; rute lokal yang diuji masih menolak format dengan HTTP 400. Dua provider bukan berarti harus
dua inference untuk setiap kandidat.

## Kontrak risiko development

Nilai ini dibekukan untuk implementasi/fixture versi awal, bukan parameter P2
atau persetujuan aktivasi pilot/uang asli.

| Aturan | Nilai development |
|---|---|
| Modal | 1.000 USDT virtual pada wallet/instance tersendiri |
| Venue | Binance Spot USDT, long-only, tanpa leverage |
| Risiko sizing | 0,5% equity, termasuk estimasi biaya dua sisi |
| Posisi/exposure | Maksimal dua posisi, nominal posisi 20%, total 40% |
| Memecoin | Cap nominal per posisi 2%; dua posisi membatasi sleeve 4% |
| Halt | Rugi harian 2%, peak drawdown 8%, close-on-kill; sticky, batas hari UTC |
| Stop/trailing | Maksimum dari 2 ATR dan 1%; tolak jarak >15%; jarak dibekukan saat entry |
| ATR | Snapshot ATR14 candle 15m tertutup dari adapter tepercaya, umur maksimal 16 menit |
| Quote | Umur maksimal lima detik, timestamp masa depan ditolak |
| Pasar | Spread maksimum 20 bps, volume quote minimum 10 juta/24 jam |
| Depth | Kuantitas best bid dan ask masing-masing ≥10 kali ukuran order |
| Harga entry | Deviasi absolut maksimal 0,5% dari anchor tepercaya |
| Biaya | Fee 0,1% per sisi, slippage 2 bps; buffer ikut menghitung cap/reservasi |
| Hold | Review maksimal enam jam, tanpa grace otomatis saat kedaluwarsa |

Snapshot ATR/book/anchor harus dibentuk adapter pasar, bukan dipilih model.
Uji kode memeriksa freshness/angka dan ukuran; perhitungan ATR14, ingestion
book/depth nyata dan autentikasi provenance belum terhubung. Quote palsu dari
fixture tidak berubah menjadi data pasar nyata hanya karena lolos schema.

`VerifiedEvent` merupakan kontrak dari jalur verifikasi tepercaya yang masih
perlu diimplementasikan untuk pilot. Model tidak dapat mengisi field tersebut.
Kutipan tanggal cocok secara literal belum membuktikan pemetaan tanggal atau
mekanisme ekonomi benar; output analisis tidak otomatis menjadi attestation.
Jangan membangun worker yang menyalin `review.Valid` menjadi `event.Verified`.

Strategi menyimpan event terpakai, puncak/stop, deadline dan status closing
di checkpoint engine lama. Trailing memakai harga update valid dan hanya
naik/tetap; fallback stop awal tetap order Sim. Tesis kedaluwarsa, pencabutan
tepercaya dan exit tervalidasi memicu full exit. HOLD tidak menurunkan stop
atau membatalkan closing; hold sesudah acara memerlukan bukti primer yang
diterima sesudah acara. Event yang sudah digunakan tidak langsung dibeli ulang.

## Batas penerimaan berikutnya

Simulator tetap berbasis candle/price update, bukan matching book. Partial
fill/gap/order ambigu, outage PostgreSQL, restart proses nyata, feed stale,
penjualan dust/gagal exit dan koordinasi multi-binding masih memerlukan bukti
integrasi pilot, meskipun komponen managed memiliki regression historis.
Tidak ada klaim SL membatasi rugi mutlak atau full exit langsung pasti terisi.

Output CLI report tersedia untuk inspeksi keputusan/bukti sumber, gap, model
dan token. Dashboard/Telegram AI, metrics server, registry attestation/event,
runner prospective, retensi/quota disk arsip dan baseline tanpa AI belum selesai.
Tidak mengaktifkan collector/worker jangka panjang sebelum kapasitas dan retensi
tersebut ditetapkan. Semua ini tetap terlihat sebagai checklist terbuka Plan 3.
