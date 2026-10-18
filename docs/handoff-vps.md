# Handoff VPS

Panduan operasional server untuk sesi/agent berikutnya. Status per **2026-10-18
16:50 UTC (23:50 WIB)**. Cakupan dan keputusan proyek ada di [plan](plan.md);
prosedur umum ada di [panduan deployment](deployment.md).

## Status sekarang

- **Uji multi-strategi Binance Spot Testnet 3 hari sedang berjalan**, mulai
  2026-10-18 16:43 UTC. **Analisis setelah 2026-10-21 16:43 UTC (23:43 WIB)**
  dengan langkah di [Analisis uji testnet](#analisis-uji-testnet).
- Source `7cc70966c0d093f0db61eca1bbacd4db3e2bae77` (detached), image
  `ghcr.io/rizz404/trading-bot@sha256:0070bdc06e384868963a3c4a3392101a84ba66df4d634a63cf6b6e7f27ece3eb`.
- `TB_APP__MODE=testnet`, kredensial Spot Testnet dan token Telegram ada di
  `.env.vps`. Telegram mengirim ke chat user (chat ID di `configs/config.yaml`
  server). Strategi: `ema-trend-btc`, `ema-fast-eth`, `grid-sol`, `dca-xrp`,
  `mm-doge` ([config](../configs/config.testnet.example.yaml)).
- Riwayat: paper 2026-10-17 14:09 → 2026-10-18 16:43 UTC (observasi 24 jam lulus,
  data tetap di DB); host reboot tak terduga 2026-10-17 15:18–15:42 UTC (bukan dari
  user, pulih otomatis). Detail dan bukti ada di plan.
- Mainnet tidak diizinkan sampai user menyetujui Fase 11. `TB_APP__ALLOW_LIVE` dipaksa
  `false` oleh manifest.

## Batas yang wajib dipertahankan

- `ssh plesk` adalah server Plesk bersama dengan banyak proyek produksi. Hanya sentuh
  project Compose `trading-bot-paper` di `/opt/trading-bot-paper`. Jangan mengubah
  Plesk, domain, web server, firewall, atau Docker daemon. Jangan restart Docker/host,
  upgrade global, `docker system prune`, atau `down -v`.
- Sebelum dan sesudah setiap perubahan jalankan
  `python3 /opt/trading-bot-paper/repo/deploy/vps-baseline-check.py` (exit 0 = tidak ada
  perubahan selain restart akibat boot host). Baseline: 28 container dan 5 service.
- Source dari GitHub (deploy key read-only), image dari CI/GHCR berdasarkan digest
  dengan pasangan commit yang sama. Jangan menyalin folder laptop.
- Secret hanya lewat `deploy/vps-set-env.py` (stdin SSH). Jangan mencetak isi
  `.env.vps`, menaruh secret di repo, chat, atau memori bersama.
- Nama project/direktori tetap `trading-bot-paper` walau mode testnet, supaya unit
  systemd, backup, dan observasi tidak perlu diubah.

## Akses dan susunan server

Claude mengakses lewat `ssh -o BatchMode=yes plesk ...` (root). Izin lokal ada di
`.claude/settings.local.json` laptop user (tidak di-commit), dibatasi pada project ini.
Server AlmaLinux 9, amd64, 8 vCPU, RAM sekitar 32 GB, disk sekitar 52 GB kosong
(sekitar 88% terisi), Docker 29 + Compose 5.

```text
/opt/trading-bot-paper/                 # mode 700
  access/                               # deploy key read-only + known_hosts khusus proyek
  repo/                                 # clone GitHub (detached pada commit rilis)
    .env.vps                            # mode 600: password DB/Grafana/web, BOT_IMAGE, mode, kredensial
    configs/config.yaml                 # config aktif (saat ini salinan config.testnet.example + chat ID)
    configs/config.paper.yaml           # config paper lama, untuk kembali ke paper
  evidence/
    baseline.json                       # baseline proyek lain (2026-10-17)
    samples-YYYYMMDD.jsonl              # sampel observasi tiap 5 menit, disimpan 14 hari
  backups/backup-YYYYMMDDThhmmssZ/      # dump DB + config, 7 terakhir disimpan
```

Unit systemd: `trading-bot-paper-observe.timer` (5 menit) dan
`trading-bot-paper-backup.timer` (harian sekitar 03:15 UTC), keduanya aktif.

## Stack

| Container | Limit | Catatan |
|---|---|---|
| bot | 512 MiB, 0,5 CPU | read-only, cap drop all, stop grace 30 s |
| db (TimescaleDB 2.30 pg17) | 1 GiB, 1 CPU | tanpa port host |
| dns (CoreDNS 1.14.7) | 128 MiB, 0,25 CPU | DNS-over-TLS ke Cloudflare khusus bot; network `resolver` 172.31.253.0/29 |
| prometheus | 512 MiB, 0,5 CPU | retensi 7 hari / 1 GB |
| grafana | 512 MiB, 0,5 CPU | biasanya 380–470 MiB, pantau |

Port host hanya loopback: dashboard 18080, metrics/health 19100, Prometheus 19090,
Grafana 13000. Tunnel dari laptop:
`ssh -N -L 18080:127.0.0.1:18080 -L 13000:127.0.0.1:13000 plesk`. Password dashboard
dan Grafana dibaca user sendiri dari `.env.vps`.

DNS ISP/Tailscale server memblokir domain Binance (Trust Positif), sehingga bot
memakai container `dns`. Error WS mencatat tahap dial (`stage dns|connect|tls|upgrade`).

## Operasi harian

```sh
cd /opt/trading-bot-paper/repo
dc() { docker compose --env-file .env.vps -f deploy/compose.vps.yml "$@" </dev/null; }
dc ps
dc logs --tail 100 bot                                   # log JSON
python3 deploy/vps-observe.py                            # satu sampel; problems: [] berarti sehat
curl -s http://127.0.0.1:19100/metrics | grep -E '^tradingbot_(build_info|kill_switch|paused)'
python3 deploy/vps-baseline-check.py                     # proyek lain tidak berubah
```

Selalu `</dev/null` atau `--interactive=false` untuk perintah Compose di dalam
skrip yang dialirkan lewat `ssh ... 'bash -s'`, karena stdin bisa tertelan.

## Analisis uji testnet

Jendela: 2026-10-18 16:43 UTC sampai 2026-10-21 16:43 UTC. Jalankan dari
`/opt/trading-bot-paper/repo` dengan fungsi `dc` di atas.

```sh
# 1. Kesehatan: sampel bermasalah, restart, OOM
python3 - <<'PY'
import json, glob
rows = [json.loads(l) for f in sorted(glob.glob("../evidence/samples-*.jsonl")) for l in open(f)]
w = [r for r in rows if "2026-10-18T16:43" <= r["time"] < "2026-10-21T16:44"]
print(len(w), "samples;", sum(1 for r in w if r["problems"]), "with problems")
print({r["time"][:16]: r["problems"] for r in w if r["problems"]})
last = w[-1]["containers"]["data"]
print([(c["service"], c["restarts"], c["oom_killed"]) for c in last])
PY
# 2. WARN/ERROR selama jendela, dikelompokkan
dc logs --no-log-prefix --since 2026-10-18T16:43:00Z bot 2>&1 | python3 -c '
import sys, json, collections
c = collections.Counter(json.loads(l)["msg"] for l in sys.stdin if l.startswith("{") and json.loads(l)["level"] in ("WARN","ERROR"))
print(c.most_common(30))'
# 3. Order dan trade per strategi
dc exec -T db psql -U bot -d tradingbot -c "
SELECT strategy, side, status, count(*) FROM orders WHERE exchange='binance-testnet' GROUP BY 1,2,3 ORDER BY 1,2,3;
SELECT strategy, count(*) trades, count(*) FILTER (WHERE net_pnl>0) wins, round(sum(net_pnl),2) net, round(sum(fees),2) fees
  FROM trades WHERE exchange='binance-testnet' GROUP BY 1 ORDER BY 1;
SELECT id, started_at, stopped_at, stop_reason FROM strategy_runs WHERE mode='testnet' ORDER BY id;"
```

Periksa juga:

- Grid: setiap buy terisi diikuti sell satu level di atas, dan sebaliknya; tidak
  ada order ganda setelah restart.
- DCA: tepat satu pembelian per jam sampai maksimal 24, take profit 3% memulai
  siklus baru.
- MM: frekuensi requote wajar (tidak ada `-1003`/rate limit di log), inventori di
  bawah `max_inventory_quote`, tidak menjual tanpa inventori.
- Emacross: entry disertai OCO SL/TP, exit sekali saja.
- Saldo bot vs exchange: posisi bot tidak boleh melebihi saldo, saldo bawaan
  testnet tidak tersentuh.
- Telegram: ringkasan harian 07:00 WIB terkirim, tidak ada `telegram send failed`.

Testnet tanpa fee: angka untung/rugi bukan dasar keputusan live.

## Rilis, deploy, dan mengganti mode

1. Push ke `main`, tunggu CI sukses, ambil digest dari langkah "Record immutable image
   reference" di log job container.
2. `python3 deploy/vps-baseline-check.py`, lalu `bash deploy/vps-backup.sh`.
3. `git fetch origin main && git checkout --detach <sha>`; set
   `BOT_IMAGE=<image@digest>` (contoh: `echo BOT_IMAGE=... | python3 deploy/vps-set-env.py`).
4. `dc run --interactive=false --rm --no-deps bot migrate status` (jalankan
   `migrate up` bila ada migrasi baru).
5. Gerbang DNS: `dc up -d --no-deps dns`, lalu cek jawabannya:
   `nsenter -t "$(docker inspect -f '{{.State.Pid}}' trading-bot-paper-dns-1)" -n dig +short @127.0.0.1 A stream.binance.com`
   (bukan `139.255.196.196`/`182.23.79.195`). Baru setelah itu `dc up -d --no-deps bot`.
6. Verifikasi log startup, `vps-observe.py`, metrics `build_info`, lalu baseline lagi.

Mengganti mode:

- **Ke paper:** `cp configs/config.paper.yaml configs/config.yaml`,
  `echo TB_APP__MODE=paper | python3 deploy/vps-set-env.py`, recreate bot. Paper
  melanjutkan sesi paper terakhir di DB.
- **Ke testnet:** salin `configs/config.testnet.example.yaml` ke `configs/config.yaml`,
  isi `telegram.chat_ids`, sesuaikan rentang grid dengan harga saat itu, set
  `TB_APP__MODE=testnet` dan kredensial Spot Testnet lewat `vps-set-env.py`.
- **Live:** belum boleh. Butuh persetujuan Fase 11, `allow_live` di manifest,
  kredensial mainnet terpisah, dan batas risk dari user.

Secret dari laptop:

```sh
# di laptop, dari root repo; nilai tidak pernah tampil
grep -E '^TB_EXCHANGES__BINANCE__API_(KEY|SECRET)=' .env | ssh -o BatchMode=yes plesk 'python3 /opt/trading-bot-paper/repo/deploy/vps-set-env.py'
```

Rollback: kembalikan commit dan digest sebelumnya (tercatat di plan), periksa
kompatibilitas migrasi, recreate bot. Rollback image tidak membatalkan migrasi DB.

## Backup dan restore

`deploy/vps-backup.sh` membuat dump custom PostgreSQL dan arsip config di
`../backups`; timer harian menyimpan 7 terakhir. Uji restore 2026-10-17 lolos
(prosedur di panduan deployment). Backup masih di disk yang sama; salinan di luar
server ditunda sampai live (keputusan user).

## Catatan teknis yang perlu diingat

- Reconnect WS proaktif tiap 23 jam menghasilkan dua WARN normal.
- Host shutdown menghentikan semua container bersamaan; run yang tertinggal ditutup
  `interrupted` saat start berikutnya, dan bot menunggu DB hingga 60 detik saat start.
- `/healthz` hanya memeriksa DB; kesehatan feed dinilai dari `vps-observe.py`.
- Belum ada Alertmanager; peringatan penting datang lewat Telegram (notice level warn).
- `deploy/vps-baseline-check.py` dan `deploy/vps-set-env.py` baru tersedia di server
  setelah rilis berikutnya di-checkout (commit sesudah `7cc7096`). Sampai saat itu,
  jalankan dari laptop: `ssh plesk 'python3 - /opt/trading-bot-paper/evidence/baseline.json' < deploy/vps-baseline-check.py`.
  Untuk `vps-set-env.py`, salin dulu dengan `scp deploy/vps-set-env.py plesk:/opt/trading-bot-paper/`
  lalu jalankan dengan path env eksplisit (`... vps-set-env.py /opt/trading-bot-paper/repo/.env.vps`)
  dan hapus salinannya setelah dipakai.
