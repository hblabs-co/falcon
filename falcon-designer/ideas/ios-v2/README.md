# Falcon Design Package

Everything needed to implement Falcon's visual system in iOS with Claude Code.

## Contents

```
falcon-design/
├── DESIGN_SYSTEM.md              # The spec — single source of truth
├── CLAUDE.md                     # Permanent rules for Claude Code
├── README.md                     # This file
└── logo/
    ├── svg/
    │   ├── falcon-icon-primary.svg       # White falcon on brand blue
    │   ├── falcon-icon-dark.svg          # Light blue falcon on ink
    │   ├── falcon-mark-monochrome.svg    # Uses currentColor for tinting
    │   └── falcon-wordmark-horizontal.svg
    └── png/
        ├── falcon-icon-1024.png          # App Store Connect asset
        ├── falcon-icon-dark-1024.png
        └── falcon-icon-{180,120,80,60,40}.png
```

## How to use with Claude Code

### Step 1 — Drop into your iOS repo

Copy `DESIGN_SYSTEM.md` and `CLAUDE.md` into the **root** of your Falcon iOS repo. Copy the `logo/` folder anywhere, but `logo/` at the root is standard.

### Step 2 — Let Claude Code see `CLAUDE.md`

Claude Code reads `CLAUDE.md` automatically when started in a repo. No extra config needed. It will follow those rules for every request in that project.

### Step 3 — Kick off implementation

Open Claude Code in the repo and run this prompt:

> Implement the Falcon design system in SwiftUI following `DESIGN_SYSTEM.md`. iOS 17+, no external dependencies.
>
> Start with the tokens layer in this exact order:
> 1. `Sources/DesignSystem/Assets.xcassets/Colors/` — create one `.colorset` per color token from section 2 of the spec, with light and dark variants.
> 2. `Sources/DesignSystem/Tokens/Color+Tokens.swift` — extension exposing all tokens as `Color.brandPrimary` etc.
> 3. `Sources/DesignSystem/Tokens/Font+Tokens.swift` — typography scale from section 3.
> 4. `Sources/DesignSystem/Tokens/Spacing.swift` and `CornerRadius.swift` — from sections 4 and 5.
>
> After each token file, show me the diff and wait for confirmation before continuing. Then move to components one by one (section 6), each with a `#Preview` showing all states in light + dark.
>
> Finally, set up `Assets.xcassets/AppIcon.appiconset/` using `logo/png/falcon-icon-1024.png`.

### Step 4 — Iterate

As features get built, Claude Code will automatically reach for tokens and components. If you catch a hardcoded color or magic number sneaking in, point it at `CLAUDE.md` section "Strict rules."

## Updating the design system

The spec is a living document. When the design evolves:
1. Edit `DESIGN_SYSTEM.md` first (add/change the token).
2. Then ask Claude Code to propagate the change through `Assets.xcassets` and any affected components.

This keeps the spec and the code in sync.

## Logo — App Store Connect

Upload `logo/png/falcon-icon-1024.png` (1024×1024, sRGB, no alpha, no transparency). iOS applies its own superellipse mask, so the squircle in the SVG is approximate — the final appearance on the home screen uses Apple's exact mask.
