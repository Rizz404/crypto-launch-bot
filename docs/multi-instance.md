> **ARSIP — 29 Oktober 2026.** Dokumen ini merekam proyek Plan 1–3.
> Roadmap aktif: [Plan 4](plan-4.md); arahan terkini: [handover agent](handover-agent.md).
> Pernyataan aktif, izin, jadwal, parameter dan instruksi resume di bawah berlaku
> pada checkpoint historisnya, bukan arahan melanjutkan pilot P3 atau transaksi.
> Pengarsipan tidak menghentikan proses lama. Lihat [indeks arsip](archive.md).

---

# Multi-instance

Model modal dan ownership Binance Spot yang dibangun pada [Plan 2](plan-2.md).
Kontrak penggunaan ada di [runtime managed](managed-runtime.md); penerimaan dan
backlog aktif di [Plan 3](plan-3.md). Strategi AI harus menjaga invarian yang
sama, dengan modal/ledger paper terpisah dari eksperimen pembanding.

## Struktur dan identitas

Satu akun/mode memiliki satu koordinator wallet. Wallet menyediakan cadangan
modal; instance menerima alokasi; satu atau beberapa binding strategi berbagi
pool instance. Instance adalah portofolio virtual, bukan container atau subakun.

| Entitas | Identitas dan batas |
|---|---|
| Wallet | Akun + mode + market + aset pendanaan; Spot/Futures terpisah |
| Instance | ID stabil, nama, lifecycle, alokasi dan risk |
| Binding | Strategi, parameter, simbol, ownership dan cap opsional |
| Run | Riwayat eksekusi; restart membuat run, tanpa modal baru |
| Order/fill/holding | Pemilik binding serta ID exchange/dedup |
| Jurnal/reservasi | Perubahan dana, penguncian aset dan recovery |

Implementasi memakai agregat JSONB/jurnal, bukan tabel SQL per entitas.
`strategy_runs.instance_id` legacy bukan ID instance baru; jangan mengganti arti
atau mengadopsi run hanya karena paling baru. Secret disimpan di luar ledger.

Contoh pembagian satu modal Rp2 juta: A Rp1 juta, B Rp600 ribu, cadangan
Rp400 ribu. Itu ilustrasi produk, bukan rekomendasi alokasi. Nilai otoritatif
memakai decimal/aset asli; USD/IDR hanya konversi bertanggal. Saldo pribadi di
exchange dan keuntungan paper tidak otomatis menjadi modal riil bot.

## Invarian dana

1. Alokasi dan transfer adalah debit/kredit atomik dengan idempotency key.
   Request ulang tidak boleh mengulang kredit.
2. Nilai bot = cadangan + equity semua instance, disesuaikan arus eksternal.
   Modal awal, biaya pokok, cash bebas, reserved, unrealized dan realized harus
   dibedakan; jangan memasukkan profit dua kali atau menjumlahkan view engine.
3. Cap strategi bukan saldo tambahan. Sizing memakai dana instance; pemeriksaan
   global tetap menghitung seluruh binding dan konsentrasi simbol bersama.
4. Transfer hanya tunai bebas yang didukung wallet; inventori, unrealized profit
   dan dana reserved belum bisa dipindah. Setoran/transfer bukan return.
5. Buy mengunci quote + buffer fee; sell hanya base milik binding, tanpa short
   Spot atau inventori pribadi/instance lain. Fee base/quote/BNB dan dust tetap
   tercatat, tidak dihapus untuk memalsukan status flat.
6. OCO mengunci satu inventori untuk dua leg alternatif. Partial fill mengubah
   sisa reservasi; request cancel belum berarti dana bebas.
7. Kapasitas wallet fisik menjadi batas tambahan. Transaksi manual, deposit dan
   withdrawal memerlukan rekonsiliasi; selisih tidak dibagi sembarang.

## Eksekusi dan recovery

Satu pemilik tulis aktif per wallet/mode, dilindungi lease/fencing. Koordinator
berbagi feed dan rate limit, merutekan event ke binding, lalu menjalankan:

1. Validasi scope, lifecycle, harga/instrumen dan risk bertingkat.
2. Simpan intent/order dan reservasi dengan ID deterministik sebelum I/O.
3. Kirim request; respons ambigu diperiksa statusnya sebelum retry.
4. Terapkan fill/fee sekali, perbarui posisi, jurnal dan durable inbox.
5. Lepas sisa reservasi setelah konfirmasi terminal; simpan checkpoint.

Transaksi DB tidak membuat request exchange atomik. Restart memulihkan inbox,
checkpoint, ownership, proteksi/trailing dan order sebelum entry. Selisih saldo
menahan entry; pengurangan risiko hanya lanjut bila dapat ditentukan aman.

Order tidak dinetting antarstrategi. Dua binding boleh memegang ETH, tetapi
masing-masing hanya menjual bagiannya dan jumlahnya tetap dibatasi wallet.
Minimum order/self-trade prevention tidak boleh diakali dengan memindahkan
ownership. Futures/unified wallet/multi-leg memerlukan desain tersendiri.

## Risiko, kontrol dan laporan

Limit strategi → instance → akun semuanya berlaku; child tidak bisa melonggarkan
parent. Risiko pause/kill/flatten bersifat scoped, dengan ownership tetap dijaga.

| Aksi | Kontrak |
|---|---|
| Pause | Tahan entry; exit/proteksi/reconciliation tetap berjalan |
| Cancel | Batalkan pending sesuai scope, tanpa menjual inventori |
| Flatten | Cancel order berkonflik, tutup inventori sendiri, tunggu ack |
| Stop proses | Simpan state untuk resume; order/koin dapat tetap ada |
| Finish | Pilih retain/flatten, selesaikan status order, snapshot dan backup |
| Archive | Order selesai dan posisi kosong termasuk dust; cash ke reserve |

Gagal/ambigu tetap closing/attention, bukan selesai. Scope akun hanya mengelola
aset bot, bukan order/inventori pribadi. Kontrol web/Telegram memeriksa otorisasi,
konfirmasi dan scope stabil. Clone menyalin konfigurasi tanpa dana/history.

Laporan akun/instance/binding harus memperlihatkan alokasi, free/reserved,
inventori, PnL/fee, exposure, drawdown, order dan transfer tanpa modal ganda.
Grafik ringkas tidak menggantikan arsip. Alasan entry/exit/menunggu harus jelas.

## Migrasi dan penerimaan

Uji DB salinan dahulu. Legacy masuk status review; ownership/dust/order ambigu
memerlukan pemetaan eksplisit. Jangan menganggap 10.000 USDT testnet sebagai
modal riil Rp2 juta. Simpan riwayat dan batas rollback.

[Penerimaan historis Plan 2](plan-2.md#p2-7--testing-menyeluruh) mencakup transaksi
serentak, dua pemilik koin sama, partial/replay/cancel ambigu/crash, risk scoped,
lease/restart, fee/arus eksternal, serta backtest satu modal bersama. Hasil akun
simulasi Rp2 juta per kombinasi tidak boleh dijumlahkan sebagai satu portofolio.
Penerimaan lanjutan mengikuti [P3-O](plan-3.md#p3-o--penerimaan-operasi-dan-tindak-lanjut-audit)
dan fase P3-3–P3-6; hasil historis tidak otomatis membuktikan integrasi AI.
