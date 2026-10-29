# trading-bot — Presale dan listing baru

Proyek bot untuk menemukan dan memverifikasi informasi presale serta listing
baru di DEX/CEX, termasuk memecoin. Target akhirnya adalah analisis dengan AI
dan transaksi otomatis setelah validasi risiko.

**Arah baru ditetapkan 29 Oktober 2026. [Plan 4](docs/plan-4.md) adalah
satu-satunya roadmap aktif.** Perubahan ini dokumentasi saja: integrasi
presale/DEX dan strategi baru belum diimplementasikan atau diuji.

## Cakupan

- Presale mendatang/aktif dan listing maksimal 30 hari pada setiap venue.
- Coin lama yang baru listing tetap masuk; umur token dan umur listing dibedakan.
- Presale, DEX dan CEX termasuk target versi pertama, dibangun bertahap.
- Chain, launchpad, venue, modal, batas risiko dan budget dipilih melalui riset
  sebelum implementasi/pilot terkait; angka Plan 3 tidak diwarisi otomatis.

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
| [Arsip](docs/archive.md) | Plan 1–3, runtime lama dan bukti historis |

Kode collector, adapter AI, ledger, paper simulator, dashboard dan Telegram
lama masih ada sebagai kandidat reuse. Keberadaan kode serta hasil pengujian
lama belum membuktikan kelayakan strategi presale/listing baru. Tidak ada
quickstart runtime Plan 4; perintah pada panduan arsip menjalankan komponen lama.

Pergantian arah tidak menghentikan eksperimen lama, memindahkan dana, atau
mengubah VPS. Target transaksi otomatis bukan izin transaksi uang asli.
