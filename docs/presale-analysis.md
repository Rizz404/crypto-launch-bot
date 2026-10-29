# Analisis AI dan policy paper — P4-3

**29 Oktober 2026.** User menyetujui profil paper dan kuota di bawah melalui
pertanyaan terstruktur, sesudah meminta commit P4-1/P4-2 lalu lanjut P4-3.
Commit sebelumnya **`108b3fa`**. Implementasi baru belum diuji/build atau
menjalankan inference; tidak mengaktifkan pilot atau uang asli.

## Policy yang dibekukan

Versi **`launch-paper-v1`**, metode **`launch-evaluation-v1`**.
Konfigurasi: [launch.policy.json](../configs/launch.policy.json). Kode menolak
perubahan parameter yang tidak sesuai policy versi ini; perubahan berikutnya
perlu persetujuan dan versi baru. Hash policy masuk ke packet/hasil analisis.

| Parameter | Nilai dan arti |
|---|---|
| Modal | 1.000 USDT virtual untuk satu buku paper bersama ketiga jalur; bukan 1.000 per kandidat |
| Exposure | Maksimal dua sekaligus, termasuk kontribusi terkunci dan reservasi |
| Presale/DEX | Budget kerugian skenario maksimal 1% modal per kandidat, menganggap seluruh nominal bisa hilang dan menyediakan ruang biaya |
| CEX | Nominal maksimal 10%, risiko rencana maksimal 0,5%; stop distance dan biaya harus diketahui dari input strategi deterministik, bukan keluaran AI |
| Halt harian | Pada penurunan equity ≥2% dari day-start WIB; reset harian hanya dari ledger, bukan model |
| Drawdown | Pada penurunan equity ≥8% dari peak sepanjang run; pergantian hari tidak mereset peak |
| Basis sizing | Nilai lebih kecil antara equity saat ini dan modal awal; keuntungan belum menaikkan plafon modal secara otomatis |
| AI | Maksimal 12 request/hari WIB, maksimal 2.048 output token/request; request gagal tetap memakai slot |
| Packet/keputusan | Input maksimal 48 KiB; maksimal delapan bukti dengan prefix teks 4.000 byte/bukti; keputusan kedaluwarsa setelah 300 detik |
| Review | Jendela dedup/review minimum 300 detik; analisis saat perubahan material tunduk kuota. Bukan janji 288 analisis/hari |

Model biaya tidak menganggap fee yang belum diketahui sebagai nol. Dengan
`B=min(equity, modal_awal)`, budget skenario presale/DEX adalah `B×1%`;
nominal dibatasi `budget/(1+cost_fraction)`. CEX dibatasi nilai lebih kecil
antara `B×10%` dan `(B×0,5%)/(stop_fraction+cost_fraction)`. Semua jalur juga
dibatasi kas tersedia setelah ruang biaya dan pembulatan ke bawah.

Contoh tanpa biaya hanya untuk menjelaskan rumus: plafon presale/DEX 10 USDT;
plafon CEX 100 USDT dengan stop 5% berarti risiko rencana 5 USDT. Biaya nyata
harus diketahui sebelum kalkulasi yang diterima; rugi gap/slippage di luar
asumsi bisa melebihi risiko rencana. Tidak ada jaminan stop dapat dieksekusi.

`PaperContext` harus eksplisit, fresh, dan memakai `day_wib` yang sesuai.
Kas tersedia tidak mencakup capital yang sudah committed. Locked/reserved
adalah bagian dari exposure yang sama, tidak dihitung sebagai posisi atau
uang tambahan. Exposure pada konteks v1 dinilai konservatif pada snapshot
yang sama dengan equity; jika basis nilai tidak konsisten, konteks ditahan.
Ini input riset, bukan ledger uang asli. P4-4 menambahkan journal sintetis
dan `mark_usdt/mark_known` yang membedakan cost basis dari valuasi. Unknown
menahan exposure baru; [kontrak paper](presale-paper.md) menjelaskan basisnya.

## Schema dan validator

Satu analisis memilih tepat satu kandidat dan mengikat `candidate_id`, revisi,
asset key, policy hash dan position ID bila ada. Aksi yang tersedia:
`REJECT`, `WATCH`, `PROPOSE_ENTRY`, `HOLD`, `PROPOSE_EXIT`,
`PROPOSE_CONTRIBUTE`, `PROPOSE_CLAIM`, `PROPOSE_REFUND`.

- Model memberi alasan, tesis, kontra-argumen, invalidation, hold rationale dan
  kutipan literal dari bukti yang benar-benar terlihat. Tidak ada field size,
  stop, risk override atau budget pada schema.
