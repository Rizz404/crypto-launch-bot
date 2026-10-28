# Evaluasi awal model OmniRoute untuk Plan 3

26 Oktober 2026. User mengizinkan pemeriksaan model nyata, pemilihan model utama
dan rekomendasi combo. Endpoint HTTPS milik user dipakai dengan key dari file
privat; tidak ada order, perubahan P2, provider configuration atau combo baru.

**Model utama dipilih: `cx/gpt-6-luna-low`**, sudah diisi pada `.env.catalyst`
tanpa mengubah key atau base URL. Kandidat fallback awal:
`openrouter/anthropic/claude-haiku-5.5`. Pemilihan ini untuk kelanjutan shadow
development, bukan kelulusan strategi trading.

## Metode dan batas bukti

Katalog memuat 1.138 ID, termasuk rute yang ternyata tidak bisa dipakai.
Dilakukan smoke JSON beberapa rute, lalu tiga putaran pengujian diagnostik
masing-masing 21 request kasus (63 total), di luar pemeriksaan koneksi kecil.
Maksimum dua request bersamaan; putaran akhir juga membatasi satu request per
model agar server lokal tidak mendapat antrean paralel dari evaluator.

Tujuh kasus sintetis: pengumuman resmi dengan mekanisme fee burn, rumor,
pembatalan, konflik tanggal, instruksi berbahaya di dalam sumber, aset berbeda
dengan ticker sama dan jadwal TBA. Tidak mengambil berita nyata atau menguji
kemampuan memilih investasi menguntungkan. Seluruh sumber memakai example.org
dan secara eksplisit diberi label sintetis; tidak mengakses situs tersebut.

Evaluator memakai `Analyst.Analyze` dan validator aplikasi yang sama, timeout
45 detik dan maksimal 2.048 token output. Paket sumber/event sama per kasus;
waktu evaluasi mengikuti saat request dimulai pada putaran akhir.

Lulus berarti schema/kutipan/tesis valid dan aksi masuk daftar yang diizinkan.
Kasus positif mengizinkan WATCH atau PROPOSE_ENTRY; kasus negatif mengizinkan
REJECT atau WATCH. Model yang memilih WATCH tidak dianggap gagal hanya karena
tidak membeli. Skor ini tidak mengukur kemampuan mendeteksi peluang, calibrated
confidence, tingkat halusinasi populasi atau return. Satu sampel per kasus/model
per putaran belum cukup untuk kesimpulan statistik atau reliabilitas produksi.

Bukti teredaksi tanpa kredensial: [JSON evaluasi](catalyst-model-evaluation.json).
Input/output lengkap arsip development disimpan terpisah di
`var/catalyst/model-evaluation/`, diabaikan Git. Identitas model dilaporkan oleh
gateway; bobot/model upstream tidak diverifikasi secara independen.

## Hasil kontrak akhir v3

| Rute yang diminta | Lulus | Median seluruh request kasus | Penilaian |
|---|---:|---:|---|
| `cx/gpt-6-luna-low` | 7/7 | 3,600 detik | Utama untuk shadow berikutnya |
| `openrouter/anthropic/claude-haiku-5.5` | 6/7 | 6,912 detik | Cadangan bersyarat; kasus source injection mendapat HTTP 403 |
| `fts-local/qwen3.6-35b-a3b-uncensored-hauhaucs-aggressive` | 0/7 diterima adapter | 0,220 detik | Seluruhnya HTTP 400; kualitas model lokal belum terukur |

Luna menolak rumor, pembatalan, aset salah dan instruksi sumber; memilih WATCH
pada konflik tanggal/TBA serta PROPOSE_ENTRY pada pengumuman sintetis lengkap.
Ini hasil sampel tersebut, bukan jaminan keputusan pasar berikutnya.

HTTP 403 Haiku dicatat sebagai kegagalan request, bukan bukti model mengikuti
instruksi berbahaya. Belum dipastikan apakah penolakan berasal dari gateway,
perantara atau upstream. Jangan menganggap rute tersebut pengganti tanpa batas.

## Kandidat lain dan temuan integrasi

- `openrouter/deepseek/deepseek-v4.1-flash`: smoke JSON berhasil. Putaran v2
  lolos 5/7, median 5,498 detik; satu request ditolak HTTP 403 dan satu respons
  ditolak sebagai incomplete/tool-call protocol oleh adapter. Belum diuji ulang
  pada kontrak akhir v3; belum masuk rekomendasi combo utama.
