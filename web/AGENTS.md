# Web UI Agent Instructions

When changing the web UI, follow `../docs/design/linear-inspired.md` and `../docs/design/tokens.md`.

- Use `web/src/styles.css` `--fx-*` tokens as the source of truth for colors, surfaces, radius, and theme values.
- Keep dark mode as the default and preserve `.light` token support when adding new tokens.
- Prefer shadcn primitives in `src/components/ui` and Forklift app primitives in `src/components/app-ui`.
- Do not place custom app components in `src/components/ui`; that folder is reserved for shadcn-compatible primitives.
- Avoid one-off hex colors in React components. Add a token or app-ui variant instead.
- Do not add route/component styling as global CSS classes in `src/styles.css`; keep styling in Tailwind utilities, shadcn variants, app-ui variants, or local extracted components.
- Limit `src/styles.css` to Tailwind imports, font faces, theme tokens, keyframes, and minimal element defaults.
- Keep forklift yellow scarce: primary action, focus, active navigation, and urgent counters.
- Do not add decorative gradients, orbs, or marketing hero sections to authenticated app screens.
- Use `TableWrap` for data tables and keep responsive behavior explicit with `min-w-0`, `max-w-full`, and controlled overflow.
