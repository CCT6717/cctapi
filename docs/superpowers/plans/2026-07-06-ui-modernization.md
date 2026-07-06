# UI Modernization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an Ant Design + Lucide + Storybook UI foundation and use it for the Fallback / Free Pool shell without changing backend behavior.

**Architecture:** Keep Semantic UI and AntD in coexistence mode. Add `web/default/src/ui/` as a small internal design layer, wrap the app with AntD theme tokens, then migrate only the Fallback outer shell and reusable Free Pool surfaces to the new layer.

**Tech Stack:** React 18, CRA 5, Ant Design 6, Lucide React, Storybook React Webpack 5, existing Jest/React Scripts tests.

## Global Constraints

- Keep Semantic UI installed and working during this slice.
- Do not change backend APIs.
- Do not change Fallback / Free Pool data loading, save, sync, cleanup, health, or usage behavior.
- Do not commit generated `web/build/default` artifacts.
- Keep local preview available on port `3008`; restart only after build.
- Use compact admin styling, not marketing-page visuals.

---

### Task 1: Install UI Dependencies

**Files:**
- Modify: `web/default/package.json`
- Do not commit: `web/default/package-lock.json` because `web/default/.gitignore` explicitly ignores it in this repo.

**Interfaces:**
- Produces: `antd`, `lucide-react`, `storybook`, `@storybook/react-webpack5`, and `@storybook/addon-essentials`.

- [ ] **Step 1: Install runtime dependencies**

Run:

```powershell
Set-Location D:\ct\project\web\default
npm install antd lucide-react
```

Expected: install exits 0.

- [ ] **Step 2: Install Storybook dev dependencies**

Run:

```powershell
Set-Location D:\ct\project\web\default
npm install -D storybook @storybook/react-webpack5 @storybook/addon-essentials
```

Expected: install exits 0.

- [ ] **Step 3: Add scripts**

Ensure `web/default/package.json` contains:

```json
"storybook": "storybook dev -p 6006",
"build-storybook": "storybook build"
```

### Task 2: AntD Theme Provider

**Files:**
- Create: `web/default/src/ui/theme.js`
- Modify: `web/default/src/index.js`

**Interfaces:**
- Produces: `cctAntdTheme`, an object consumed by AntD `ConfigProvider`.

- [ ] **Step 1: Create theme tokens**

Create `web/default/src/ui/theme.js`:

```javascript
export const cctAntdTheme = {
  token: {
    colorPrimary: '#2563eb',
    colorSuccess: '#16a34a',
    colorWarning: '#d97706',
    colorError: '#dc2626',
    colorInfo: '#2563eb',
    borderRadius: 8,
    fontFamily:
      "Inter, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Noto Sans SC', 'PingFang SC', 'Microsoft YaHei', sans-serif",
  },
  components: {
    Card: {
      borderRadiusLG: 8,
    },
    Button: {
      borderRadius: 8,
      controlHeight: 34,
    },
    Table: {
      borderRadius: 8,
      headerBg: '#f8fafc',
      headerColor: '#475569',
    },
    Tag: {
      borderRadiusSM: 999,
    },
  },
};
```

- [ ] **Step 2: Wrap app with ConfigProvider**

Modify `web/default/src/index.js` to import:

```javascript
import { ConfigProvider } from 'antd';
import 'antd/dist/reset.css';
import { cctAntdTheme } from './ui/theme';
```

Wrap the existing providers inside:

```jsx
<ConfigProvider theme={cctAntdTheme}>
  <StatusProvider>
    ...
  </StatusProvider>
</ConfigProvider>
```

### Task 3: Internal UI Components

**Files:**
- Create: `web/default/src/ui/index.js`
- Create: `web/default/src/ui/AdminCard.jsx`
- Create: `web/default/src/ui/PageHeader.jsx`
- Create: `web/default/src/ui/StatusTag.jsx`
- Create: `web/default/src/ui/MetricCard.jsx`
- Create: `web/default/src/ui/ActionToolbar.jsx`
- Create: `web/default/src/ui/ui.css`

