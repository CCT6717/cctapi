# UI Modernization Design

## Goal

Modernize the cctapi admin frontend with a mature React UI foundation while keeping the current app stable and deployable at every step.

## Selected Stack

- Ant Design is the primary UI framework for new and migrated admin screens.
- Lucide React is the icon system for new internal UI components.
- Storybook documents and verifies reusable UI building blocks.
- Semantic UI React remains installed during migration because existing pages depend on it heavily.

## Scope

This first slice builds the UI foundation and applies it to the Fallback / Free Pool area only. It does not migrate every legacy page, change backend APIs, or remove Semantic UI.

## Architecture

Create a small internal UI layer under `web/default/src/ui/`. Business pages should consume these wrappers instead of importing AntD directly everywhere. The first wrapper set covers layout, cards, page headers, status tags, metric cards, toolbars, and icon buttons. Fallback pages can migrate to these components while their existing data loading and save logic remains unchanged.

## Visual Direction

The admin UI should feel dense, calm, and operational. Use restrained surfaces, clear hierarchy, small rounded corners, compact spacing, and status colors only where they carry state. Avoid marketing-style heroes, decorative gradients, and oversized cards. Tables and forms should prioritize scanning and repeated operation.

## Migration Rules

- Keep Semantic UI and AntD coexisting until each page is migrated.
- Do not rewrite business hooks or API clients for visual-only work.
- Do not submit generated `web/build/default` files unless explicitly preparing a local preview.
- Keep Fallback / Free Pool behavior unchanged.
- Prefer Lucide icons in new internal UI components.
- Use Storybook stories for reusable UI components, not whole app pages.

## Success Criteria

- `antd`, `lucide-react`, and Storybook are available in the frontend project.
- AntD `ConfigProvider` wraps the app with cctapi theme tokens.
- `web/default/src/ui/` exposes reusable UI primitives.
- Fallback / Free Pool outer layout uses the new UI layer.
- Existing frontend tests pass.
- `npm run build` exits 0.
- Local preview on `http://127.0.0.1:3008/` can be rebuilt after the UI slice.
