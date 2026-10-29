# Migrasi server — 29 Oktober 2026

Stack paper P2/riset (`trading-bot-paper`) dan pilot P3 (`trading-bot-catalyst`) dipindah dari
`plesk` (srv1142454, 31.97.109.89) ke `plesk-newest` (srv2088216, 187.124.139.72) atas izin user.
Nama yang terlihat di host disamarkan: project/container/volume/network, image, `/opt` dan unit systemd.

| Lama | Baru |
|---|---|
| `trading-bot-paper`, `/opt/trading-bot-paper` | `ana-core`, `/opt/ana-core` |
| `trading-bot-catalyst`, `/opt/trading-bot-catalyst` | `ana-scout`, `/opt/ana-scout` |
| `ghcr.io/rizz404/trading-bot`, `-catalyst` | `ghcr.io/rizz404/ana-core`, `ana-scout` |
| unit `trading-bot-paper-*`, `trading-bot-catalyst-*` | `ana-core-*`, `ana-scout-*` |

Tetap: nama DB `tradingbot_*`, user DB, metrik `tradingbot_*`, nama service Compose, modul Go,
User-Agent collector, image build-only `trading-bot-spider`.

## Rilis dan data

- Source `4b893ab` (rename deploy/CI saja; Go tidak berubah sejak `d30b93b`),
  [CI 40913479007](https://github.com/Rizz404/trading-bot/actions/runs/40913479007) lulus.
- `ana-core@sha256:f7b3bbf69b724648f5d5096828c2af0a641c43740e48048209440d52864fb538`
- `ana-scout@sha256:0f1731d50ae228346eef6d138ac7e1092290f876cb989ed5fb0dae7ce739c2f8`
- Backup pra-stop di plesk: `backup-20261029T164613Z` (paper), `backup-20261029T164615Z` (P3).
- Stop graceful plesk ±16:46 UTC, semua exit 0, PG tanpa `postmaster.pid`. Volume disalin fisik
  NEWEST←plesk langsung (rsync SSH server-ke-server); checksum rsync 0 selisih, byte file identik.
- Start NEWEST 16:48:22 UTC (23:48 WIB). Migrasi 1–5 applied, tanpa migrasi baru.

## Verifikasi 16:49–16:52 UTC

- CoreDNS DoT menjawab IP Binance asli; DNS host NEWEST juga memberi halaman blokir.
- Health 200 (metrics 19100, research 19102, P3 18081); restart 0, nol WARN/ERROR.
- Candle 1m 16:40–16:49 lengkap per simbol di DB tahap dua/tiga (backfill downtime).
- Observer paper `problems: []`; P3 hanya alert lama `research_error`/`latest_review_invalid`.
- Timer `ana-*` enabled; baseline NEWEST baru `differences: []` (baseline lama: `evidence/baseline-plesk.json`).
- `mysql84` dan container lain NEWEST tidak berubah.

## Rollback dan sisa

plesk menyimpan container `exited` (restart `no`), volume, image, backup dan timer `disabled`.
Rollback: stop `ana-*` di NEWEST, lalu start stack lama di plesk dan enable timer. Jangan jalankan
keduanya bersamaan (Telegram poller, ledger). Hapus sisa di plesk hanya dengan izin terpisah.

Objek `trading-bot-*` usang dari migrasi whole-server di NEWEST dihapus; script cutover-nya
mengecualikan/melindungi `ana-*` (salinan asli `*.bak-pre-ana`). Belum diuji: review/ledger
penuh lewat dashboard, pesan Telegram setelah pindah, backup terjadwal pertama di NEWEST.
