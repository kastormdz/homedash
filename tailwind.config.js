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
