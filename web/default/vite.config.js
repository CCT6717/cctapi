import { defineConfig, transformWithEsbuild } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'path';

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    // 让 Vite 在 import-analysis 之前对 .js 文件启用 JSX 解析
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
    // 兼容 REACT_APP_* 环境变量
    'process.env.REACT_APP_VERSION': JSON.stringify(process.env.REACT_APP_VERSION || ''),
    'process.env.REACT_APP_SERVER': JSON.stringify(process.env.REACT_APP_SERVER || ''),
  },
  test: {
    globals: true,
    environment: 'jsdom',
    include: ['src/**/*.test.{js,jsx}'],
    setupFiles: ['./vitest.setup.js'],
    deps: {
      inline: ['@storybook/react-vite'],
    },
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
});