- Kontrak Structured Outputs memakai `json_schema`, `strict` dan menolak
  properti tambahan; validasi lokal tetap memeriksa field wajib/null, duplikasi,
  identitas, lifecycle, kutipan, freshness, exposure, consumed ID serta halt.
  Acuan protokol: [dokumentasi resmi OpenAI](https://developers.openai.com/api/docs/guides/structured-outputs?api-mode=chat).
  Dukungan gateway/model belum dibuktikan untuk prompt baru.
- Held/rejected tidak boleh menghasilkan proposal entry/kontribusi. Presale
  mendatang hanya WATCH. Claim/refund harus menunjuk sale dan exposure tepat,
  dengan status claimable/refundable eksplisit; tidak diturunkan dari tanggal.
- HOLD mempertahankan tesis dan invalidation posisi asli. Exit membutuhkan
  bagian inventori tradable yang tidak terreservasi. Sesudah partial claim,
  bagian yang sudah diterima boleh diteliti untuk exit, sementara sisa vesting
  tetap terkunci. Tidak menjual principal atau token yang belum diterima.
- Input dibangun ulang dari arsip sebelum inference. Sesudah respons, revisi
  dan kelayakan kandidat diperiksa lagi. Packet/model tidak dapat meningkatkan
  status verifikasi atau memasukkan evidence palsu.

Semua hasil **`entry_allowed=false`**. Bahkan proposal valid adalah hasil
riset, belum reservasi/fill/order. Gap P4-2 membuat presale/DEX v1 tetap held;
P4-3 tidak memalsukan verifikasi agar bisa menghasilkan entry.

## Alur dan kuota

`report/collect` tetap tanpa AI/credential. `analyze` adalah aksi eksplisit:
pilih kandidat dari arsip → cek policy/context → bangun packet → simpan input
→ reservasi dedup dan slot harian durable → panggil OmniRoute tanpa tools
→ decode/validasi → arsipkan hasil → tampilkan status.

Slot WIB dan marker disimpan sebelum HTTP. Error, timeout, output salah,
refusal/tool call, atau crash setelah reservasi tidak mengembalikan slot.
Tidak ada retry/failover otomatis. Dedup mengikat kandidat/revisi, policy,
model, state posisi/consumed serta jendela lima menit; timestamp receipt baru
atau parafrase tesis tidak memberi model hak membelanjakan ulang input sama.
Report membaca hasil lama dan menandai expired/superseded tanpa menulis ulang
bukti hasil asli. Default report tetap offline.

Kuota berlaku per arsip. Menyalin/mengganti arsip tidak menjadi cara menambah
kuota; gunakan satu arsip Plan 4. Jumlah request/token **bukan plafon uang**:
tarif gateway dan biaya layanan belum terverifikasi, input token dicatat,
dan aktivitas internal gateway berada di luar penghitung aplikasi.

## Metode simulasi dan evaluasi v1

Bekukan metode ini sebelum pilot; implementasi fill/ledger adalah P4-4.

| Jalur | Aturan penilaian |
|---|---|
| Presale | Catat kontribusi, alokasi, claim/vesting/refund sesuai bukti. Tanpa data akses/alokasi/terms andal, hasil observasi saja; tidak membuat fill atau return. Dana tetap committed sampai penyelesaian terbukti |
| DEX | Butuh quote/rute, jumlah, kondisi jual dan biaya/gas yang terverifikasi pada waktu keputusan. Candle CEX atau USD pool price tidak menghasilkan fill. Gagal/pending/revert dan gas tetap masuk hasil |
| CEX | Gunakan book fresh, spread/depth dan fee terverifikasi; batasi fill pada depth tersedia dan catat partial/reject, jangan memakai candle close sebagai harga pasti |

Untuk semua jalur, parameter strategi yang memasok stop/biaya, freshness
pra-submit, batas slippage/depth dan model finality harus dicatat serta
dibekukan pada adapter P4-4 sebelum ada simulasi fill prospektif. Jika belum
tersedia, simpan reason unknown dan tetap observasi. Jangan mengarang profit
atau reset halt untuk membuat strategi tampak berhasil.

Bandingkan AI dengan baseline tanpa filter AI pada kandidat, waktu, policy,
biaya dan kapasitas yang sama. Baseline memakai gate deterministik; ties
diurutkan first-seen lalu ID. Semua kandidat gagal/tidak selesai tetap masuk.
Sampel pengembangan dipisah dari evaluasi dan hanya data yang diketahui saat
keputusan boleh digunakan. Tidak ada target profit yang diasumsikan.

Penerimaan AI sebelum pilot: minimal sepuluh kasus berlabel per jalur yang
mencakup unknown/konflik, kondisi pasar buruk, lifecycle dan adversarial prompt;
tidak boleh ada pelanggaran validator/risiko, fabricated citation yang lolos,
atau perintah finansial yang dieksekusi. Kasus keluaran model invalid tetap
dilaporkan, bukan disembunyikan lewat retry. Inference nyata tetap perlu tugas
verifikasi tersendiri dan tunduk kuota; 30 kasus bukan izin 30 call sekaligus.

T0, periode pilot, jumlah peluang yang cukup serta jadwal review ditetapkan
sebelum P4-5. Persetujuan parameter P4-3 tidak mengaktifkan pilot. Tidak ada
Telegram, signer, order, claim/refund nyata, atau deployment pada fase ini.
