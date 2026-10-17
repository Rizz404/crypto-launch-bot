// Renders charts declared with data-chart="line|candles" and data-src.
(function () {
  const css = (name) => getComputedStyle(document.documentElement).getPropertyValue(name).trim();

  function baseChart(el) {
    return LightweightCharts.createChart(el, {
      autoSize: true,
      layout: { background: { color: "transparent" }, textColor: css("--muted") },
      grid: { vertLines: { color: css("--border") }, horzLines: { color: css("--border") } },
      rightPriceScale: { borderColor: css("--border") },
      timeScale: { borderColor: css("--border"), timeVisible: true },
    });
  }

  async function load(el) {
    const res = await fetch(el.dataset.src, { credentials: "same-origin" });
    if (!res.ok) throw new Error(res.status + " " + (await res.text()));
    return res.json();
  }

  async function line(el) {
    const points = await load(el);
    if (!points.length) { el.innerHTML = '<p class="muted">No data yet.</p>'; return; }
    const chart = baseChart(el);
    const s = chart.addSeries(LightweightCharts.LineSeries, { color: css("--accent"), lineWidth: 2 });
    s.setData(points);
    chart.timeScale().fitContent();
  }

  async function candles(el) {
    const data = await load(el);
    if (!data.candles.length) { el.innerHTML = '<p class="muted">No candles in range.</p>'; return; }
    const chart = baseChart(el);
    const s = chart.addSeries(LightweightCharts.CandlestickSeries, {
      upColor: css("--pos"), downColor: css("--neg"), borderVisible: false,
      wickUpColor: css("--pos"), wickDownColor: css("--neg"),
    });
    s.setData(data.candles);
    const markers = data.markers.map((m) => ({ ...m, color: m.color === "pos" ? css("--pos") : m.color === "neg" ? css("--neg") : css("--accent") }));
    LightweightCharts.createSeriesMarkers(s, markers);
    chart.timeScale().fitContent();
  }

  function render(root) {
    root.querySelectorAll("[data-chart]").forEach((el) => {
      if (el.dataset.rendered) return;
      el.dataset.rendered = "1";
      const fn = el.dataset.chart === "candles" ? candles : line;
      fn(el).catch((err) => { el.innerHTML = '<p class="alert">Chart failed: ' + String(err.message).replace(/</g, "&lt;") + "</p>"; });
    });
  }

  document.addEventListener("DOMContentLoaded", () => render(document));
  document.addEventListener("htmx:afterSwap", (e) => render(e.target));
})();
