# Forklift Web Design Direction

Forklift uses a Linear-inspired product-console style adapted for repository operations. Do not copy Linear assets, CSS, or product UI. Treat the reference as a design direction: quiet surfaces, dense information, scarce accent color, and fast scanning.

## Principles

- Prefer operational clarity over decoration. The authenticated app should feel like a focused admin console, not a marketing page.
- Use the design tokens in `web/src/styles.css` as the source of truth. Do not introduce one-off hex colors in components unless a token cannot express the state.
- Keep forklift yellow scarce. Use it for primary actions, focus rings, active navigation, and high-priority counters only.
- Build pages from stable primitives: `PageHeader`, `PageDescription`, `Panel`, `TableWrap`, `Table`, `Badge`, and shadcn `Button`.
- Avoid global CSS class declarations for component styling. Keep global CSS limited to Tailwind imports, theme tokens, font faces, keyframes, and minimal element defaults.
- Favor hairline borders and subtle surface changes over heavy shadows.
- Keep radius restrained. Default panels and controls should stay near 8px unless a shadcn primitive requires otherwise.
- Tables and list views should be dense but readable: compact row height, muted metadata, clear hover state, and horizontal scrolling for narrow screens.
- Authenticated app screens should not use decorative gradient orbs, atmospheric blobs, or large illustrative hero sections.
- Login may use the subtle infrastructure grid/wave background, but keep animation low intensity and respect `prefers-reduced-motion`.

## Theme Model

The web UI is dark by default. Light mode is token-ready through a `.light` class on the document root. New components should use Tailwind/shadcn semantic colors (`bg-card`, `border-border`, `text-muted-foreground`, `text-primary`) or explicit `--fx-*` tokens where needed.

Preferred token families:

- `--fx-canvas`: page background.
- `--fx-surface-1`, `--fx-surface-2`, `--fx-surface-3`: panel, hover, and elevated surfaces.
- `--fx-border`, `--fx-border-strong`: hairline separators and stronger outlines.
- `--fx-text`, `--fx-text-muted`, `--fx-text-subtle`: text hierarchy.
- `--fx-accent`, `--fx-accent-hover`, `--fx-accent-pressed`: forklift yellow accent.
- `--fx-success`, `--fx-warning`, `--fx-danger`: status colors.
- `--fx-radius-*`: component radius scale.

## Component Rules

- `components/ui/*` stays close to shadcn primitives.
- `components/app-ui/*` expresses Forklift product UI defaults.
- Page routes should compose app-ui components instead of redefining cards, tables, or badges.
- If a component needs a special visual state, add a named variant or token before adding ad hoc classes across routes.
- Prefer Tailwind utilities, shadcn variants, or app-ui component variants over custom classes in `styles.css`.
- Complex one-off visuals should keep their styling next to the JSX with scoped constants or extracted components, not shared global selectors.
- Use `TableWrap` for every data table that can exceed mobile width.
- Use `Panel` for repeated or framed operational sections. Do not nest panels inside panels.

## Responsive Rules

- Page content must stay within viewport width. Prefer `min-w-0`, `max-w-full`, and `overflow-x-auto` on dense data.
- On mobile, page header actions should wrap and share width.
- Avoid hiding critical actions behind hover-only affordances.
- Text in buttons, badges, and navigation must not overlap adjacent content.

## Anti-Patterns

- Hardcoded product colors in React components.
- Decorative gradients inside authenticated app screens.
- Large marketing-style hero layouts for app pages.
- Floating cards used as page sections.
- One-off table styling outside `components/app-ui/table.tsx`.
- Route-level visual classes in `web/src/styles.css`.
