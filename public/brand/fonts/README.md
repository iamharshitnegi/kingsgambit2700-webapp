# Self-hosted application fonts

Downloaded 8 September 2026 from official Google Fonts CSS and `fonts.gstatic.com`.
The application requires no external font network requests at runtime. These are Latin
subsets; unsupported scripts and symbols use the configured system fallback fonts.

The root layout imports `public/brand/fonts.css`. The files are:

- `syne-latin-variable.woff2` — Syne, normal, variable weights 400–800.
- `manrope-latin-variable.woff2` — Manrope, normal, variable weights 400–800.
- `ibm-plex-mono-latin-400.woff2` — IBM Plex Mono, normal, weight 400.
- `ibm-plex-mono-latin-500.woff2` — IBM Plex Mono, normal, weight 500.

SIL Open Font License texts are included as `OFL-Syne.txt`, `OFL-Manrope.txt`, and
`OFL-IBMPlexMono.txt`, copied from the corresponding family directories in the
[official Google Fonts repository](https://github.com/google/fonts/tree/main/ofl).
No font files were modified.

Original stylesheet request:

```text
https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:wght@400;500&family=Manrope:wght@400..800&family=Syne:wght@400..800&display=swap
```

Original font files:

```text
https://fonts.gstatic.com/s/syne/v24/8vIH7w4qzmVxm2BL9A.woff2
https://fonts.gstatic.com/s/manrope/v20/xn7gYHE41ni1AdIRggexSg.woff2
https://fonts.gstatic.com/s/ibmplexmono/v20/-F63fjptAgt5VM-kVkqdyU8n1i8q1w.woff2
https://fonts.gstatic.com/s/ibmplexmono/v20/-F6qfjptAgt5VM-kVkqdyU8n3twJwlBFgg.woff2
```
