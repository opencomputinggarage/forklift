# Web Test Layout

Tests are grouped by test intent rather than by implementation file path.

- `unit/`: stable logic such as API request handling, formatting, and small pure helpers.
- `features/`: user-facing feature behavior with mocked APIs, rendered through React Testing Library.
- `e2e/`: Playwright browser tests for thin, high-value user flows.
- `utils/`: shared render helpers, API mocks, and fixtures.

Storybook stories stay under `src/stories/`; Storybook test-runner smoke checks are driven by
`pnpm test:storybook` and `pnpm test:storybook:ci`.

Prefer role, label, and user-event assertions over DOM structure, CSS classes, or snapshots while
the UX is still changing.
