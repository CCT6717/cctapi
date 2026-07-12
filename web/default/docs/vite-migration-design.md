# CRA → Vite Migration Design

## 目标
将 `web/default` 从 Create React App (CRA) 5.0.1 迁移到 Vite + Vitest + Storybook Vite builder，完整移除 react-scripts、Webpack 和 Workbox 依赖链。

## 方案
兼容优先，使用 Vite 6.4.x、Vitest 3.2.x、Storybook 8.6.14（仅 builder 切换）。

## 环境确认
- Node 24.15.0 ✅ 满足 Vite 6.4.x 要求

## 关键变更清单

### 1. 依赖替换
- 移除 `react-scripts` 及 ESLint 隐式依赖
- 新增 `vite@6.4.x`、`@vitejs/plugin-react`
- 新增 `vitest@3.2.x`、`jsdom`、`@testing-library/react`、`@testing-library/jest-dom`
- Storybook 移除 `@storybook/preset-create-react-app`、`@storybook/react-webpack5`
- Storybook 新增 `@storybook/react-vite@8.6.14`、`@storybook/builder-vite`

### 2. 配置文件
- `vite.config.js`：React 插件、JSX-in-`.js`、代理 `/api`/`/v1`/`/metrics` → 3008、输出 `build/`
- `vitest.config.js`：jsdom 环境、全局 API、测试匹配 `*.test.js`

### 3. HTML 入口
- 将 `public/index.html` 移至前端根目录
- 替换 `%PUBLIC_URL%` 为 Vite 入口脚本标签

### 4. 环境变量
- 兼容 `REACT_APP_*`（通过 Vite `envPrefix` 或 `define`）
- 同时支持 `VITE_*` 新变量

### 5. 测试迁移
- `jest` → `vi`（Vitest 兼容模式）
- 更新 `setupTests.js` 或 `vitest.setup.js`
- 保留 35 项测试语义不变

### 6. Storybook 迁移
- `.storybook/main.js` 移除 `react-webpack5` 框架，改用 `react-vite`
- 移除 CRA preset

### 7. ESLint
- 改为显式依赖 `eslint`、`eslint-plugin-react` 等
- 移除 `react-app` 和 `react-app/jest` extends

### 8. package.json 脚本
- `start` → `vite`
- `build` → `vite build && node scripts/sync-build.js`
- `test` → `vitest run`
- `test:watch` → `vitest`
- `storybook` / `build-storybook` 保持不变（Storybook CLI 自动适配）

### 9. 构建后验证
1. `npm ci`
2. ESLint 检查
3. 35 项测试
4. Vite 生产构建
5. Storybook 构建
6. `npm audit`
7. Go 全量测试 + 二进制重编译
8. 浏览器验收（桌面+移动）
9. 真实 `openrouter/auto` smoke（如 Token 可用）

## 风险与回退
- 回退：保留 `package.json` 备份，Vite 构建失败可立即回退 `npm install` 旧依赖
- 风险：Vitest 与 Jest 全局 API 差异（`vi.fn()` 替代 `jest.fn()`）
- 缓解：使用 `globals: true` 保留 Jest 风格，逐步替换
