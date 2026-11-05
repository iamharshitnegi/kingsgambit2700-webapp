import type { Config } from "tailwindcss";

const config: Config = {
  content: ["./app/**/*.{js,ts,jsx,tsx,mdx}", "./sanity/**/*.{js,ts,jsx,tsx}"],
  theme: {
    extend: {
      colors: {
        brand: {
          cream: "#f4f0e5",
          paper: "#fbf9f4",
          charcoal: "#201d18",
          gold: "#b1843c",
          goldLight: "#d9c59a",
          border: "#d8d0bf",
          muted: "#6f685f",
          dark: "#201d18",
          accent: "#b1843c",
          light: "#f4f0e5",
          primary: "#201d18",
        },
      },
      fontFamily: {
        sans: ["var(--font-inter)", "sans-serif"],
        serif: ["var(--font-merriweather)", "serif"],
      },
    },
  },
  plugins: [],
};

export default config;
