# KingsGambit2700 · Open Play

**Status: approved and integrated into the application, version 1.0.0.**

The user approved the brown design and requested application-wide integration on
8 September 2026. Preserve public content, all existing workflows, and the original
logo in a circular crop when extending the design.

## Source of truth

[brand.config.json](./brand.config.json) owns palette, typography, spacing, radii,
layout, motion, and shadow tokens. [brand.schema.json](./brand.schema.json) validates
the token shape using JSON Schema draft 2020-12. These files and this guide are
versioned with the application.

The application imports the config in `app/lib/brand.ts`. `getBrandStyles()` maps
tokens to CSS variables on `<html>` in `app/layout.tsx`, so initial renders use
the approved palette without a client-side color flash. Shared foundations live
in `app/brand.css`; public, portal, and classroom styles scope their own layouts.
Existing `--gold`, `--cream`, and font aliases resolve to these shared tokens.

Change `defaultAccent` to an existing `accents` key to select a palette.
Each accent needs all six color roles and a label. New accent names use lowercase
letters, digits, and hyphens. Check that
`defaultAccent` exists in `accents`: standard JSON Schema cannot express this
dynamic sibling-key relationship. Schema validation does not validate CSS syntax
or color contrast.

## Direction and hierarchy

Open Play presents chess as a curious, welcoming practice with room to grow.
The brown revision keeps the accepted layout and typography while drawing its
colors from the current application: `--gold-dark`, `--gold-light`, `--cream`,
`--paper`, and `--charcoal` in `app/globals.css`.
Expressive headlines and tactile chess imagery give the public landing page its
character. The parent dashboard prioritizes the next class, progress, and coach
notes. The classroom prioritizes the board, the lesson, and clear controls.

Use one dominant action per decision. Group supporting details by purpose, align
them to a clear grid, and reserve decoration for places where it carries the
academy's personality. Use specific, calm copy. Prototype students, class data,
and interactions illustrate the design; they are not production service claims.

## Color roles

| Role | Approved default | Use |
| --- | --- | --- |
| Primary / on-primary | Warm brown `#8C652D` / white `#FFFFFF` | Main actions and selected navigation |
| Highlight / on-highlight | Sand-gold `#D9C59A` / dark brown `#302419` | Occasional emphasis and playful details |
| Canvas / surface | `#F4F0E5` / `#FBF9F4` | Quiet page ground and grouped content |
| Ink / muted | `#2D2923` / `#6B645B` | Main text and supporting text |
| Dark / on-dark | `#201D18` / `#F4F0E5` | Classroom and strong sectional contrast |
| Primary-soft | `#F8F2E7` | Selected or related supporting surfaces |
| Line / subtle | `#D8D0BF` / `#EDE6D8` | Separators and low-emphasis structure |

Brown / Sand is the default. Cobalt / Citrus, Forest / Butter and Plum / Peach
remain available as alternative explorations in the same system.
Always change foreground and background pairs together. Semantic success, warning,
and danger stay independent from the selected brand accent. Pair statuses with
words or icons. Keep pale borders decorative; meaningful controls and focus must
remain distinguishable. Check every chosen text pair and interaction state for
appropriate contrast before adoption. The muted token is `#6B645B` so small
labels remain readable on both canvas and subtle surfaces. Use `onHighlight`
for text on sand-gold, rather than the primary brown.

The active hero, `public/brand/chess-sculpture-brown.png`, is a raster
illustration with brown/sand colors. It was edited from the original cobalt/citrus
artwork with the built-in image tool, preserving the composition and material.
Changing UI tokens does not recolor it. An approved alternate palette needs a
separately reviewed artwork variant. The board also has dedicated `boardLight` (`#EDDEC1`)
and `boardDark` (`#B08C5C`) tokens; accent switching does not change those tokens.

## CSS mapping

`app/lib/brand.ts` applies config values to the document's root custom properties.
Camel-case token names become kebab-case.

