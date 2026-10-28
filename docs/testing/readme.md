# Bukti pengujian

Laporan di sini merekam pengujian pada waktunya; bukan status real-time,
kelulusan fitur sesudahnya, holdout baru atau izin live. Tanggal tetap ada di
isi laporan/payload. Rencana aktif: [Plan 3](../plan-3.md); pembanding lama:
[Strategi 2](../strategy-2.md). Plan 2 sudah selesai sebagai siklus/arsip.
Penerimaan tersisa dan risiko laporan historis dipetakan ke P3-O/P3-R/P3-LIVE.

## Operasi dan Plan 2

| Bukti | Isi |
|---|---|
| [WIP pilot katalis](catalyst-pilot-local.md) | Checkpoint unit/PG/native binary sesudah rilis komponen; belum tes lengkap checkout akhir atau aktivasi pilot |
| [Rilis komponen P3](catalyst-release.json) | Seluruh CI/GHCR dan deployment report VPS 28 Oktober; digest/source, batas isolasi dan 35 container lama tetap identik; bukan start pilot |
| [Verifikasi Spider](catalyst-spider-verification.json) | Build/smoke image, regression stdout p3.1 dan tiga putaran binary TLS fixture tanpa jaringan keluar; batas egress/situs publik |
| [Preflight VPS Spider](catalyst-vps-preflight.json) | Sampel kapasitas read-only 26 Oktober, usulan penempatan terpisah; belum instalasi/pilot |
| [Evaluasi model OmniRoute](catalyst-model-evaluation.md) / [JSON](catalyst-model-evaluation.json) | Inference nyata pada tujuh kasus sintetis, model utama dan kandidat fallback; bukan shadow prospektif |
| [Development Plan 3](plan-3-local.md) | Collector/adapter/validator, Spider CLI/proxy fixture 28 Oktober, paper replay dan batas penerimaan nyata |
| [Rilis notifikasi 23 Oktober](telegram-notification-release.json) | Source/digest, snapshot ledger, restore pra-rilis, feed dan batas penerimaan Telegram/backup pascarilis |
| [Start pembanding tahap dua](research-stage-two-start.json) | Ekspansi 6/18, CPU1, modal virtual eksplisit, DB baru dan pemeriksaan awal |
| [Rilis koreksi tahap dua](research-stage-two-release.json) | Commit/push/CI, koreksi pembacaan ledger berulang setelah feed tertinggal, dan pemeriksaan VPS setelah rilis |
| [Start riset tahap awal](research-stage-one-start.json) | Paper VPS 3 instance/9 binding, DB baru, T0/review 1/3/7, pemeriksaan rilis/ledger/menu dan batas CPU |
| [Stop P2-9](p2-9-stop.json) / [review](p2-9-review.json) | ETH paper ditarik 21 Oktober: tanpa trade, backup tersimpan, sinyal momentum dan temuan operasi |
| [Start P2-9](p2-9-vps-start.json) | Rilis, migrasi DB salinan, restore drill dan insiden warmup |
| [Test lokal](plan-2-local.md) / [smoke modal bersama](plan-2-shared-capital-smoke.json) | Ledger, ownership, reservasi, recovery, lease, HTTP/CLI dan benchmark singkat |
| [QA browser](plan-2-browser-qa.md) | Fixture Firefox desktop/mobile, form, tema dan batas bukti |
| [Review kode](plan-2-code-review.md) | Tiga fix dana/risiko, regression dan risiko saat review |
| [Audit awal](p2-0-audit.md) / [data](p2-0-audit.json) / [error exchange](p2-0-exchange-errors.json) | OCO, dust DOGE, equity transient dan kontrak penutupan |
| [Testnet final](testnet-stop/analysis.md) | Trial ±43 jam, tujuh cancel exchange, PnL dan backlog |
| [Handover awal VPS](vps-handoff.json) / [operasi](vps-ops.json) | Checkpoint, deployment, resolver dan restore historis |
| [Browser awal](browser-verification.json) / [monitoring](monitoring-verification.json) | Dashboard, Grafana, datasource dan alert |

Trial P2-9 berhenti sebelum tujuh hari; jadwal review dibatalkan dan belum ada
backtest pengganti baru. Masalah lama yang dicatat laporan review dapat sudah
mendapat implementasi lanjutan; cek runtime/plan, jangan menganggap laporan
historis sebagai daftar bug terkini.

## Riset historis

| Bukti | Isi |
|---|---|
| [Backtest awal](historical-backtest.json) | EMA BTC 1h Q1 2025 |
| [EMA lintas periode](emacross-grid.json) / [filter tren](emacross-trend.json) | Walk-forward lima koin, belum ada edge konsisten |
| [DCA/grid](dca-grid.json) | Split waktu dan pembanding buy-and-hold |
| [Baseline](backtest/summary.txt) / [hasil](backtest/results.json) | 20 kombinasi, modal independen, semua rugi pada parameter/periode yang diuji |
| [Kandidat](candidates/summary.txt) / [aturan](candidates/candidate-spec.json) | Lima kandidat beku dan batas interpretasi |
| [Riset](research-analysis.json) / [hasil](research-results.json) / [plan](research-queue.json) | 2.340 pekerjaan dan bias seleksi |
| [Validasi](high-return-validation-analysis.json) / [hasil](high-return-validation-results.json) / [plan](high-return-validation-plan.json) | 360 replay 1m; belum memenuhi target 50% konsisten |
| [Sensitivitas biaya](fee-sensitivity/summary.txt) / [hasil](fee-sensitivity/results.json) | Tarif standar vs diskon BNB hipotetis |
| [Profil ETH](eth-paper-reference.json) / [fee ETH](eth-reference-fees/summary.json) / [start lokal](eth-paper-start.json) | Parameter, simulasi tahunan dan riwayat lokal yang sudah dihentikan |

## Penyimpanan dan reproduksi

JSON bukti dipertahankan byte-for-byte saat penggantian nama. Path/nama lama di
payload adalah provenance historis, bukan lokasi terkini; gunakan indeks ini.
Checksum sumber tetap merujuk byte payload asli. Jangan mengubah angka, tanggal,
ID simbol/run, atau path data runtime untuk sekadar menyeragamkan nama file.

Ekspor `*-equity.csv`/`*-trades.csv`, arsip harga, binary/log antrean dan DB tidak
masuk Git; ringkasan, parameter, manifest/checksum dan bukti final tetap masuk.
Rekaman adapter testnet ada di `internal/exchange/binance/testdata/testnet/`.
Script reproduksi memakai direktori baru tanpa tanggal; jangan jalankan ulang
atau menimpa bukti tanpa tugas pengujian yang jelas. Eksperimen lanjutan memakai
nama berikutnya (`backtest-2`, dst.), bukan mengganti bukti pertama.
