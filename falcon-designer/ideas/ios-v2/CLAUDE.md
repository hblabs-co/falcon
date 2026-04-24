# Falcon iOS — Project Rules for Claude Code

## Context

Falcon is an iOS app (iOS 17+, SwiftUI) for AI-powered job matching, targeting candidates primarily. Backend is a Go microservices platform (falcon-auth, falcon-cv-ingest, falcon-scout, falcon-dispatch, falcon-match-engine, falcon-signal) communicating over NATS JetStream.

## Design system — always consult `DESIGN_SYSTEM.md`

Before creating or modifying **any UI component, screen, or visual element**, read `DESIGN_SYSTEM.md` at the repo root. That document is the single source of truth for:

- Color tokens (light and dark variants)
- Typography scale and weights
- Spacing scale (4pt base unit)
- Corner radii
- Component anatomy and dimensions
- Logo usage

### Strict rules

1. **Never hardcode colors in Swift files.** All colors must be defined as Color Sets in `Assets.xcassets` with both light and dark appearances. Reference them through the `Color` extension (`Color.brandPrimary`, not `Color(hex: "#1656D6")`).

2. **Never use magic spacing or size values.** If `DESIGN_SYSTEM.md` defines `space4 = 16`, use `Spacing.space4` — not `16`. If you need a value that isn't defined, stop and propose adding it to the spec first.

3. **Never introduce fonts or font weights outside the defined set.** Only `.regular` (400), `.medium` (500), `.semibold` (600). Only the sizes defined in section 3.1 of the spec.

4. **Every component needs a `#Preview`** showing: default state, interactive states where applicable, and both light and dark mode side by side.

5. **Minimum tap target is 44×44pt.** Every interactive element must meet this.

6. **Use SF Symbols for icons** wherever possible. Only import custom SVG icons if SF Symbols genuinely lacks what you need, and propose it before adding.

## Code style

- SwiftUI first. No UIKit wrappers unless absolutely required.
- No external dependencies without explicit approval — the design system must work with zero third-party packages.
- Every new file begins with a brief header comment: file purpose, primary public API.
- Prefer `struct` over `class`. Prefer value semantics.
- Group related modifiers with `.modifier(...)` custom ViewModifiers when a combination repeats 2+ times.

## Project structure

Follow the structure in section 8 of `DESIGN_SYSTEM.md`. Design system code lives in `Sources/DesignSystem/`. Feature code lives in `Sources/Features/<FeatureName>/`. Features **consume** design system tokens and components — they never define their own.

## Workflow expectations

- When asked to build a screen, build the primitives first (tokens, components) if they don't exist, then compose.
- Validate that the project compiles after each significant change.
- When uncertain about a design decision not covered in the spec, **ask** — don't invent. The spec can be updated, but should stay authoritative.
- Match the existing code style of neighboring files.

## Logo assets

Logo files live in `logo/`:
- SVGs in `logo/svg/` (primary, dark, monochrome, wordmark)
- PNG exports in `logo/png/` (including `falcon-icon-1024.png` for App Store Connect)

For the app icon in `Assets.xcassets/AppIcon.appiconset/`, use `falcon-icon-1024.png`. Generate all required sizes from that source — iOS 17+ only needs the 1024 asset with automatic sizing, but confirm the target iOS version before simplifying.
