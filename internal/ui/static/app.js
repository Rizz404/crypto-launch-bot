// Alpine components (CSP build: templates may only reference properties and
// methods by name, so all logic lives here) and chart rendering.
(function () {
  const css = (name) => getComputedStyle(document.documentElement).getPropertyValue(name).trim();

  document.addEventListener("alpine:init", () => {
    Alpine.data("theme", () => ({
      mode: "system",
      init() {
        try { this.mode = localStorage.getItem("theme") || "system"; } catch (e) {}
        matchMedia("(prefers-color-scheme: dark)").addEventListener("change", () => this.apply());
      },
      cycle() {
        this.mode = this.mode === "system" ? "light" : this.mode === "light" ? "dark" : "system";
        try {
          if (this.mode === "system") localStorage.removeItem("theme");
          else localStorage.setItem("theme", this.mode);
        } catch (e) {}
        this.apply();
      },
      apply() {
        const dark = this.mode === "dark" || (this.mode === "system" && matchMedia("(prefers-color-scheme: dark)").matches);
        document.documentElement.classList.toggle("dark", dark);
        window.dispatchEvent(new CustomEvent("themechange"));
      },
      get isSystem() { return this.mode === "system"; },
      get isLight() { return this.mode === "light"; },
      get isDark() { return this.mode === "dark"; },
      get label() {
        return "Tema: " + (this.mode === "system" ? "otomatis" : this.mode === "light" ? "terang" : "gelap");
      },
    }));

    Alpine.data("dropdown", () => ({
      open: false,
      toggle() { this.open = !this.open; },
      close() { this.open = false; },
    }));

    Alpine.data("menu", () => ({
      open: false,
      toggle() { this.open = !this.open; },
      close() { this.open = false; },
    }));

    // Confirmation for anything marked data-confirm (see the htmx:confirm hook below).
    Alpine.data("confirmDialog", () => ({
      open: false, title: "", body: "", label: "", danger: false, issue: null,
      el: null,
      ask(e) {
        const el = e.detail.el;
        this.el = el;
        this.title = el.dataset.confirm;
        this.body = el.dataset.confirmBody || "";
        this.label = el.dataset.confirmLabel || "Ya, lanjutkan";
        this.danger = el.dataset.confirmTone === "danger";
        this.issue = e.detail.issue;
        this.open = true;
        this.$nextTick(() => this.$refs.cancel.focus());
      },
      accept() {
        // data-confirm-set="field=value" fills a hidden form field only after the
        // user agreed, so the server-side confirmation flag cannot be sent unconfirmed.
        const set = this.el && this.el.dataset.confirmSet;
        const form = this.el && (this.el.form || this.el.closest("form"));
        if (set && form) {
          const [name, value] = set.split("=");
          if (form.elements[name]) form.elements[name].value = value;
        }
        const go = this.issue;
        this.close();
        if (go) go(true);
      },
      close() { this.open = false; this.issue = null; },
      get okClass() {
        return this.danger
          ? "rounded-lg bg-neg px-4 py-2 text-sm font-medium text-page hover:opacity-90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-neg"
          : "rounded-lg bg-primary px-4 py-2 text-sm font-medium text-on-primary hover:bg-primary-hover focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary";
      },
    }));

    // Trading screen order form. Quantity presets use the selected manual
    // strategy's free cash (buy) or holding (sell); the server re-checks all.
    Alpine.data("orderForm", () => ({
      side: "buy", type: "market", qty: "", price: "", cash: 0, hold: 0,
      init() { this.pick(); },
      pick() {
        const o = this.$refs.scope && this.$refs.scope.selectedOptions[0];
        this.cash = o ? Number(o.dataset.cash) : 0;
        this.hold = o ? Number(o.dataset.hold) : 0;
      },
      buy() { this.side = "buy"; },
      sell() { this.side = "sell"; },
      market() { this.type = "market"; },
      limit() {
        this.type = "limit";
        if (!this.price) this.price = lastPrice() ? String(lastPrice()) : "";
      },
      get isBuy() { return this.side === "buy"; },
      get isSell() { return this.side === "sell"; },
      get isMarket() { return this.type === "market"; },
      get isLimit() { return this.type === "limit"; },
      get protectable() { return this.side === "buy" && this.type === "market"; },
      get available() {
        return this.side === "buy" ? this.cash.toFixed(2) + " USDT" : String(this.hold) + " koin";
      },
      percent(e) {
        const pct = Number(e.currentTarget.dataset.pct) / 100;
        if (this.side === "sell") { this.qty = trim(this.hold * pct); return; }
        const px = this.type === "limit" ? Number(this.price) : lastPrice();
        // Leave room for the fee and slippage reserve.
        if (px > 0) this.qty = trim((this.cash * pct * 0.995) / px);
      },
      get summary() {
        const px = this.type === "limit" ? "@ " + this.price : "harga pasar";
        return (this.side === "buy" ? "Beli " : "Jual ") + this.qty + " " + px + "\nStrategi: " + (this.$refs.scope ? this.$refs.scope.value : "");
      },
      get submitLabel() { return (this.side === "buy" ? "Beli" : "Jual") + (this.type === "limit" ? " limit" : " market"); },
      get submitClass() {
        return "w-full rounded-lg px-4 py-2 text-sm font-semibold text-page hover:opacity-90 " + (this.side === "buy" ? "bg-pos" : "bg-neg");
      },
    }));

    Alpine.data("accountTabs", () => ({
      tab: "positions",
      positions() { this.tab = "positions"; },
      orders() { this.tab = "orders"; },
      fills() { this.tab = "fills"; },
      funds() { this.tab = "funds"; },
      get isPositions() { return this.tab === "positions"; },
      get isOrders() { return this.tab === "orders"; },
      get isFills() { return this.tab === "fills"; },
      get isFunds() { return this.tab === "funds"; },
      get posCurrent() { return this.tab === "positions" ? "page" : "false"; },
      get ordersCurrent() { return this.tab === "orders" ? "page" : "false"; },
      get fillsCurrent() { return this.tab === "fills" ? "page" : "false"; },
      get fundsCurrent() { return this.tab === "funds" ? "page" : "false"; },
    }));

    Alpine.data("toast", () => ({
      shown: false, message: "", timer: null,
      show(e) {
        const d = e.detail;
        this.message = typeof d === "string" ? d : (d && d.value) || "";
        if (!this.message) return;
        this.shown = true;
        clearTimeout(this.timer);
        this.timer = setTimeout(() => { this.shown = false; }, 6000);
      },
      hide() { this.shown = false; },
    }));
  });

  // Replace hx-confirm: styled dialog, same semantics (cancel => no request).
  document.addEventListener("htmx:confirm", (e) => {
    const d = e.detail;
    const sub = d.triggeringEvent && d.triggeringEvent.submitter;
    const el = [sub, d.elt].find((x) => x && x.dataset && x.dataset.confirm);
    if (!el) return;
    e.preventDefault();
    window.dispatchEvent(new CustomEvent("confirm-ask", { detail: { el, issue: d.issueRequest } }));
  });

  const toast = (message) => window.dispatchEvent(new CustomEvent("toast", { detail: message }));
  document.addEventListener("htmx:responseError", (e) => {
    toast("Permintaan gagal (" + e.detail.xhr.status + "). Muat ulang halaman bila berulang.");
  });
  document.addEventListener("htmx:sendError", () => toast("Tidak bisa menghubungi server."));

  // Charts: <div data-chart="line|candles" data-src="/api/..."> ---------------
  const charts = new Map();

  function baseChart(el) {
    const chart = LightweightCharts.createChart(el, {
      autoSize: true,
      layout: { background: { color: "transparent" }, textColor: css("--c-muted"), fontFamily: "inherit" },
      grid: { vertLines: { color: css("--c-outline") }, horzLines: { color: css("--c-outline") } },
      rightPriceScale: { borderColor: css("--c-outline") },
      timeScale: { borderColor: css("--c-outline"), timeVisible: true },
    });
    charts.set(el, chart);
    return chart;
  }

  async function load(el) {
    const res = await fetch(el.dataset.src, { credentials: "same-origin" });
    if (!res.ok) throw new Error(res.status + " " + (await res.text()));
    return res.json();
  }

  function note(el, text) {
    const p = document.createElement("p");
    p.className = "py-8 text-center text-sm text-muted";
    p.textContent = text;
    el.replaceChildren(p);
  }

  async function line(el) {
    const points = await load(el);
    if (!points.length) return note(el, "Belum ada data.");
    const chart = baseChart(el);
    chart.addSeries(LightweightCharts.LineSeries, { color: css("--c-link"), lineWidth: 2 }).setData(points);
    chart.timeScale().fitContent();
  }

  async function candles(el) {
    const data = await load(el);
    if (!data.candles.length) return note(el, "Tidak ada candle pada rentang ini.");
    const chart = baseChart(el);
    const s = chart.addSeries(LightweightCharts.CandlestickSeries, {
      upColor: css("--c-pos"), downColor: css("--c-neg"), borderVisible: false,
      wickUpColor: css("--c-pos"), wickDownColor: css("--c-neg"),
    });
    s.setData(data.candles);
    const pick = (c) => (c === "pos" ? css("--c-pos") : c === "neg" ? css("--c-neg") : css("--c-link"));
    LightweightCharts.createSeriesMarkers(s, data.markers.map((m) => ({ ...m, color: pick(m.color) })));
    chart.timeScale().fitContent();
  }

  // Trading screen ---------------------------------------------------------
  // One EventSource per page: book, tape and ticker update in place; the
  // chart's last candle follows the stream; account panels re-fetch on notice.
  let source = null, liveSeries = null, last = 0;
  function lastPrice() { return last; }
  function trim(n) { return n > 0 ? String(Number(n.toFixed(8))) : ""; }
  const fmt = (v) => { const n = Number(v); return n >= 100 ? n.toFixed(2) : n >= 1 ? n.toFixed(4) : String(n); };
  const fmtQty = (v) => String(Number(Number(v).toFixed(6)));

  async function tradeChart(el) {
    const data = await load(el);
    const chart = baseChart(el);
    const s = chart.addSeries(LightweightCharts.CandlestickSeries, {
      upColor: css("--c-pos"), downColor: css("--c-neg"), borderVisible: false,
      wickUpColor: css("--c-pos"), wickDownColor: css("--c-neg"),
    });
    s.setData(data.candles);
    const pick = (c) => (c === "pos" ? css("--c-pos") : c === "neg" ? css("--c-neg") : css("--c-link"));
    LightweightCharts.createSeriesMarkers(s, data.markers.map((m) => ({ ...m, color: pick(m.color) })));
    liveSeries = s;
  }

  function bookRows(box, levels, tone, max) {
    const rows = levels.map(([p, q]) => {
      const row = document.createElement("div");
      row.className = "relative grid grid-cols-3 py-0.5 tabular-nums";
      const bar = document.createElement("span");
      bar.className = "absolute inset-y-0 right-0 " + (tone === "pos" ? "bg-pos/10" : "bg-neg/10");
      bar.style.width = Math.min(100, (Number(q) * Number(p) / max) * 100) + "%";
      const cells = [[fmt(p), tone === "pos" ? "text-pos" : "text-neg"], [fmtQty(q), "text-right"], [fmt(Number(p) * Number(q)), "text-right text-muted"]];
      row.append(bar, ...cells.map(([t, c]) => { const s = document.createElement("span"); s.className = "relative " + c; s.textContent = t; return s; }));
      return row;
    });
    box.replaceChildren(...rows);
  }

  function onMarket(root, d) {
    const book = root.querySelector("[data-book]");
    if (book) {
      book.querySelector("[data-book-wait]").hidden = d.synced;
      const all = d.bids.concat(d.asks);
      const max = Math.max(1, ...all.map(([p, q]) => Number(p) * Number(q)));
      bookRows(book.querySelector("[data-asks]"), d.asks, "neg", max);
      bookRows(book.querySelector("[data-bids]"), d.bids, "pos", max);
      const spread = book.querySelector("[data-spread]");
      if (d.bids.length && d.asks.length) {
        // Price strings carry the tick precision; float maths must not add digits.
        const places = Math.max(...[d.bids[0][0], d.asks[0][0]].map((p) => (p.split(".")[1] || "").length));
        const bid = Number(d.bids[0][0]), ask = Number(d.asks[0][0]);
        spread.textContent = ((bid + ask) / 2).toFixed(places + 1) + "  ·  spread " + (ask - bid).toFixed(places);
      }
    }
    const tape = root.querySelector("[data-tape]");
    if (tape && d.trades.length) {
      const rows = d.trades.slice().reverse().map((t) => {
        const row = document.createElement("div");
        row.className = "grid grid-cols-3 py-0.5 tabular-nums";
        const cells = [[fmt(t.p), t.s ? "text-neg" : "text-pos"], [fmtQty(t.q), "text-right"], [new Date(t.t).toLocaleTimeString(), "text-right text-muted"]];
        row.append(...cells.map(([x, c]) => { const s = document.createElement("span"); s.className = c; s.textContent = x; return s; }));
        return row;
      });
      tape.prepend(...rows);
      while (tape.children.length > 40) tape.lastChild.remove();
    }
    if (d.ticker) {
      last = Number(d.ticker.last);
      const set = (k, v, tone) => {
        const el = root.querySelector('[data-ticker="' + k + '"]');
        if (!el) return;
        el.textContent = v;
        if (tone !== undefined) el.className = el.className.replace(/\btext-(pos|neg)\b/g, "") + " " + tone;
      };
      const up = Number(d.ticker.pct) >= 0;
      set("last", fmt(d.ticker.last), up ? "text-pos" : "text-neg");
      set("pct", (up ? "+" : "") + d.ticker.pct + "%", up ? "text-pos" : "text-neg");
      set("high", fmt(d.ticker.high));
      set("low", fmt(d.ticker.low));
      set("qv", Number(d.ticker.qv).toLocaleString());
    }
    if (d.candle && liveSeries) {
      try { liveSeries.update(d.candle); } catch (e) { /* older than the loaded history */ }
    }
  }

  function trade(root) {
    const el = root.querySelector("[data-trade]");
    if (!el || el.dataset.live) return;
    el.dataset.live = "1";
    if (source) source.close();
    const state = el.querySelector("[data-stream-state]");
    source = new EventSource(el.dataset.stream);
    source.addEventListener("open", () => { if (state) state.textContent = "live"; });
    source.addEventListener("error", () => { if (state) state.textContent = "terputus, mencoba lagi…"; });
    source.addEventListener("market", (e) => onMarket(el, JSON.parse(e.data)));
    source.addEventListener("account", () => window.dispatchEvent(new CustomEvent("account-changed")));
  }

  function render(root) {
    root.querySelectorAll("[data-chart]").forEach((el) => {
      if (el.dataset.rendered) return;
      el.dataset.rendered = "1";
      const fn = el.dataset.chart === "trade" ? tradeChart : el.dataset.chart === "candles" ? candles : line;
      fn(el).catch((err) => note(el, "Grafik gagal dimuat: " + err.message));
    });
    trade(root);
  }

  // Leaving the screen through a boosted link must close the stream.
  document.addEventListener("htmx:beforeSwap", (e) => {
    if (source && e.detail.target === document.body) { source.close(); source = null; liveSeries = null; }
  });
  document.addEventListener("DOMContentLoaded", () => render(document));
  document.addEventListener("htmx:afterSettle", (e) => render(e.target));
  window.addEventListener("themechange", () => {
    charts.forEach((chart, el) => {
      chart.remove();
      if (el.isConnected) delete el.dataset.rendered;
    });
    charts.clear();
    render(document);
  });
})();
