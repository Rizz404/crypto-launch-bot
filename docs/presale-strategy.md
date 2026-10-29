# Strategi presale dan listing baru

Rancangan aktif [Plan 4](plan-4.md), ditetapkan 29 Oktober 2026. Belum ada
klaim keunggulan atau target return yang terbukti untuk strategi ini.

## Universe dan identitas

- Presale mendatang atau aktif masuk discovery. Batas horizon masa depan
  ditentukan lewat riset sumber; bukan otomatis horizon event P3.
- Listing DEX/CEX masuk bila waktu mulai perdagangan terverifikasi berada
  dalam 30 × 24 jam terakhir, termasuk tepat di batas. Listing mendatang
  disimpan sebagai watchlist, tanpa entry sebelum venue dapat diperdagangkan.
- Umur token dihitung dari perdagangan publik pertama yang dapat diverifikasi;
  umur listing dihitung per venue/pool. Simpan keduanya terpisah dari first-seen
  bot. Waktu yang tidak diketahui tidak diasumsikan baru.
- Coin lama dengan listing baru tetap kandidat. Meme termasuk cakupan tanpa
  pengecualian pemeriksaan identitas dan risiko.
- Satu aset bisa punya banyak sale/listing. Kontrak lintas-chain tidak digabung
  hanya karena nama/ticker sama; pasangan CEX harus dipetakan dengan bukti.
- Sesudah 30 hari, hentikan entry baru dari listing tersebut. Posisi, dana
  terkunci, claim/refund dan kewajiban yang sudah ada tetap dipantau sampai selesai.

## Discovery sampai keputusan

1. Ambil pengumuman dan dokumen melalui Spider; lengkapi dengan API/feed dan
   observasi on-chain. Simpan sumber asli, waktu pengambilan dan revisi.
2. Gabungkan salinan informasi, verifikasi sale/listing dan identitas aset.
   Bedakan pernyataan promosi, bukti independen dan fakta yang belum diketahui.
3. Periksa syarat partisipasi, jadwal, tokenomics, distribusi/vesting, hak
   kontrak serta kondisi pasar; tampilkan konflik/gap secara eksplisit.
4. AI mengusulkan tolak, pantau, entry, hold atau exit dengan kutipan bukti,
   tesis, kontra-argumen, kondisi batal dan tenggat review. Untuk presale,
   usulan harus menyebut aksi yang dimaksud, bukan menyamakan kontribusi dengan BUY.
5. Validator deterministik memeriksa bukti terbaru, kelayakan aksi, policy,
   dana dan biaya. Informasi tidak cukup berarti tahan entry, bukan isi asumsi.
6. Pantau perubahan material dan kewajiban setelah entry. AI tidak memegang
   kredensial, menandatangani transaksi, atau mengubah limit.

## Lifecycle yang berbeda

| Jalur | Tahapan yang harus dapat dilacak |
|---|---|
| Presale | Pengumuman → verifikasi → kelayakan/whitelist → kontribusi → alokasi → menunggu TGE/claim → vesting/unlock → inventori tradable |
| Alternatif presale | Pembatalan → refund bila tersedia; kontribusi gagal, tidak mendapat alokasi, claim terlambat/gagal atau dana tidak dapat dipulihkan dicatat tersendiri |
| DEX | Pool/listing terverifikasi → quote/rute → izin token bila perlu → submit → konfirmasi/finality → settlement → monitoring/exit |
| CEX | Pengumuman → pasar terbuka → metadata/book valid → order/ack → partial/full fill atau cancel/reject → settlement → monitoring/exit |

Ini tahapan konseptual, bukan jaminan semua launchpad menyediakan refund atau
semua token bisa dijual. Aksi claim, unlock dan transfer tidak otomatis
menandakan profit terealisasi. Satu lifecycle menyimpan hasil tiap aksi dan
status dana; kegagalan tidak boleh ditampilkan sebagai selesai.

## Hold dan exit

Hold mengikuti tesis, kondisi pasar dan policy yang dibekukan, dengan evaluasi
ulang pada perubahan material. Jadwal acara selesai tidak otomatis mewajibkan
jual. Batas rugi/trailing, tenggat hold dan sizing dirancang pada P4-3, bukan
menyalin angka lama. Dana presale terkunci tidak dilindungi oleh stop loss
seperti inventori likuid. Jika exit tidak mungkin, tandai exposure dan insiden;
jangan mencatat penjualan fiktif. Rincian: [risiko](presale-risk.md).