| Config path | CSS custom property |
| --- | --- |
| `colors.ink`, `colors.onDark` | `--ink`, `--on-dark` |
| `accents[selected].primaryHover` | `--primary-hover` |
| `accents[selected].onHighlight` | `--on-highlight` |
| `typography.display/body/mono` | `--font-display/body/mono` |
| `typography.displayWeight/bodyWeight` | `--font-display-weight/body-weight` |
| `typography.scale.hero` | `--text-hero` |
| `spacing.4` | `--space-4` |
| `radius.medium` | `--radius-medium` |
| `layout.contentMax` | `--layout-content-max` |
| `motion.fast`, `motion.ease` | `--motion-fast`, `--motion-ease` |
| `shadow.card` | `--shadow-card` |

Accent labels and descriptive metadata are not CSS tokens. Config values are
mapped globally; individual compositions still contain optical size and spacing adjustments.
Use the shared variables when extending the application.

## Type, space, and movement

- **Syne 600** gives headings an open, distinctive shape. Use **Manrope 400** for
  everyday reading, with heavier weights for actions. **IBM Plex Mono 400/500**
  supports dates, compact labels, and chess notation.
- Use the configured text scale as the starting point. Keep long text readable;
  small editorial labels should never carry essential instructions. Inputs should
  remain at least 16px on mobile.
- Space starts at 4px: 4, 8, 12, 16, 24, 32, 48, 64, and 96px. Prefer generous
  separation between subjects and tighter spacing within a related group.
- Use 8px corners for controls, 16px for cards, 28px for large feature surfaces,
  and pill corners only when the shape has a clear purpose.
- The content maximum is 1320px with a fluid gutter. The demo adapts around 1100px,
  850px, and 620px, with a large-screen adjustment at 1600px. Treat these as
  composition breakpoints; test intermediate widths and long content too.
- Fast feedback is 160ms, ordinary transitions 260ms, and entrance guidance 650ms.
  Animate opacity and transforms with the configured easing. Honor
  `prefers-reduced-motion`; do not make animation necessary to understand content.
- Keep keyboard focus visible, use semantic buttons/links, label form controls,
  preserve logical focus order, and target at least 44px touch areas. Dialogs need
  Escape handling and focus return. Display clear outcomes for actions.

Fonts are self-hosted in `public/brand/fonts/`, with each family's official
SIL Open Font License and provenance. The root layout loads `public/brand/fonts.css`;
no external font request is needed. Latin subsets use system fallbacks for
unsupported characters. Preserve the licenses.

`BrandLogo` uses the unchanged `public/logo.png` inside a circular, clipped
wrapper with a little padding to protect the original mark. Use this component
for headers, workspaces, authentication, and classrooms. Do not replace it with
a generated chess icon. Artwork provenance is recorded in [ARTWORK.md](./ARTWORK.md).

## Repeatable workflow for the next agent

1. Read the user's latest scope, this guide, the config, and the approved screens.
   The brown direction is approved; preserve existing data flows and access rules.
2. State who is using the screen, what they need next, and the primary action.
   Identify necessary populated, empty, loading, error, and disabled states.
3. Reuse the semantic tokens and visual hierarchy. Make any new design decision
   explicit in the proposal; avoid silently introducing another palette or font.
4. Render the result and inspect actual desktop and mobile screenshots. Check
   typography, balance, alignment, artwork crop, and density against this direction.
5. Exercise navigation, keyboard focus, dialogs, responsive behavior, and reduced
   motion. Check console errors and color contrast across every accent.
6. Ask a separate reviewer to assess coherence, originality, craft, and usability.
   Fix concrete findings, then record what was checked and any remaining limits.
7. Update this guide and config with accepted decisions so future work inherits
   the reasoning. Keep design experiments separate from runtime assets.

## Research applied

Research reviewed on 8 September 2026:

- [Vercel: Teaching agents product design](https://vercel.com/blog/teaching-agents-product-design-at-vercel)
  informed the durable config and decision guide, explicit proposal status, and
  emphasis on the user's task before visual details.
- [Vercel: Web Interface Guidelines](https://vercel.com/design/guidelines)
  informed visible focus, semantic controls, touch targets, reduced motion, and
  restrained CSS transitions.
- [Anthropic: Harness design for long-running applications](https://www.anthropic.com/engineering/harness-design-long-running-apps)
  informed separate creation and review responsibilities, distinctive art direction,
  and a rendered verification loop with concrete usability criteria.
