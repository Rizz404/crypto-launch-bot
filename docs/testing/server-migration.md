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

Tetap: nama DB `tradingbot_*`, user DB, metrik `tradingbot_*`, nama service Compose, modul Go dan
image build-only `trading-bot-spider`. User-Agent collector menjadi `ana-scout-research/1.0`;
label image hanya `org.opencontainers.image.revision`.

## Rilis dan data

- Cutover memakai source `4b893ab` (rename deploy/CI; Go sama dengan `d30b93b`). Atas izin user,
  history di-force-push tanpa trailer: `4b893ab`→`0795127`, `6a69a74`→`1b97be8` (tree identik).
  Image `4b893ab` (`ana-core@sha256:f7b3bbf6…`, `ana-scout@sha256:0f1731d5…`) hanya dipakai
  29 Oktober 23:48 – 30 Oktober 00:10 WIB.
- **Berjalan sekarang:** source `e74a9af` (UA collector + label), [CI 40917315665](https://github.com/Rizz404/trading-bot/actions/runs/40917315665)
  lulus, worker di-recreate 17:10 UTC (00:10 WIB 30 Oktober):
  `ana-core@sha256:fdd93a16e71631f67babdb81a48c032e46b96ca19d29234cef90438eb87cfb55`,
  `ana-scout@sha256:99cf4ee63ee206fdac935ec27a0f2b5174053fb6bcb4049804a68d6e0cbcb420`.
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

## Verifikasi lanjutan 17:01–17:12 UTC

- Backup manual lewat unit `ana-core-backup`/`ana-scout-backup` sukses
  (`backup-20261029T170150Z`, `backup-20261029T170153Z`); P3 kembali healthy.
- Ledger dari `managed_wallets`: P2 kas total 740,4056 USDT flat (cocok review 740,40558347),
  EMA 1m dasar/tren `paused`, empat lainnya `running`, StartedAt 22 Okt 12:58:45 UTC; journal
  `run_end` 16:46:36 (plesk) lalu `run_start` 16:48:23 (NEWEST). Tahap tiga enam kasus `running`,
  StartedAt 16:14:26 UTC. P3 1.000 USDT flat, StartedAt 28 Okt 15:53:13 UTC.
- Sesudah rilis `e74a9af`: health 200, versi log cocok, restart/WARN 0, observer sama seperti di atas.
- Telegram: `sendMessage` dari NEWEST ke chat whitelist `ok=True`. `/help` P2 dibalas (dikonfirmasi user 30 Oktober).

## Rollback dan sisa

plesk menyimpan container `exited` (restart `no`), volume, image, backup dan timer `disabled`.
Rollback: stop `ana-*` di NEWEST, lalu start stack lama di plesk dan enable timer. Jangan jalankan
keduanya bersamaan (Telegram poller, ledger). User memilih menahan sisa plesk tujuh hari (sampai ±5 November), lalu ditanya lagi sebelum dihapus.

Objek `trading-bot-*` usang dari migrasi whole-server di NEWEST dihapus; script cutover-nya
mengecualikan/melindungi `ana-*` (salinan asli `*.bak-pre-ana`). `/opt/ana-scout/deploy-pilot.sh` lama dihapus.
Belum terjadi: backup terjadwal pertama di NEWEST.
