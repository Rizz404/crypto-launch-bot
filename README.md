# trading-bot — Presale dan listing baru

Proyek bot untuk menemukan dan memverifikasi informasi presale serta listing
baru di DEX/CEX, termasuk memecoin. Target akhirnya adalah analisis dengan AI
dan transaksi otomatis setelah validasi risiko.

**Arah baru ditetapkan 29 Oktober 2026. [Plan 4](docs/plan-4.md) adalah
satu-satunya roadmap aktif.** Discovery/verifikasi P4-2 sudah di-commit dan
kode analisis/policy P4-3 serta journal/adapter paper sintetis P4-4 ditulis.
Tes/build, integrasi nyata, evaluasi
model baru dan pilot belum dijalankan.

## Cakupan

- Presale mendatang/aktif dan listing maksimal 30 hari pada setiap venue.
- Coin lama yang baru listing tetap masuk; umur token dan umur listing dibedakan.
- Presale, DEX dan CEX termasuk target versi pertama, dibangun bertahap.
- Integrasi awal: BSC 56, PinkSale, PancakeSwap V2 dan Binance Spot USDT.
  Profil paper disepakati pada P4-3; pilot dan uang asli belum diaktifkan.

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
| [Analisis/policy](docs/presale-analysis.md) | Profil paper disepakati, AI, validator dan kuota |
| [Simulasi paper](docs/presale-paper.md) | Replay fixture tiga jalur dan journal virtual terisolasi |
| [Arsip](docs/archive.md) | Plan 1–3, runtime lama dan bukti historis |

Kode collector, adapter AI, ledger, paper simulator, dashboard dan Telegram
lama masih ada sebagai kandidat reuse. Keberadaan kode serta hasil pengujian
lama belum membuktikan kelayakan strategi presale/listing baru. CLI `launch`
baru mendukung collect/report, analisis AI eksplisit dan replay paper
sintetis, tanpa Telegram, wallet atau order live; izin entry riset tetap mati.
Perintah panduan arsip menjalankan komponen lama.

Pergantian arah tidak menghentikan eksperimen lama, memindahkan dana, atau
mengubah VPS. Target transaksi otomatis bukan izin transaksi uang asli.
