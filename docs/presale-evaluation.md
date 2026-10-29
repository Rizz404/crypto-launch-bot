# Evaluasi dan penerimaan

Rencana pengujian untuk [Plan 4](plan-4.md), 29 Oktober 2026. Semua skenario
di sini **belum dijalankan untuk strategi baru**. Pekerjaan docs hanya memakai
review statis. [Bukti lama](testing/readme.md) tetap historis. P4-3 membekukan
[metode `launch-evaluation-v1`](presale-analysis.md#metode-simulasi-dan-evaluasi-v1)
dan profil paper; model fill/ledger baru diimplementasikan pada P4-4.

## Metode dan metrik

Bekukan universe, sumber, policy, biaya, waktu keputusan dan metode penilaian
sebelum pilot. Simpan data sebagaimana diketahui bot pada waktu itu; jangan
menggunakan revisi, listing atau harga masa depan dalam replay. Masukkan
kandidat gagal, rug pull, tidak listing dan hasil yang belum selesai agar
laporan tidak hanya berisi pemenang. Pisahkan sampel pengembangan dan evaluasi.

| Jalur | Simulasi dan batas interpretasi |
|---|---|
| Presale | Syarat partisipasi, kontribusi/alokasi, lock/claim/vesting/refund dan biaya; alokasi/akses tak diketahui tidak dianggap fill. Tanpa data andal, laporkan observasi saja |
| DEX | Quote/rute, likuiditas/price impact, slippage, gas/gagal, transfer tax dan finality; replay candle CEX tidak mewakili AMM atau kondisi eksekusi |
| CEX | Book aktual, spread/depth, partial fill, fee dan status order; harga candle/announcement tidak menjamin fill saat listing |

Laporkan per jalur: jumlah ditemukan/terverifikasi/ditolak, alasan dan gap,
latensi deteksi, freshness, error/outage, keputusan AI dan invalid output,
biaya layanan/AI, fill/reject/gagal, dana tersedia/terkunci, biaya transaksi,
PnL terealisasi serta valuasi belum terealisasi dengan sumber dan asumsi.
Catat drawdown dan exposure tidak likuid; jangan menghitung harga indikatif
token tak bisa dijual sebagai profit terealisasi. Bandingkan dengan baseline
tanpa AI pada universe dan biaya yang sama bila datanya memungkinkan.

## Skenario wajib saat implementasi

- Batas umur listing tepat 30 hari/lewat batas; listing mendatang, waktu
  unknown, timezone, coin lama baru listing, pool/venue berbeda dan ticker kembar.
- Duplikasi/revisi berita, jadwal berubah/dibatalkan, sumber konflik/basi,
  kontrak palsu, parsing gagal serta konten yang mencoba memberi instruksi AI.
- Mint/freeze/upgrade, holder terkonsentrasi, likuiditas hilang, jual terblokir,
  tax/fee/gas/slippage melampaui limit dan data risiko unknown menahan entry.
- Presale gagal/tidak mendapat alokasi, dana terkunci, refund tidak tersedia,
  claim/vesting parsial, tenggat terlewat dan peluncuran tertunda.
- Order/swap partial atau gagal, timeout setelah submit, restart, duplicate
  event, pending lama, reorg/finality, claim ulang dan rekonsiliasi idempoten.
- AI/RPC/feed/DB/signer outage, budget habis, halt, recovery dan pemisahan
  ledger/modal lama; proteksi tidak bergantung respons AI.

## Gerbang penerimaan

1. **Integrasi:** pilihan P4-1 dan gap akses terdokumentasi untuk ketiga jalur.
2. **Lokal/contract:** uji relevan atas kode baru dan adapter lulus dengan
   bukti tersendiri; fixture diberi label sintetis dan tidak menjadi klaim live.
3. **Pilot:** user menyetujui policy, modal virtual, budget, kapasitas, T0,
   durasi, kriteria stop/review dan metode simulasi masing-masing jalur.
   Pelaporan observasi tidak boleh diberi label paper fill yang terbukti.
4. **Review:** hasil prospektif menyertakan semua jalur, biaya, kegagalan dan
   batas ketidakpastian. Jalur tanpa bukti memadai belum lulus.
5. **Live:** temuan kritis selesai, signer/limit/backup/recovery/insiden siap,
   dan ada izin eksplisit user untuk integrasi serta nominal konkret.

Tidak ada target keuntungan atau ambang penerimaan numerik yang diasumsikan.
Nilainya harus dibekukan sebelum pilot, bukan dipilih setelah melihat hasil.
Penyimpanan laporan Plan 4 harus terpisah dari payload historis; jangan
menjalankan script lama untuk menimpa bukti.
