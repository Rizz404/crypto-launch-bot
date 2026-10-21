# Audit awal transaksi dan akuntansi

Audit historis **20 Oktober 2026**, tahap P2-0. Dua fix reconciliation lokal dan
regression tersedia saat audit; kelanjutan implementasinya ada di [Plan 2](../plan-2.md).
Laporan ini bukan daftar bug terkini atau izin live.

## Bukti dan metode

Baca [penutupan testnet](testnet-stop/analysis.md), [evidence](testnet-stop/evidence.json),
[supplement](testnet-stop/supplement.json), [klasifikasi error](p2-0-exchange-errors.json)
dan [hasil audit](p2-0-audit.json). Log container stopped diperiksa read-only;
raw error/URL/secret tidak disalin. Perhitungan memakai decimal dan checksum,
tanpa menimpa bukti atau mengubah DB/exchange.

Reproduksi saat diminta (stdout saja):

```sh
python3 scripts/audit-testnet.py --exchange-errors docs/testing/p2-0-exchange-errors.json
```

## Temuan dan perbaikan

| Area | Bukti | Hasil audit |
|---|---|---|
| C-01 OCO ETH | Dua HTTP 400/-2010, hubungan harga order tidak valid, 20 Okt 08:30:15 dan 09:32:26 UTC | Bukan bukti kurang saldo; harga berubah antara validasi lokal/request adalah hipotesis, tanpa rekonstruksi tick |
| C-01 cancel | 23 -2011 unknown order | Perlu resync status final; error tidak boleh langsung dianggap cancel sukses |
| C-01 proteksi BTC | Satu pasangan stop/target expired tanpa fill | Fallback market exit diperlukan |
| C-02 dust DOGE | Fill 10.192 buy − 10.104 sell = 88 DOGE, ledger run 9 hanya 83 | Lima reconciliation masing-masing membuang 1 DOGE karena gagal minimum order |
| C-02 equity transient | Kuantitas kandidat dipakai menghitung cost sebelum perubahan ledger dikonfirmasi | Cost/PnL berasal dari dua versi posisi, menghasilkan equity dip palsu |

**Proteksi OCO.** `TestOCOPriceRelationshipRejectionClosesOnce` mereproduksi
-2010 pada bracket awal dan penggantian trailing: dua leg rejected, cancel
bracket lama bila ada, tepat satu market exit. Test lama mencakup satu leg
expired, kedua leg expired dan bracket parsial gagal. Audit ini belum mengurangi
penolakan saat harga berubah atau menyelesaikan semua cancel ambigu; jangan
retry tanpa mengetahui status exchange.

**Dust.** Kode lama memakai `CheckOrder(entry_price, qty)` dan pembulatan step
untuk menilai apakah aset masih ada. Satu DOGE ±0,08 USDT gagal minimum order
5 USDT lalu hilang setelah dua reconciliation meskipun wallet masih memegangnya.
Fix mempertahankan kuantitas wallet persis, dibatasi aset milik bot, dan hanya
menghapus posisi bila wallet benar-benar tidak positif. Minimum/step tetap
berlaku pada order, bukan kepemilikan aset. `TestReconcileSpotKeepsDustAndSubStepHoldings`
mencakup 1 dan 0,25 DOGE, identitas, cost dan mark setelah reconciliation berulang.
Dust tetap dapat tidak terjual; jangan menampilkan flat hanya karena terlalu kecil.

**Equity transient.** Posisi BTC 0,00025 @80.000, mark 90.000, quote 10.000;
sampel wallet 0,0002 belum terkonfirmasi membuat balance lama 10.016, padahal
ledger yang dipertahankan memerlukan 10.020. Setelah konfirmasi, 0,0002/10.016
baru konsisten. Fix menghitung cost dari posisi setelah `reconcilePositions`.
`TestReconcileSpotValuesThePositionActuallyRetained` mencakup dua tahap dan
mismatch yang pulih. Snapshot wallet/fill belum menjadi atomik karena fix ini.

## Batas rekonstruksi PnL

| Ukuran | USDT |
|---|---:|
| Total trade net tersimpan | 79,9042839866754051 |
| Balance pertama tersimpan | 10.000,042202 |
| Balance akhir | 10.079,5518598139792050792 |
| Trade net − perubahan balance | 0,3946261726962000208 |

Penghapusan dust konsisten dengan mekanisme drift, tetapi biaya pokok setiap
kejadian belum cukup untuk rekonstruksi penuh. Mark candle terakhir dan engine
intra-menit berbeda waktu. Jangan menganggap seluruh selisih sudah dijelaskan.
Hitung inventori run 9 saja; snapshot run 7/8 bukan aset tambahan. Fee testnet
nol; net fill bukan rumus lengkap live dengan fee base/arus eksternal.

## Kontrak penutupan dan DCA

`/stop` menyimpan state untuk resume, berbeda dari finish. Tujuh cancel exchange
20 Oktober terjadi sesudah recorder mati sehingga DB historis masih `new`;
bukti exchange adalah status final, bukan alasan mengedit DB lama.

Finish harus menahan entry, menyelesaikan order unknown, memilih retain/flatten,
memproses cancel/fill sampai terminal selama recorder aktif, lalu snapshot saldo/
order, flush, tutup run dan atur timer/backup. Gagal/ambigu tetap closing/attention.
Dust tidak boleh dibuang untuk membuat flat. Ini kontrak audit; implementasi
lanjutan diperiksa di [runtime](../managed-runtime.md).

DCA trial memang tanpa stop loss dan memakai raw intent; `Unprotected()` benar
mendeteksi ketiadaan native stop. ERROR saat startup perlu policy/tingkat pesan
sesuai strategi, berbeda dari OCO ETH ditolak. Jangan menyembunyikan kegagalan
proteksi wajib atau menganggap DCA tanpa stop aman karena desainnya disengaja.

## Validasi historis dan batasnya

Regression dust/sub-step serta equity transient gagal sebelum fix dan lulus
sesudahnya. Sesi audit mencatat race suite, test OCO/bot/engine, vet, build dan
lint scope terkait lulus. Storage integration, migrasi up/down/up dan restart
paper diuji pada DB sementara `tradingbot_p20_audit_test`, terpisah dari trial.

DB sementara dihapus; DB development kembali stopped. Bot/DB ETH tetap mati,
marker larangan laptop dipertahankan. Tidak ada deploy, restart VPS atau transaksi
exchange dari audit; log hanya dibaca. Pemeriksaan tersebut tidak dijalankan
ulang saat perapian dokumentasi, dan audit bukan kelulusan seluruh Plan 2.
