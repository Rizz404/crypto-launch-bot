# Sumber data dan pemilihan integrasi

Protokol riset P4-1 pada [Plan 4](plan-4.md), 29 Oktober 2026.
**Belum ada katalog provider, tarif atau endpoint baru yang diverifikasi dalam
perombakan docs ini.** Chain, launchpad, DEX dan CEX awal belum dipilih.

## Kebutuhan sumber

| Kategori | Informasi yang dicari | Peran |
|---|---|---|
| Website proyek/launchpad resmi | Sale, syarat, jadwal, alokasi, kontrak, claim/refund, vesting dan revisi | Spider dan dokumen primer; klaim proyek tetap perlu pemeriksaan |
| Pengumuman dan API resmi CEX | Listing, waktu pasar terbuka, mapping aset, metadata, book, order/fill | Discovery listing dan adapter pasar/eksekusi |
| Factory/pool/program DEX dan API resmi | Pembuatan pool, awal perdagangan, rute, reserve/likuiditas dan transaksi | Discovery dan verifikasi kondisi eksekusi |
| RPC/explorer/data on-chain | Network/kontrak, bytecode/authority, distribusi, izin, status transaksi/finality | Verifikasi independen terhadap klaim pemasaran |
| Agregator/feed/berita/komunitas | Lead discovery dan konteks | Tidak menjadi satu-satunya bukti identitas atau izin entry |

Website memberi konteks; API/feed/on-chain memberi observasi terstruktur dan
cepat. Keberadaan halaman presale atau pengumuman bukan bukti dana aman atau
likuiditas tersedia. Ikuti akses yang diizinkan, robots dan rate limit;
jangan mengganti Spider diam-diam untuk melewati kegagalan crawl.

## Prosedur riset dan keputusan

1. Temukan kandidat dari sumber resmi; cocokkan chain/network, pemilik domain,
   dokumentasi API dan kontrak/program. Baca dokumentasi terkini pada saat riset.
2. Isi matriks berikut untuk setiap integrasi, termasuk pilihan yang ditolak.
   Pisahkan fakta terverifikasi dari asumsi dan uji yang belum dilakukan.
3. Bandingkan cakupan presale/meme/listing, mutu identitas, latensi/riwayat,
   kemampuan eksekusi/rekonsiliasi, kelayakan akses dan total biaya.
4. Pilih kombinasi awal yang mencakup presale, DEX dan CEX, dengan urutan
   implementasi dan gap tertulis. Jangan menjanjikan semua chain/exchange.
5. Bekukan freshness, polling/subscription, finality, fallback dan kuota menurut
   kebutuhan integrasi terpilih; minta keputusan user jika biaya atau akses
   membutuhkan komitmen baru.

## Template hasil riset

| Field | Wajib dicatat |
|---|---|
| Identitas | Nama provider/venue, URL dokumentasi resmi, chain/network dan jenis jalur |
| Provenance | Tanggal/waktu verifikasi, metode inspeksi dan referensi bukti |
| Cakupan | Sale/listing yang tercakup, histori, kontrak/pool ID, jadwal/revisi dan known gaps |
| Akses | Auth, syarat partisipasi/wilayah, robots/ToS, rate limit, pagination, stream dan retensi |
| Biaya | Tarif beserta tanggal/sumber, kuota gratis, estimasi biaya pada beban sasaran; unknown bila belum jelas |
| Eksekusi | Kontribusi/claim/refund atau order/swap, status/finality, sandbox dan batas simulasi |
| Operasi | Timeout/outage, reorg/koreksi, fallback, kebutuhan RPC/storage dan risiko ketergantungan |
| Keputusan | Pilih/tolak/tunda, alasan, prioritas implementasi, pemilik gap dan kriteria verifikasi ulang |

Jangan menaruh credential dalam matriks. Kebutuhan akses berbayar atau signer
dicatat sebagai prasyarat, bukan dianggap tersedia dari env proyek lama.
