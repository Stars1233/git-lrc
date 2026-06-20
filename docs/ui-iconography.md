# UI Iconography Guide

This document is the source of truth for action icons in git-lrc web UI surfaces.

It applies to:

- `internal/staticserve/static/` review UI
- `internal/staticserve/static/components/PrecommitBar.js`
- `internal/staticserve/static/ui-connectors/` manager UI

## Goals

- Keep icon choices consistent across all shipped web UI surfaces.
- Keep the UI visually aligned with the VS Code design language.
- Avoid ad hoc Unicode, emoji, and one-off SVG drift.
- Give agents and contributors one stable decision framework.

## Required Rendering Path

Use the shared icon registry in `internal/staticserve/static/components/icons.js`.

- Feature components should reference semantic aliases such as `sendToAgent`, `copyLogs`, or `filesTab`.
- Feature components should not hardcode raw icon paths, raw vendor glyph names, or new Unicode button symbols.
- New icons should be added to the registry first, then consumed through aliases.

## Selection Hierarchy

Choose icons in this order:

1. Use a semantic action icon that matches what the button does.
2. Use a brand icon only when the surface is representing vendor identity itself.
3. If a branded action still needs context, keep the semantic icon and rely on the text label for the brand name.
4. If no approved icon exists, add a curated self-hosted icon only after it is normalized to this system.
5. If none of the above is justified, keep the control text-first and do not invent a one-off glyph.

## Brand Icon Rules

Brand icons are not the default.

- Do not force a brand icon just because a button label includes a vendor name.
- Brand icons are appropriate for provider selectors, provider badges, connector identity rows, or auth identity surfaces.
- Brand icons are not the default for actions like send, retry, copy, connect, save, or delete.

Example:

- `Send to Claude` should use the semantic `sendToAgent` icon plus the text label `Claude`.
- A connector row representing Claude as a configured provider may use an approved Claude brand icon or normalized monogram.

## Visual Rules

- Use `currentColor` so icons follow the surrounding theme and button state.
- Default action icon size is `14px` unless the surrounding control system explicitly uses another size.
- Default stroke icons should keep a consistent optical weight. Do not mix heavy and light icon styles in the same control group.
- Icon-only controls must have `aria-label`.
- Decorative icons in text-plus-icon buttons should stay `aria-hidden`.
- Emoji must not be used for action buttons.
- Unicode symbols must not be used as the shipped icon system for action buttons.

## Approved Exceptions

These are allowed outside the main action icon system:

- Product logos and brand marks used for product identity.
- Larger decorative illustrations or empty-state artwork.
- Temporary migration leftovers only while actively being replaced in the same change series.

## Adding a New Icon

When adding a new icon:

1. Add or extend a semantic alias in `internal/staticserve/static/components/icons.js`.
2. Prefer a reusable action meaning over a feature-specific name.
3. If the request is brand-specific, first decide whether the surface is action-oriented or identity-oriented.
4. Add or update a deterministic JS test for the new alias or selection rule.
5. Use `make dev-ui` for manual verification and `make test-js` for deterministic regression coverage.

## Review Checklist

Before shipping UI icon changes, verify:

- no new emoji or Unicode button icons were introduced,
- new buttons use the shared icon registry,
- brand-specific buttons follow the hierarchy above,
- icon-only controls have labels,
- text-plus-icon controls still read cleanly,
- and the PR includes the required GIF or video walkthrough for UI work.