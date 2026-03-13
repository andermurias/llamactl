# Changelog

All notable changes to **llamactl** are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

---

## [v1.3.4] — 2026-03-13

### Fixed
- Desktop layout: removed duplicate `margin-left` on `.main` that added a
  blank gap between the sidebar and content area at ≥ 768 px viewport width.

---

## [v1.3.3] — 2026-03-13

### Changed — Web UI complete redesign
- Replaced three parallel navigation systems (sidebar + tablet tab-bar +
  mobile bottom-nav) with a **single unified sidebar** that works at every
  breakpoint.
- Desktop (≥ 768 px): sidebar is always visible (`position: sticky`).
- Mobile (< 768 px): sidebar slides in from the left via hamburger button;
  a blurred overlay closes it on tap outside.
- Design system rewritten from scratch — no Tailwind CDN dependency:
  - CSS custom properties for all colours and spacing.
  - Minimal dark palette inspired by Claude/OpenAI (bg `#0f0f10`, surface
    `#1c1c1e`, text `#f5f5f7`).
  - Consistent component classes: `.card`, `.btn`, `.badge`, `.stat`,
    `.dot`, `.info-dl`, `.log-box`, `.code-editor`.
- Template reduced from 1187 → 772 lines (−35 %).

---

## [v1.3.2] — 2026-03-11

### Added
- Mobile-first responsive layout with bottom navigation bar.
- `viewport-fit=cover` + `env(safe-area-inset-bottom)` for iOS notch support.
- Touch-friendly buttons (min 40 px tap targets, `touch-action: manipulation`).

---

## [v1.3.1] — 2026-03-10

### Added
- `llamactl config` subcommand group: `edit`, `show`, `validate`, `path`,
  `reload` (sends SIGHUP to live llama-swap process).
- `GET /api/config` — read `llama-swap.yaml` as JSON.
- `POST /api/config` — write new YAML content; llama-swap auto-reloads via
  `--watch-config`.
- Config editor panel in the web dashboard with load / save buttons and an
  unsaved-changes indicator.

---

## [v1.3.0] — 2026-03-09

### Added
- **Embedded web UI** at `http://localhost:3333`.
  - Service control (start / stop / restart llama-swap and ComfyUI).
  - Model list with loaded/idle status badges.
  - Live log viewer (5 s polling, auto-scroll).
  - Summary stat row (API status, ComfyUI status, model count, in-memory count).
- `llamactl web` subcommand group: `start`, `stop`, `restart`, `status`,
  `enable`, `disable`, `install`, `uninstall`, `logs`, `serve`.
- Separate `com.llamastack.llamactl-web` LaunchAgent plist.
- `//go:embed` bundles templates + static assets into a single binary.

---

## [v1.2.x] — 2026-02-xx

### Added
- Git-based self-upgrade (`llamactl upgrade --self`): `git pull` → rebuild →
  re-sign → redeploy.  Removed GitHub release download dependency.
- Makefile overhaul: `build` compiles and ad-hoc code-signs; `install` deploys
  to `../scripts/llamactl`; `dist` = build + install.
- `llamactl upgrade --check` reports available updates without upgrading.

---

## [v1.1.x] — 2026-01-xx

### Added
- `llamactl comfyui` subcommand: `start`, `stop`, `status`, `logs`.
- ComfyUI LaunchAgent (`com.llamastack.comfyui` service lifecycle).
- `llamactl models` — unified model listing across API, GGUF files, and
  HuggingFace cache.

---

## [v1.0.0] — 2025-12-xx

### Added
- Initial Go rewrite replacing the legacy Bash `llamactl` script.
- Full `launchd` integration: plist generation, `bootstrap`/`bootout`,
  `kickstart`, auto-start enable/disable.
- Commands: `start`, `stop`, `restart`, `status`, `enable`, `disable`,
  `logs`, `version`, `upgrade`.
- `github.com/pterm/pterm` for rich terminal output (spinners, tables,
  status badges).
- `github.com/spf13/cobra` for structured command hierarchy.
- Unit tests for service layer and updater.