**Interfaces:**
- Produces: reusable components imported from `../../ui`.

- [ ] **Step 1: Add UI stylesheet**

Create `web/default/src/ui/ui.css` with compact admin styles for `.cct-page-header`, `.cct-admin-card`, `.cct-metric-card`, and `.cct-action-toolbar`.

- [ ] **Step 2: Add component wrappers**

Create components wrapping AntD `Card`, `Tag`, `Space`, `Button`, and Lucide icons. Components must accept `children`, `className`, and common AntD props without hiding escape hatches.

- [ ] **Step 3: Export components**

Create `web/default/src/ui/index.js` exporting all UI components and import `./ui.css`.

### Task 4: Storybook Setup

**Files:**
- Create: `web/default/.storybook/main.js`
- Create: `web/default/.storybook/preview.js`
- Create: `web/default/src/ui/AdminCard.stories.jsx`
- Create: `web/default/src/ui/StatusTag.stories.jsx`
- Create: `web/default/src/ui/MetricCard.stories.jsx`

**Interfaces:**
- Produces: Storybook stories for reusable UI components.

- [ ] **Step 1: Add Storybook config**

Create `main.js` using `@storybook/react-webpack5` and `@storybook/addon-essentials`.

- [ ] **Step 2: Add preview provider**

Wrap stories in AntD `ConfigProvider` with `cctAntdTheme`.

- [ ] **Step 3: Add component stories**

Add basic stories for card, status tags, and metric cards.

### Task 5: Fallback Shell Migration

**Files:**
- Modify: `web/default/src/pages/Fallback/index.js`
- Modify: `web/default/src/pages/Fallback/Fallback.css`

**Interfaces:**
- Consumes: `PageHeader`, `AdminCard`, `ActionToolbar`, `StatusTag`.
- Produces: same Fallback route behavior with a cleaner AntD/Lucide shell.

- [ ] **Step 1: Replace Semantic UI shell controls**

Use AntD `Button`, `Tooltip`, and Lucide `RefreshCw`, `HelpCircle`, and panel icons in the Fallback shell. Do not change child panels yet.

- [ ] **Step 2: Wrap main blocks**

Use `PageHeader` for the page title/actions and `AdminCard` for guide, nav, and content surfaces.

- [ ] **Step 3: Preserve existing panel routing**

Keep `PANEL_ITEMS`, `activePanel`, `renderActivePanel`, and all hook calls unchanged.

### Task 6: Verification, Commit, Preview

**Files:**
- Source files from Tasks 1-5.

**Interfaces:**
- Produces: tested frontend and refreshed local preview.

- [ ] **Step 1: Run tests**

Run:

```powershell
Set-Location D:\ct\project\web\default
npm test -- --watchAll=false
```

Expected: 6 suites / 21 tests pass.

- [ ] **Step 2: Run build**

Run:

```powershell
npm run build
```

Expected: exits 0. Existing unrelated warnings may remain.

- [ ] **Step 3: Build Storybook**

Run:

```powershell
npm run build-storybook
```

Expected: exits 0.

- [ ] **Step 4: Commit source changes only**

Run:

```powershell
git add web/default/.gitignore web/default/package.json web/default/.storybook web/default/src/ui web/default/src/index.js web/default/src/pages/Fallback/index.js web/default/src/pages/Fallback/Fallback.css web/default/src/pages/Fallback/Fallback.test.js docs/superpowers/specs/2026-07-06-ui-modernization-design.md docs/superpowers/plans/2026-07-06-ui-modernization.md
git commit -m "feat: add ui modernization foundation"
```

- [ ] **Step 5: Restart preview**

Run:

```powershell
Set-Location D:\ct\project
go build -o one-api.exe .
```

Restart `one-api.exe --port 3008` and verify `http://127.0.0.1:3008/` returns HTTP 200.
