# Review kode managed runtime — 20 Oktober 2026

Lanjutan [handover](../handover-agent.md) langkah 2, sesi Claude. Fokus:
recovery checkpoint/inbox, OCO parsial, order tak dikenal, reconciliation dan
laporan/batas risiko di `internal/managed` (runtime, ledger, control, limits,
store). Tidak ada `bot run`, deploy atau DB eksperimen; semua bukti adalah unit
test lokal.

## Temuan yang diperbaiki

| # | Temuan | Dampak | Perbaikan dan regression |
|---|---|---|---|
| 1 | `State.Reconcile` menyimpan baseline **sebelum** memeriksa kecukupan wallet. Wallet kurang dana memberi attention, tetapi baseline negatif sudah tersimpan | Rekonsiliasi berikutnya (tiap menit) menganggap saldo cocok dan menghapus attention, sehingga bot memakai modal virtual yang tidak ada di exchange | Cek kecukupan sebelum baseline disimpan; baseline baru dibuat setelah wallet tercukupi. `TestReconciliationKeepsUnderfundedWalletBlocked` |
| 2 | Binance `CancelOrder` tidak memancarkan event; ack terminal datang belakangan lewat user stream. Selama itu leg OCO `canceling` tetap memegang reservasi base | Di testnet/live, `exit()` (cancel proteksi lalu market sell) dan penggantian trailing OCO ditolak `ErrFunds`: flatten/finish/close risiko/exit sinyal gagal, posisi bisa tanpa proteksi. Tidak terlihat di Sim karena cancel Sim langsung memancarkan event | `scopedBroker.CancelOrder` pada broker live memanggil `SyncOrders` untuk order itu setelah cancel sukses atau -2011/-2013, jadi status final dan fill yang hilang dibukukan sebelum order berikutnya. `TestLiveCancelReleasesProtectionBeforeExit` (fake exchange berperilaku seperti Binance) |
| 3 | Equity strategi tanpa `cap` diukur terhadap `Funded` instance. `Transfer` menggeser peak/day-start instance, tetapi tidak milik binding; perubahan `cap` juga tidak | Memindahkan saldo bebas keluar dari instance atau memasang cap terbaca sebagai rugi strategi, sehingga strategi bisa di-halt, dan dengan `CloseOnKill` posisinya dijual | Helper `bindingCapital` dan `rebaseBindings`; transfer dan perubahan cap menggeser referensi binding. `TestCapitalFlowsDoNotTripStrategyLossLimits` |

Ketiga test gagal pada kode sebelum perbaikan dan lulus sesudahnya. Setelah
perbaikan: `go test -race ./...`, `golangci-lint run` 0 issues,
`go vet -tags integration ./...`, `make build` lulus; tiap commit perantara
juga lulus `go test ./internal/managed`. Integration PostgreSQL tidak diulang
karena store/schema tidak berubah.

## Diperiksa, tidak ditemukan masalah

- OCO parsial: fill satu leg menyetel reservasi leg lain ke sisa kuantitas;
  `Reserved` menghitung grup sekali; ack terminal dengan fill yang belum
  dibukukan ditunda (`Deferred`) sampai fill masuk.
- Inbox: event dicatat ke `Pending` dalam transaksi yang sama dengan efek
  finansialnya; checkpoint engine dan penghapusan inbox satu transaksi; dedup
  fill memakai `orderClientID:tradeID`, sama untuk WS dan pemulihan `myTrades`.
- Startup: order checkpoint tanpa reservasi ledger ditolak lokal; inbox diputar
  ulang sebelum view di-rebase.
- Event duplikat untuk order yang sudah terminal diabaikan engine.

## Risiko terbuka (belum diubah)

1. **Submission tak pasti tanpa jalan keluar operator.** Order `submitting`/`unknown`
   yang tidak dikenal exchange (query gagal) membuat attention permanen; belum ada
   perintah terjurnal untuk menandainya rejected setelah verifikasi manual.
   Perlu diputuskan sebelum/selama P2-9.
2. **Pertumbuhan state JSONB.** `Orders`, `Executions`, `History` dan `Requests`
   tidak dipangkas, padahal seluruh state di-unmarshal/rewrite beberapa kali per
   candle. Aman untuk trial 7 hari dengan trailing; ukur di P2-9 sebelum operasi panjang.
3. **Attention transien saat reconcile.** Snapshot saldo diambil sebelum `drain`
   kedua; fill yang tiba di antaranya bisa memberi selisih palsu sampai
   rekonsiliasi berikutnya (≤1 menit). Entry tertahan sementara, tidak ada dana salah.
4. **Restart paper** menempatkan ulang order `canceling` ke Sim. Hanya paper.
5. `PGStore.Update` tidak mencatat idempotency key bila state tidak berubah,
   berbeda dari `MemoryStore`. Belum ditemukan jalur yang dirugikan.
