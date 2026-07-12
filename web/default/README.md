# CCT API Frontend

## Basic Usages

```shell
# Install the locked dependencies
npm ci

# Run the Vite development server
npm start

# Run lint and tests
npm run lint
npm test

# Build and sync production assets to `../build/default`
npm run build
```

Set `VITE_SERVER_URL` to use an API server other than the current origin. Existing deployments may continue to use `REACT_APP_SERVER` as a compatibility fallback.

Set `VITE_APP_VERSION` to display and compare the frontend version. `REACT_APP_VERSION` remains supported as a compatibility fallback.

Before you start editing, make sure your `Actions on Save` options have `Optimize imports` & `Run Prettier` enabled.

## References

1. https://github.com/OIerDb-ng/OIerDb
2. https://github.com/cornflourblue/react-hooks-redux-registration-login-example
