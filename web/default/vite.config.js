import { defineConfig, loadEnv, transformWithEsbuild } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');
  const appVersion =
    process.env.VITE_APP_VERSION ||
    process.env.REACT_APP_VERSION ||
    env.VITE_APP_VERSION ||
    env.REACT_APP_VERSION ||
    '';
  const serverUrl =
    process.env.VITE_SERVER_URL ||
    process.env.REACT_APP_SERVER ||
    env.VITE_SERVER_URL ||
    env.REACT_APP_SERVER ||
    '';

  return {
    plugins: [
      // Transform legacy .js files containing JSX before import analysis.
      {
        name: 'jsx-in-js',
        enforce: 'pre',
        async transform(code, id) {
          if (id.endsWith('.js') && !id.includes('node_modules')) {
            return transformWithEsbuild(code, id, {
              loader: 'jsx',
              jsx: 'automatic',
            });
          }
        },
      },
      react({
        include: [/\.js$/, /\.jsx$/, /\.ts$/, /\.tsx$/],
      }),
    ],
    root: process.cwd(),
    publicDir: 'public',
    build: {
      outDir: 'build',
      assetsDir: 'static',
      sourcemap: true,
      emptyOutDir: true,
      // The existing SPA ships as one application bundle; route-level lazy
      // loading is a separate optimization and should not be faked with
      // circular vendor chunks.
      chunkSizeWarningLimit: 1600,
    },
    server: {
      port: 3000,
      open: false,
      proxy: {
        '/api': 'http://localhost:3008',
        '/v1': 'http://localhost:3008',
        '/metrics': 'http://localhost:3008',
      },
    },
    define: {
      'import.meta.env.VITE_APP_VERSION': JSON.stringify(appVersion),
      'import.meta.env.VITE_SERVER_URL': JSON.stringify(serverUrl),
    },
    test: {
      globals: true,
      environment: 'jsdom',
      include: ['src/**/*.test.{js,jsx}'],
      setupFiles: ['./vitest.setup.js'],
    },
  };
});
