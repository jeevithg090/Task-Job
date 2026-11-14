// Applies the saved colour theme before first paint to avoid a flash of the
// wrong theme. Loaded synchronously from <head>; app.js handles the toggle.
(function () {
  var theme = null;
  try { theme = localStorage.getItem('theme'); } catch (e) { /* storage blocked */ }
  if (theme === 'light' || theme === 'dark') {
    document.documentElement.setAttribute('data-theme', theme);
  }
})();
