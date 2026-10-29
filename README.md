# trading-bot — Presale dan listing baru

Proyek bot untuk menemukan dan memverifikasi informasi presale serta listing
baru di DEX/CEX, termasuk memecoin. Target akhirnya adalah analisis dengan AI
dan transaksi otomatis setelah validasi risiko.

**Arah baru ditetapkan 29 Oktober 2026. [Plan 4](docs/plan-4.md) adalah
satu-satunya roadmap aktif.** Riset P4-1 dan kode development discovery/verifikasi
P4-2 sudah ditulis; tes/build serta integrasi nyata belum dijalankan.

## Cakupan

- Presale mendatang/aktif dan listing maksimal 30 hari pada setiap venue.
- Coin lama yang baru listing tetap masuk; umur token dan umur listing dibedakan.
- Presale, DEX dan CEX termasuk target versi pertama, dibangun bertahap.
- Integrasi awal: BSC 56, PinkSale, PancakeSwap V2 dan Binance Spot USDT.
  Modal dan batas risiko dipilih sebelum pilot; angka P3 tidak diwarisi otomatis.

## Alur yang dituju

Discovery → arsip/deduplikasi → verifikasi identitas dan informasi → AI dengan
konteks pasar/on-chain → validasi risiko → keputusan → eksekusi → monitoring/exit.

Go, Spider dan OmniRoute tetap menjadi fondasi. API/feed/on-chain melengkapi
crawl website untuk data terstruktur dan cepat. AI mengusulkan keputusan;
program memvalidasi batas dan melaksanakan aksi yang diizinkan.

## Mulai membaca

| Dokumen | Kegunaan |
|---|---|
| [Indeks docs](docs/readme.md) | Peta seluruh dokumentasi aktif |
| [Plan 4](docs/plan-4.md) | Tahapan, dependensi dan kriteria penerimaan |
| [Handover agent](docs/handover-agent.md) | Status, batas pekerjaan dan langkah berikutnya |
| [Integrasi awal](docs/presale-integrations.md) | Hasil riset sumber resmi, pilihan dan gap |
| [Discovery/report](docs/presale-runtime.md) | CLI development, kontrak laporan dan batasnya |
| [Arsip](docs/archive.md) | Plan 1–3, runtime lama dan bukti historis |

Kode collector, adapter AI, ledger, paper simulator, dashboard dan Telegram
lama masih ada sebagai kandidat reuse. Keberadaan kode serta hasil pengujian
lama belum membuktikan kelayakan strategi presale/listing baru. CLI `launch`
baru hanya mengumpulkan/melaporkan riset, tanpa AI, Telegram, wallet atau order.
Perintah panduan arsip menjalankan komponen lama.

Pergantian arah tidak menghentikan eksperimen lama, memindahkan dana, atau
mengubah VPS. Target transaksi otomatis bukan izin transaksi uang asli.
