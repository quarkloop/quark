import type { Config } from "tailwindcss";

/**
 * Tailwind configuration for the Quark Platform documentation.
 *
 * Design language: warm, rich, premium. Like a well-worn leather notebook
 * with brass accents. Solid colors only — no gradients.
 *
 * The palette is built on two axes:
 *   - "sand": warm neutral backgrounds and text (cream → espresso)
 *   - "ember": warm accent (butter → deep rust)
 *
 * Both axes use HSL hues in the 25-35 range (orange/amber territory) so
 * every color feels like it belongs to the same warm world.
 */
const config: Config = {
  content: [
    "./app/**/*.{ts,tsx,mdx}",
    "./components/**/*.{ts,tsx}",
    "./content/**/*.{md,mdx}",
    "./lib/**/*.{ts,tsx}",
    "./mdx-components.tsx",
    "./node_modules/fumadocs-ui/dist/**/*.js",
  ],
  darkMode: "class",
  theme: {
    extend: {
      fontFamily: {
        sans: ["var(--font-inter)", "system-ui", "sans-serif"],
        mono: ["var(--font-jetbrains)", "ui-monospace", "monospace"],
        display: ["var(--font-inter)", "system-ui", "sans-serif"],
      },
      colors: {
        // Warm neutral base — "sand" scale (cream → espresso)
        // Every shade has a warm undertone (hue ~30-35), never cold gray.
        sand: {
          50: "#fdf8f0", // warm cream — light mode background
          100: "#f9f0e3", // warm ivory — card backgrounds
          200: "#f0e4d0", // warm beige — borders, dividers
          300: "#e2d2b8", // warm tan — muted accents
          400: "#c4a87a", // warm gold-tan — secondary text (light)
          500: "#9a7d54", // warm bronze — secondary text (dark)
          600: "#6b5538", // warm brown — primary text (dark mode)
          700: "#4a3a26", // warm espresso — headings (dark mode)
          800: "#2d2218", // warm dark brown — dark mode card bg
          900: "#1c1610", // warm near-black — dark mode background
          950: "#120e0a", // warm deepest — dark mode deepest
        },
        // Warm accent — "ember" scale (butter → deep rust)
        // Richer and more saturated than the previous amber.
        ember: {
          50: "#fef3e7", // pale butter
          100: "#fde4c8", // light cream
          200: "#fac892", // warm gold
          300: "#f7a854", // warm orange
          400: "#f58a2e", // vibrant orange — hover state
          500: "#e87015", // rich orange — primary accent
          600: "#c75a0e", // deep orange — active state
          700: "#9e420c", // rust — dark accents
          800: "#7d3610", // dark rust
          900: "#5e2b10", // deepest rust
          950: "#331508", // near-black rust
        },
      },
      boxShadow: {
        // Warm-tinted shadows (not cold gray)
        "warm-sm": "0 1px 2px 0 rgba(74,58,38,0.08), 0 1px 3px 0 rgba(74,58,38,0.06)",
        "warm": "0 2px 4px 0 rgba(74,58,38,0.08), 0 4px 12px -2px rgba(74,58,38,0.10)",
        "warm-lg": "0 4px 8px -2px rgba(74,58,38,0.10), 0 12px 32px -4px rgba(74,58,38,0.14)",
        "warm-xl": "0 8px 16px -4px rgba(74,58,38,0.12), 0 24px 48px -8px rgba(74,58,38,0.18)",
        "glow": "0 0 32px -8px rgba(232,112,21,0.40)",
        "glow-sm": "0 0 16px -4px rgba(232,112,21,0.30)",
      },
      transitionTimingFunction: {
        premium: "cubic-bezier(0.16, 1, 0.3, 1)",
      },
      animation: {
        "fade-in": "fade-in 0.4s cubic-bezier(0.16, 1, 0.3, 1) both",
        "slide-up": "slide-up 0.5s cubic-bezier(0.16, 1, 0.3, 1) both",
      },
      keyframes: {
        "fade-in": {
          "0%": { opacity: "0" },
          "100%": { opacity: "1" },
        },
        "slide-up": {
          "0%": { opacity: "0", transform: "translateY(8px)" },
          "100%": { opacity: "1", transform: "translateY(0)" },
        },
      },
    },
  },
};

export default config;
