# Review akhir P2, tahap tiga dan P3 — 1 November 2026

Snapshot baca-saja di `plesk-newest`, cutoff **1 November 2026 19:03–19:05 WIB**
(12:03–12:05 UTC), atas permintaan user sebelum seluruh eksperimen dimatikan dan
analisis dimulai ulang. Waktunya menggabungkan review P3 hari 3 (telat ±20 jam)
dan tahap tiga D+3 (±4 jam lebih awal). Tidak ada restart, deploy, perubahan
config/parameter/ledger/T0 atau inference paksa.

Sumber: `/api/state` ketiga dashboard (loopback, basic auth), `/research.json` P3,
`vps-observe.py`, `catalyst-observe.py`, `docker inspect/stats`, journal observer
sejak 30 Oktober, folder `daily-budget` P3, candle 1h tahap tiga.

## Ringkasan

| Eksperimen | T0 (WIB) | Lama | Modal virtual | Equity | Return | Trade tertutup |
|---|---|---|---|---|---|---|
| P2 pembanding (6 instance/18 binding) | 22 Okt 19:58 | ±10 hari | 747,8396 | 740,4056 | **−0,9941%** | hanya EMA 1m |
| Tahap tiga (6 kasus BTC/ETH) | 29 Okt 23:14 | ±2,8 hari | 664,7463 | 664,1539 | **−0,0891%** | 3 |
| Pilot AI P3 | 28 Okt 22:53 | ±3,8 hari | 1.000 | 1.000 | **0%** | 0 |

Pembanding buy-and-hold (close 1h): BTC 81.700,01 → 77.962,62 (−4,57%) sejak T0 P2,
81.269,56 → 77.962,62 (−4,07%) sejak T0 tahap tiga; ETH 2.410,39 → 2.270,32 (−5,81%)
dan 2.353,20 → 2.270,32 (−3,52%). Pasar turun. Kerugian bot kecil terutama karena
hampir selalu memegang kas, bukan karena strategi terbukti melindungi modal.

Semua posisi nol, tidak ada order pending, tidak ada halt/attention pada ketiga wallet.

## Operasi

- Kesembilan container `ana-*` Up, `RestartCount` 0. Kedua observer exit 0 `problems: []`.
- Timer `ana-{core,scout}-{observe,backup}` active. Backup terbaru 1 November 03:22 UTC (core)
  dan 03:51 UTC (scout).
- Journal observer core sejak 30 Oktober: 719 sampel bersih, 1 gangguan health/candle,
  1 gangguan Grafana.
- Journal observer P3: 530 sampel bersih; 168 `research_error`/`latest_review_invalid`,
  11 dengan `source_gap`/`source_stale_or_failed`, 14 `container_restarted`, 2 `observer_failed`.
  Alert review invalid lama sudah hilang karena review terbaru valid.
- P3 punya 37 run; 30 Oktober penuh run `interrupted` (masalah restart/spam). Run sesudah
  31 Oktober 17:05 UTC dan 1 November 03:51 UTC bertepatan dengan rilis UI/scout.
  Akar restart 30 Oktober tetap belum terbukti.
- Sampel CPU/RAM sesaat: bot P2 4,58%/91,7 MiB, research 2,11%/51,4 MiB, P3 4,83%/55,7 MiB,
  Grafana 419,7/512 MiB (paling dekat batas).

## P2 pembanding

| Instance | Status | Return | Catatan |
|---|---|---|---|
| ema-base-1m | paused 29 Okt | −3,829% | 69 fill hot, fee 1,8403 |
| ema-trend-1m | paused 29 Okt | −2,881% | 49 fill hot, fee 1,3187 |
| ema-trend-4h | running | 0% | tidak pernah trade |
| tren-500k / 2000k / 5000k | running | 0% | momentum BTC/ETH 4h dan pullback SOL tidak pernah trade |