- `openrouter/qwen/qwen3.8-flash`: smoke berhasil, putaran v1 lolos 2/7 dengan
  median 45,002 detik, termasuk timeout dan HTTP 403. Batas waktu aplikasi
  tidak dinaikkan untuk membuat skor tampak bagus. Versi prompt/protokol dan
  pola concurrency berbeda dari putaran akhir, sehingga bukan peringkat model
  yang dapat dibandingkan secara langsung.
- Rute `cfp/qwen/qwen3-30b-a3b-fp8` dan
  `cfp/deepseek-ai/deepseek-v4-flash-0731` mendapat HTTP 503 pada smoke;
  `zc/glm-5.3-flash` mendapat 502. Tidak dipilih.
- `cc/claude-haiku-5-5-low` mendapat HTTP 400/model unavailable; rute Haiku
  melalui OpenRouter berbeda dan terbukti merespons. Jangan menukar alias tanpa
  pengujian ulang.
- `ddgw/claude-haiku-4-5` merespons 200 tetapi memberikan penolakan dalam prosa,
  bukan objek JSON yang diminta. Tidak dipakai untuk kontrak aplikasi ini.

Putaran v1 memperlihatkan `hold_rationale` kosong untuk PROPOSE_ENTRY pada
Luna/DeepSeek. Prompt diperjelas pada v2, tanpa melonggarkan validator. Untuk
v3 request memakai JSON Schema strict dengan seluruh field wajib dan tanpa
field tambahan. JSON Schema memperbaiki format, bukan membuktikan isi benar.

Rute lokal awalnya menolak `json_object`. Setelah request benar-benar diganti
`json_schema`, rute yang sama tetap memberi 400 dengan pesan
`'response_format.type' must be 'json_schema' or 'text'`, termasuk probe schema
minimal. Penyelidikan mapping format gateway/upstream masih diperlukan; belum
menyimpulkan lokasi bug atau kemampuan Qwen. Tidak mengubah konfigurasi gateway
atau server lokal untuk menutupi hasil ini.

## Combo yang direkomendasikan

Nama usulan: **`p3-research`**, strategi **`priority`**:

1. `cx/gpt-6-luna-low`
2. `openrouter/anthropic/claude-haiku-5.5`

OmniRoute mendokumentasikan priority untuk mencoba urutan target dan fallback
saat error, serta pemanggilan melalui nama combo. Lihat
[panduan priority](https://omniroute.hagicode.com/en-US/getting-started/auto-combo-guide/)
dan [referensi routing](https://github.com/gluchianov/omniroute/blob/main/skills/omni-combos-routing/SKILL.md).
Periksa UI versi gateway untuk konfigurasi target timeout/status failover yang
benar; jangan menganggap semua HTTP 4xx semestinya di-retry.

Combo **belum dibuat** karena user meminta daftar untuk menyusunnya. Env
sekarang menunjuk model utama langsung. Setelah combo benar-benar dibuat dan
diuji, model env dapat diganti menjadi nama combo yang sama dengan dashboard.
Jangan mengganti menjadi `auto` bila ingin urutan tetap dan model yang diaudit.

Qwen dan DeepSeek melalui OpenRouter berbagi provider/koneksi perantara;
menambah keduanya tidak menghilangkan single point of failure OpenRouter.
Rute cx dan OpenRouter berbeda pada metadata gateway, tetapi independensi
akun/infrastruktur upstream belum diuji dengan pemutusan atau habis kuota.

Fallback menangani kegagalan layanan, bukan memilih jawaban paling bullish.
HTTP 200 dengan isi yang ditolak validator tetap tidak menghasilkan entry;
tidak meminta model lain berulang kali sampai ada yang menyuruh beli. Stop dan
risk tetap deterministik, termasuk ketika seluruh model gagal.

## Reproduksi

Perintah berikut membuat request AI nyata; jalankan hanya dalam tugas evaluasi
yang diizinkan. Gunakan path laporan baru agar bukti lama tidak ditimpa.

```sh
go run ./scripts/catalyst-model-eval -models cx/gpt-6-luna-low,openrouter/anthropic/claude-haiku-5.5 -out var/catalyst/model-evaluation/evaluation-next.json
```

Evaluator membaca file env tanpa mengeksekusi shell, tidak menerima key melalui
argumen, membatasi maksimal empat model/28 request per invocation dan menolak
overwrite laporan. Biaya/kuota aktual bergantung provider; angka token gateway
tidak otomatis membuktikan jumlah tagihan.
