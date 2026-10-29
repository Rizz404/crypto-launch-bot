# Integrasi awal — Hasil riset P4-1

**29 Oktober 2026, WIB.** Metode: membaca dokumentasi resmi, source kontrak
publisher dan halaman launchpad melalui web. Ini verifikasi referensi, bukan
uji adapter, jaminan endpoint dapat diakses dari VPS, atau rekomendasi membeli
token tertentu. Tidak memakai key/layanan berbayar atau menghubungkan wallet.

## Pilihan dan urutan

Pilih **BNB Smart Chain mainnet (`eip155:56`)**, PinkSale, PancakeSwap V2,
dan Binance Spot USDT. Satu ABI EVM membatasi permukaan implementasi awal.
Urutan implementasi: kontrak/arsip → discovery ketiga jalur → pemeriksaan
pasar/on-chain → laporan. Eksekusi finansial tetap P4-4.

Pilihan ini membatasi integrasi, bukan membatasi daftar token: kandidat
ditemukan dari halaman, registry pool dan daftar simbol venue yang berubah.
Umur token global tetap unknown bila belum ada bukti; tidak disamakan dengan
umur pool, listing atau first-seen bot.

| Integrasi | Bukti resmi dan cakupan | Akses/biaya/batas | Keputusan dan gap |
|---|---|---|---|
| PinkSale BSC | [Panduan partisipasi](https://docs.pinksale.finance/investing/how-to-buy-a-presale), [daftar launchpad](https://www.pinksale.finance/launchpads) | Discovery website tanpa koneksi wallet; crawl wajib robots. Tidak ditemukan kontrak API discovery publik stabil dalam referensi yang dibaca. Biaya akses scraping/SLA belum terverifikasi | Pilih Spider pada daftar dan tautan `/launchpad/bsc/…`; jangan gunakan endpoint internal tidak terdokumentasi. HTML kosong/JS, struktur berubah dan pagination menjadi gap |
| PancakeSwap V2 BSC | [Alamat resmi](https://developer.pancakeswap.finance/contracts/v2/addresses), [interface registry publisher](https://raw.githubusercontent.com/pancakeswap/pancake-swap-core/master/contracts/interfaces/IPancakeFactory.sol) | Baca registry via JSON-RPC; tidak membayar gas untuk call publik. Kuota tergantung RPC | Pilih factory `0xca143ce32fe78f1f7019d7d551a6402fc5350c73`; registry `allPairsLength/allPairs`, lalu token0/token1. Tautan source dari halaman alamat resmi mengembalikan 404 saat riset; kecocokan ABI deployed belum diuji. V3/Infinity tidak termasuk |
| RPC BSC | [Endpoint dan batas RPC](https://docs.bnbchain.org/bnb-smart-chain/developers/json_rpc/json-rpc-endpoint/), [finality API](https://docs.bnbchain.org/bnb-smart-chain/developers/json_rpc/bsc-api-list/) | Endpoint publik tercantum, batas terdokumentasi 10.000 request/5 menit; `eth_getLogs` dinonaktifkan pada endpoint mainnet yang disebut di halaman | Pilih `https://bsc-dataseed.bnbchain.org`; verifikasi chain ID 0x38, header finalized, code/call pada nomor block tersebut. Tanpa fallback latest; tidak memakai scanner log historis |
| DEX Screener | [Referensi API](https://docs.dexscreener.com/api/reference) | Endpoint pasangan publik; tarif/SLA dan kuota endpoint pasangan belum dipastikan dari halaman yang berhasil dibaca. Tidak ada langganan dipasang | Pelengkap metadata/likuiditas per pool, bukan sumber identitas primer. Cocokkan chain, pool dan kedua token dengan registry. `pairCreatedAt` adalah klaim pembuatan pool, bukan bukti trade pertama |
| GoPlus | [Token security](https://docs.gopluslabs.io/reference/tokensecurityusingget_1), [arti field](https://docs.gopluslabs.io/reference/response-details), [kuota](https://docs.gopluslabs.io/reference/support) | Dokumentasi menyebut gratis, 30 request/menit; auth/ketersediaan aktual belum diuji | Pelengkap risiko untuk chain 56; field hilang tetap unknown. Proxy/mint/admin/fee dan indikasi honeypot bukan bukti pasti exit dapat dieksekusi |
| Binance Spot | [REST publik](https://developers.binance.com/en/docs/products/spot/rest-api), [pasar dan aggregate trades](https://developers.binance.com/docs/binance-spot-api-docs/rest-api/market-data-endpoints) | Market-data-only host tanpa key trading. Rate limit mengikuti response/config venue; tidak diasumsikan tak terbatas. Biaya perdagangan tidak relevan untuk discovery dan belum dibekukan | Pilih exchangeInfo SPOT, pasangan USDT dinamis, aggregate trade ID 0 dan book ticker. Jika histori tidak dimulai dari ID 0, waktu listing tetap unknown. Simbol base adalah identitas venue, bukan mapping kontrak EVM |

Angka biaya layanan lain tetap **unknown**, bukan nol. Gas/fee kontribusi,
swap, claim, refund dan trading diriset ulang saat policy/eksekusi dirancang.
Tidak mewarisi budget AI atau parameter uang dari P3.

## Lifecycle presale dan kelayakan akses

Kontribusi membutuhkan wallet dan dapat dibatasi whitelist serta limit pool.
Claim bergantung penyelesaian sale dan dapat mengikuti vesting; pengembalian
dana bergantung kondisi kegagalan serta kontrak. Rujukan terpisah:
[claim](https://docs.pinksale.finance/investing/how-to-claim-tokens),
[vesting](https://docs.pinksale.finance/launchpads/vesting-contributor),
[withdrawal](https://docs.pinksale.finance/investing/how-to-withdraw-your-contribution).
Publikasi halaman dan adanya bytecode belum membuktikan pool resmi, aturan
sale, kelayakan akun, ataupun hak refund. P4-2 menahan seluruh entry presale
sampai provenance kontrak/ABI, state dan syarat partisipasi diverifikasi untuk
adapter eksekusi P4-4. Tidak ada wallet yang disiapkan pada fase ini.

## Alternatif yang ditunda

| Pilihan | Alasan dan pemicu evaluasi ulang |
|---|---|
| Solana | Perlu model mint/authority, program, RPC/finality dan adapter tersendiri. Tunda agar jalur EVM pertama selesai; bukan penilaian potensi token |
| Ethereum/Base dan chain EVM lain | Registry serta provenance kontrak berbeda; tambah setelah penerimaan jalur BSC. Biaya aktual tidak dibandingkan atau diklaim lebih mahal |
| GeckoTerminal/CoinGecko | [Dokumentasi new pools terkini](https://docs.coingecko.com/reference/latest-pools-network) menampilkan host Pro berkey. Dokumentasi lama GeckoTerminal mengarah ke portal CoinGecko; akses public lama tidak dibuktikan. Tunda agar tidak mengasumsikan endpoint key-free yang belum terverifikasi |
| DODO CrowdPooling | [Dokumentasi subgraph](https://docs.dodoex.io/en/developer/developers-portal/api/web3-data/subgraph-data) mensyaratkan key untuk layanan self-hosted atau layanan Graph berbiaya. Tunda sampai akses tersedia; baca source factory bukan bukti semua sale masih aktif |
| CEX selain Binance, Pancake V3/Infinity | Memerlukan kontrak data/adapter dan evaluasi akses baru; tidak diperlukan untuk mencakup tiga jalur awal |

## Kontrak yang dibekukan untuk discovery v1

- Network BSC 56; alamat EVM nonzero dinormalisasi lowercase. Identitas CEX
  memakai ID base Binance. Tidak menggabungkan kontrak lintas-chain/ticker.
- ID kandidat dari jenis, venue, ID sale/pool/pair dan chain; revisi terpisah
  dari receipt time. Perubahan identitas terverifikasi pada ID yang sama ditahan.
- Waktu UTC; sumber tanggal dibedakan: publisher schedule, provider pool
  creation dan first aggregate trade ID 0. Listing tepat 30 × 24 jam masih
  dalam universe; sesudahnya tidak mendapat entry baru.
- RPC harus memberi chain 56 dan finalized block dengan hash/nomor/timestamp
  valid, maksimal dua menit ketika snapshot diperoleh. Tidak mengarang finality
  dari jumlah detik tunggu atau mengganti finalized ke latest.
- Bukti kandidat/referensinya maksimal 15 menit untuk penilaian riset. TTL
  pra-transaksi yang lebih ketat belum ditetapkan dan wajib dibekukan di P4-3.
- Status `accepted_for_research`, `held`, `rejected`; semua `entry_allowed=false`.
  DEX/presale belum dapat diterima untuk transaksi hanya dari scanner ini.

## Budget discovery dan prasyarat penerimaan

CLI satu putaran, satu collector pada satu arsip. Default lima pool terakhir
dan lima simbol CEX per putaran (maksimal sepuluh); Spider maksimal lima halaman
termasuk seed. CEX memprioritaskan yang belum diperiksa lalu yang paling lama
diperiksa; bootstrap bukan backfill penuh. DEX memulai dari tail lalu menyimpan
cursor untuk mengambil backlog pool baru tanpa melompati burst; kegagalan
registry menahan cursor. Jika sudah mengejar head, tail diperiksa ulang.
Pool sebelum bootstrap dan refresh seluruh kandidat lama belum tercakup.
Cakupan parsial selalu dilaporkan sebagai gap.

Maksimal 160 request API, jarak 2,1 detik antarpanggilan, deadline keseluruhan
12 menit, cooldown per sumber lima menit. Respons API maksimal 4 MiB dan
record tetap tunduk batas arsip 8 MiB. Kuota arsip 256 MiB menghentikan penulisan
baru, tidak menghapus bukti. Ini budget development, belum kapasitas VPS yang
diukur atau jadwal daemon. Jangan menafsirkan cooldown sebagai SLA freshness.

Prasyarat saat runtime kelak diuji: Linux/WSL/container untuk archive yang
memakai syscall Unix, Go sesuai go.mod, binary Spider pinned 2.53.9-p3.1 bagi
presale, serta akses publik DNS/TLS ke host terpilih. Tidak memerlukan token
Telegram, key trading, key OmniRoute atau database untuk P4-1/P4-2.

Gap penerimaan yang masih terbuka: crawl sumber nyata/robots, parsing sale
nyata, akses RPC/GoPlus/Binance dari host sasaran, ukuran exchangeInfo, latensi,
coverage/backfill dan seluruh tes adapter. Masing-masing harus memiliki bukti
baru sebelum dinyatakan diterima; batas tersebut tidak disembunyikan oleh
selesainya riset referensi atau penulisan kode.
