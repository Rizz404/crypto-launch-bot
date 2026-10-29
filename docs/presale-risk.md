# Risiko dan gate transaksi

Rancangan [Plan 4](plan-4.md), 29 Oktober 2026. Modal, sizing, exposure,
stop/trailing, halt, freshness dan budget harus dirancang ulang pada P4-3.
Belum ada policy numerik atau izin live Plan 4. Profil P3 tidak berlaku otomatis.

## Pemeriksaan sebelum entry

| Area | Pemeriksaan yang harus dirancang untuk integrasi terpilih |
|---|---|
| Identitas | Network/kontrak/mint, sale/pool/venue dan penerima dana cocok dengan bukti; cegah ticker palsu dan kontrak tiruan |
| Kontrak/authority | Kemampuan mint/freeze/blacklist/pause, upgrade/admin, transfer tax, pembatasan jual dan perubahan hak; audit bukan jaminan |
| Distribusi | Konsentrasi holder, kepemilikan terkait bila diketahui, vesting/unlock dan ketidaklengkapan klasifikasi wallet |
| Presale | Syarat kelayakan, penerima kontribusi, alokasi, oversubscription, pembatalan/refund, jadwal claim serta lamanya dana terkunci |
| DEX | Rute, likuiditas aktual dan potensi penarikan, kemampuan menjual, price impact, slippage, gas/approval serta risiko urutan transaksi |
| CEX | Pasar benar-benar terbuka, batas order, book/spread/depth, saldo, fee serta status deposit/withdrawal bila dibutuhkan |
| Bukti | Freshness, konflik sumber, gap kritis, koreksi/reorg dan expiry keputusan; periksa ulang tepat sebelum I/O |

Pemeriksaan harus melaporkan pass/fail/unknown dengan bukti dan waktu.
Unknown pada syarat entry wajib menahan transaksi. Simulasi keberhasilan swap
atau penjualan bukan jaminan kondisi itu tetap berlaku saat transaksi nyata.

## Exposure dan proteksi

- Pisahkan kas tersedia, reservasi, kontribusi belum final, dana terkunci,
  alokasi, token claimable/vesting dan inventori tradable. Semuanya masuk
  penilaian exposure sesuai policy; jangan menggandakan nilai antarstatus.
- Tetapkan batas per aset, sale, chain/venue, posisi dan total dana tidak likuid.
  Jangan menganggap risiko presale dibatasi jarak stop; modal kontribusi bisa
  seluruhnya tidak kembali. Usulkan asumsi kerugian konservatif sebelum pilot.
- Hitung fee trading, gas termasuk kegagalan, transfer tax, slippage dan biaya
  claim/refund. Nilai alokasi atau harga indikatif bukan kas yang dapat ditarik.
- AI hanya mengusulkan. Policy deterministik menahan entry saat limit, data
  basi, signer/RPC/model/storage tidak siap atau status finansial belum pasti.
- Halt/kill switch mencegah entry baru dan mempertahankan rekonsiliasi,
  pemantauan kewajiban serta aksi perlindungan yang diizinkan. Halt tidak
  menjamin exit, refund atau claim berhasil.

## Kegagalan dan kewenangan

Setelah timeout, cari status order/tx dahulu. Konfirmasi/finality dan recovery
harus spesifik chain/venue; reorg atau perubahan receipt dapat membatalkan
settlement sementara. Retry tidak boleh menggandakan kontribusi, order,
approval, claim atau pembukuan. Catat partial fill, reject, revert, transaksi
pending, biaya terpakai dan dana hilang/tidak pulih tanpa menyembunyikannya.

Batasi signer/API key pada aksi, network, kontrak/penerima dan nominal yang
diizinkan. Model tidak menerima private key/seed/API secret dan tidak punya
jalur tool untuk menandatangani. Persetujuan akses dan aktivasi uang asli
memerlukan konfigurasi konkret serta izin tersendiri pada P4-6.
