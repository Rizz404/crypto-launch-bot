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

  function render(root) {
    root.querySelectorAll("[data-chart]").forEach((el) => {
      if (el.dataset.rendered) return;
      el.dataset.rendered = "1";
      (el.dataset.chart === "candles" ? candles : line)(el).catch((err) => note(el, "Grafik gagal dimuat: " + err.message));
    });
  }

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
