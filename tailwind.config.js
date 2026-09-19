/** Tailwind + DaisyUI compilados en build (reemplaza los CDN de layout.html).
 *
 * Los temas y los dos custom ("nothing", "terminal") son EXACTAMENTE los que
 * declaraba window.tailwind.config en static/app.js cuando se usaba el CDN:
 * si se agrega un tema nuevo hay que tocar los dos lugares.
 */
module.exports = {
  content: [
    "./templates/**/*.html",
    "./static/**/*.js",
    "./cmd/**/*.go",
  ],
  darkMode: "media",
  theme: {
    extend: {},
  },
  plugins: [require("daisyui")],
  daisyui: {
    themes: [
      "dark",
      "dracula",
      "synthwave",
      "cyberpunk",
      "retro",
      "dim",
      "coffee",
      "sunset",
      "night",
      {
        // Tema del panel NOC: los mismos colores que tenía hardcodeados en
        // test2_b.html, pero como tema de DaisyUI. Es el default, así que el panel
        // se ve igual que antes; elegir otro tema redefine estas mismas variables.
        noc: {
          "base-100": "#080A0E",
          "base-200": "#0E1218",
          "base-300": "#1B2431",
          "base-content": "#E9EEF6",
          primary: "#5BBCFF",
          secondary: "#95A2B4",
          accent: "#3BD99E",
          neutral: "#141C26",
          success: "#3BD99E",
          warning: "#F2A93B",
          error: "#FF6157",
          info: "#5BBCFF",
          "--rounded-box": "10px",
          "--rounded-btn": "7px",
        },
        nothing: {
          primary: "#ffffff",
          "base-100": "#000000",
          neutral: "#121212",
          accent: "#ff0000",
          "--rounded-box": "2.5rem",
          "--rounded-btn": "9999px",
        },
        terminal: {
          primary: "#10b981",
          "base-100": "#050505",
          "base-200": "#0a0a0a",
          "base-300": "#1a1a1a",
          "base-content": "#d1d5db",
          neutral: "#1a1a1a",
          error: "#ef4444",
          warning: "#f59e0b",
          accent: "#22d3ee",
          "--rounded-box": "0.25rem",
          "--rounded-btn": "0.25rem",
        },
      },
    ],
  },
};