Return total tidak berubah sejak review tujuh hari 29 Oktober. Fill hot yang terlihat
118; total 214 termasuk arsip sesuai [review tujuh hari](research-stage-two-review.md).
Selama ±10 hari, 12 binding non-1m menghasilkan **nol** entry.

## Tahap tiga (D+3)

| Kasus | Fill | Fee | Realized net | Return |
|---|---|---|---|---|
| ema-btc-15m | 4 (2 round trip) | 0,1088 | −0,1012 | −0,091% |
| ema-eth-1h | 2 (1 round trip) | 0,0546 | −0,4912 | −0,443% |
| ema-btc-1h, ema-eth-15m | 0 | 0 | 0 | 0% |
| pullback-btc-4h, pullback-eth-4h | 0 | 0 | 0 | 0% |

Gross ema-btc-15m sekitar +0,008 USDT; fee membuatnya negatif. Tiga trade tidak cukup
untuk menilai strategi apa pun.

## Pilot AI P3

- 17 review AI: 11 valid, semuanya **WATCH**; 6 invalid (3 `inference transport failed`
  30 Oktober pada gateway lama, 2 `citation source outside policy`, 1 simbol kosong).
- Sejak 30 Oktober 18:05 UTC seluruh 7 review valid dengan model served `gpt-6-luna`.
  Gateway baru terbukti melayani inference nyata.
- Kuota terpakai per hari WIB: 1, 4, 5, 4, 3 dari 12. Token total 140.820 input / 5.421 output.
- Tiga sumber (Ethereum, Anza, Dogecoin blog) HTTP 200 pada cutoff.
- `Events` selalu kosong. Policy mewajibkan event terjadwal dengan tanggal literal dari
  sumber primer, jadi AI secara desain tidak bisa entry. Nol trade adalah akibat pipeline,
  bukan penilaian pasar.

## Kesimpulan

1. **Tidak ada eksperimen yang menunjukkan edge.** P2 rugi 0,99%, terutama fee EMA 1m.
   Tahap tiga −0,09% dari tiga trade. P3 belum pernah trade.
2. **Masalah utama adalah frekuensi sinyal, bukan risiko.** 16 dari 18 binding indikator
   non-1m (12 P2 + 6 tahap tiga) tidak pernah entry. Timeframe cepat menghasilkan trade, tetapi fee 0,1%/sisi memakannya.
3. **Pilot AI terhambat di hulu.** Tiga blog proyek tidak menghasilkan event terjadwal.
   AI dan validator bekerja, tetapi tidak pernah mendapat input yang bisa dieksekusi.
4. **Operasi cukup stabil untuk paper** setelah 30 Oktober: observer bersih, backup jalan,
   gateway AI baru berhasil. Akar restart P3 30 Oktober belum terbukti.
5. Sampel terlalu kecil untuk keputusan strategi. Hasil ini bukan bukti profit/rugi jangka panjang
   dan bukan dasar izin uang asli.

## Masukan untuk analisis ulang

- Ukur frekuensi sinyal dan biaya per trade di replay **sebelum** deploy paper; tolak kasus
  yang secara historis jarang entry atau yang gross-nya lebih kecil dari fee.
- Pakai satu ledger/satu dashboard untuk semua kasus agar review tidak perlu tiga snapshot.
- Untuk AI: sumber event harus memuat jadwal (kalender listing/upgrade/unlock, pengumuman
  exchange), atau aturan entry diubah agar tidak hanya bergantung pada event bertanggal.
- Simpan buy-and-hold periode sama sebagai pembanding default di dashboard.

## Batas review ini

- Tidak ada build, test, restart, recovery, restore drill atau QA browser/Telegram.
- Equity dari kas karena semua holding nol; rekonsiliasi Decimal terhadap arsip fill P2
  tidak diulang (angka total sama dengan review 29 Oktober).
- Buy-and-hold memakai close 1h terdekat, bukan harga eksekusi. SOL tidak dihitung.
- Hitungan observer adalah sampel timer, bukan persentase uptime.
- Belum ada yang dimatikan. Penghentian adalah langkah terpisah yang perlu izin.
