# CRA to Vite Migration Design

## Goal

Migrate `web/default` from Create React App 5.0.1 to Vite, Vitest, and the Storybook Vite builder while preserving the existing React application behavior and Go embed output path.

## Selected Stack

- Vite 6.4.x
- Vitest 3.2.x with jsdom
- Storybook 10.5.0 using `@storybook/react-vite`
- ESLint 9 flat configuration
- Node.js 20.19 or newer; the verified Windows environment uses Node.js 24

## Build Boundary

- `index.html` lives at the frontend root and loads `/src/index.js`.
- Vite writes production files to `web/default/build`.
- `scripts/sync-build.js` moves that output to `web/build/default`.
- Go embeds `web/build/*`; the Go binary must be rebuilt after every frontend production build.

## Development Server

Vite runs on port 3000 and proxies these paths to the local Go service on port 3008:

- `/api`
- `/v1`
- `/metrics`

Legacy source files containing JSX keep their `.js` extension. A pre-transform plugin converts those files before Vite import analysis.

## Environment Variables

Application code reads:

- `VITE_APP_VERSION`
- `VITE_SERVER_URL`

The Vite config also accepts `REACT_APP_VERSION` and `REACT_APP_SERVER` as compatibility fallbacks for existing deployment environments. Values may come from the process environment or Vite `.env` files.

## Tests

- `npm test` runs Vitest once.
- `npm run test:watch` starts watch mode.
- Tests use jsdom and `vitest.setup.js` for DOM cleanup.
- Jest mock APIs are migrated to Vitest `vi` APIs.
- Focused tests are passed as file paths after `npm test --`.

## Storybook

Storybook uses `@storybook/react-vite`; CRA and Webpack presets are removed. Existing stories and decorators remain unchanged.

## Linting

`npm run lint` executes ESLint with `--max-warnings=0`. A release-ready tree must have zero errors and zero warnings.

## Acceptance

1. `npm ci`
2. `npm run lint`
3. `npm test`
4. `npm run build`
5. `npm run build-storybook`
6. `npm audit`
7. Full Go tests and binary rebuild
8. Browser checks at desktop and mobile viewports
9. Real `openrouter/auto` smoke test

## Rollback

The migration is isolated to frontend tooling, test syntax, generated frontend assets, and documentation. If a release regression is found, revert the migration commits and rebuild `one-api.exe` from the restored CRA build output.
