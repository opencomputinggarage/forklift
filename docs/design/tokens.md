# Web Design Tokens

The canonical design tokens live in `web/src/styles.css` as CSS custom properties. Tailwind 4 reads them through `@theme inline`, so shadcn and app-ui components share the same palette.

## Token Layers

`--fx-*` tokens are the source of truth.

Legacy aliases such as `--bg`, `--panel`, `--text`, and `--accent` remain only for older scoped CSS and should not be used in new React component code.

## Color Tokens

| Token | Purpose |
| --- | --- |
| `--fx-canvas` | Page background. |
| `--fx-canvas-elevated` | Slightly raised background, mostly for gradients or shell surfaces. |
| `--fx-surface-1` | Panels, cards, sidebar. |
| `--fx-surface-2` | Muted fills, hover surfaces, secondary controls. |
| `--fx-surface-3` | Elevated popovers or specialized visualizations. |
| `--fx-input` | Inputs and code-like read-only fields. |
| `--fx-border` | Default hairline border. |
| `--fx-border-strong` | Stronger outlines and dividers. |
| `--fx-text` | Primary readable text. |
| `--fx-text-muted` | Secondary text and metadata. |
| `--fx-text-subtle` | Low emphasis labels and helper marks. |
| `--fx-accent` | Forklift yellow, used sparingly. |
| `--fx-success`, `--fx-warning`, `--fx-danger` | Status states. |

## Theme Modes

Dark mode is the default:

```html
<html>
```

Light mode can be enabled by applying `.light` to the document root:

```html
<html class="light">
```

Use semantic Tailwind colors where possible:

```tsx
<section className="border border-border bg-card text-card-foreground" />
<span className="text-muted-foreground" />
<button className="bg-primary text-primary-foreground" />
```

Use direct tokens only for visuals that Tailwind semantic colors cannot describe:

```tsx
<div className="shadow-[var(--fx-overlay-shadow)]" />
```

## Contribution Rules

- Add a token or component variant before scattering new color values.
- Keep both dark and light values when adding a new `--fx-*` token.
- Do not add global CSS classes for route or component styling. Use Tailwind utilities, shadcn/app-ui variants, or a local component instead.
- Keep global CSS focused on imports, `@font-face`, `@theme`, `:root`/theme tokens, keyframes, and minimal element defaults.
- Do not use `--fx-accent` for ordinary hover states or passive decoration.
- Do not add gradients to authenticated app screens unless the design document explicitly allows the surface.
