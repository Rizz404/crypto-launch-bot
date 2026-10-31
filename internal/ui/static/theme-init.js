// Runs synchronously in <head> so the saved theme is applied before first paint.
(function () {
  var saved = null;
  try { saved = localStorage.getItem("theme"); } catch (e) {}
  var dark = saved === "dark" || (saved !== "light" && window.matchMedia("(prefers-color-scheme: dark)").matches);
  document.documentElement.classList.toggle("dark", dark);
})();
