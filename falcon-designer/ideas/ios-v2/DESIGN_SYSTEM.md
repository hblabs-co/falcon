# Falcon — Design System Specification

> **Single source of truth** for Falcon's iOS app visual system.
> When implementing UI, always consult this document. Never hardcode values that are defined here as tokens.

**Product**: Falcon — AI-powered job matching for candidates
**Platform**: iOS 17+ (SwiftUI)
**Vibe**: Tech modern, minimalist. References: Linear, Raycast, Arc, Revolut.
**Primary audience**: Candidates. Secondary: recruiters/companies.

---

## 1. Brand identity

### 1.1 Logo variants

| File | Use case |
|---|---|
| `logo/svg/falcon-icon-primary.svg` | Main app icon (white falcon on `#1656D6` squircle) |
| `logo/svg/falcon-icon-dark.svg` | Dark mode variant (light blue falcon on `#0B1220` squircle) |
| `logo/svg/falcon-mark-monochrome.svg` | Mark-only, uses `currentColor` — for tinting in SwiftUI |
| `logo/svg/falcon-wordmark-horizontal.svg` | Icon + "Falcon" text, for headers |
| `logo/png/falcon-icon-1024.png` | App Store Connect submission asset |

### 1.2 Logo usage rules

- **Minimum size**: 24px for the icon alone. Below this, legibility breaks.
- **Clear space**: Padding around the icon ≥ 25% of icon size.
- **Never**: stretch, rotate, add shadows, recolor outside the defined variants, or place on busy photographic backgrounds.
- **App icon corner radius**: iOS applies its own mask. The squircle in the SVG uses `rx="228"` for a 1024×1024 canvas (22.27% of edge), matching iOS's superellipse approximation.

---

## 2. Color tokens

All colors must be defined as **Color Sets in `Assets.xcassets`** with explicit light/dark variants — never hardcoded in Swift source files. Swift code references them through a `Color` extension that exposes semantic names.

### 2.1 Brand ramp

| Token | Light | Dark | Usage |
|---|---|---|---|
| `brand50` | `#EBF2FE` | `#0A2968` | Subtle blue tints, hover surfaces |
| `brand100` | `#C7DCFB` | `#0E3FA0` | Selected states, chips background |
| `brand300` | `#5B9DF9` | `#5B9DF9` | Accent on dark surfaces, secondary brand |
| `brand500` | `#1656D6` | `#5B9DF9` | **Primary brand color** — CTAs, active tabs, logo fill |
| `brand700` | `#0E3FA0` | `#C7DCFB` | Pressed state of primary buttons |
| `brand900` | `#0A2968` | `#EBF2FE` | Text on brand tints, emphasis |

### 2.2 Neutrals

| Token | Light | Dark | Usage |
|---|---|---|---|
| `bgPrimary` | `#FFFFFF` | `#0B1220` | Main screen background |
| `bgSecondary` | `#F7F8FA` | `#131B2E` | Cards, elevated surfaces, grouped sections |
| `bgTertiary` | `#EEF1F5` | `#1C2640` | Input fields, disabled surfaces |
| `borderPrimary` | `#E5E7EB` | `#243047` | Default borders, dividers |
| `borderSecondary` | `#D1D5DB` | `#334363` | Focused borders, emphasized dividers |
| `textPrimary` | `#0B1220` | `#F5F7FA` | Body text, headings |
| `textSecondary` | `#6B7280` | `#9CA3AF` | Labels, metadata, captions |
| `textTertiary` | `#9CA3AF` | `#6B7280` | Placeholders, disabled text |
| `textOnBrand` | `#FFFFFF` | `#FFFFFF` | Text on `brand500` surfaces (always white) |

### 2.3 Semantic status

| Token | Light fill | Light text | Dark fill | Dark text | Usage |
|---|---|---|---|---|---|
| `successBg` / `successText` | `#ECFDF5` | `#065F46` | `#052E20` | `#6EE7B7` | Match confirmed, application accepted |
| `warningBg` / `warningText` | `#FFFBEB` | `#92400E` | `#3D2A06` | `#FCD34D` | Pending review, expiring soon |
| `dangerBg` / `dangerText` | `#FEF2F2` | `#991B1B` | `#3B0A0A` | `#FCA5A5` | Rejected, error states |
| `infoBg` / `infoText` | `#EBF2FE` | `#0A2968` | `#0A2968` | `#C7DCFB` | Informational callouts (same as brand50) |

### 2.4 Match score colors (domain-specific)

Used for the match percentage indicator on job cards.

| Range | Color token | Hex (light) | Hex (dark) |
|---|---|---|---|
| 90–100% | `matchExcellent` | `#059669` | `#10B981` |
| 75–89% | `matchGood` | `#1656D6` | `#5B9DF9` |
| 60–74% | `matchFair` | `#D97706` | `#F59E0B` |
| < 60% | `matchPoor` | `#6B7280` | `#9CA3AF` |

---

## 3. Typography

**Font family**: SF Pro (iOS system font). Use `.system(...)` with explicit design and weight. Do not import custom fonts.

### 3.1 Type scale

| Token | Size | Weight | Tracking | Line height | Usage |
|---|---|---|---|---|---|
| `displayLarge` | 34 | `.semibold` | -0.022em | 1.1 | Landing hero, onboarding headers |
| `displayMedium` | 28 | `.semibold` | -0.020em | 1.15 | Screen greetings ("Good morning, Helmer") |
| `titleLarge` | 22 | `.semibold` | -0.015em | 1.2 | Section headers, match card titles |
| `titleMedium` | 20 | `.semibold` | -0.010em | 1.25 | Job titles on cards |
| `titleSmall` | 17 | `.semibold` | -0.005em | 1.3 | Navigation titles, list headers |
| `bodyLarge` | 17 | `.regular` | 0 | 1.4 | Default body, job descriptions |
| `bodyMedium` | 15 | `.regular` | 0 | 1.4 | Secondary content, captions in cards |
| `bodySmall` | 13 | `.regular` | 0 | 1.4 | Metadata (location, date, salary) |
| `label` | 12 | `.medium` | 0.06em UPPER | 1.3 | Section labels ("TOP MATCH"), tab bar |
| `caption` | 11 | `.regular` | 0 | 1.3 | Legal, tiny helper text |

### 3.2 Weights to use

Only these weights — do not introduce others:
- `.regular` (400) — body
- `.medium` (500) — labels, emphasis
- `.semibold` (600) — headings, titles, buttons

---

## 4. Spacing

Base unit: **4pt**. All spacing values are multiples of 4.

| Token | Value | Usage |
|---|---|---|
| `space1` | 4 | Tight internal gaps (icon-to-text) |
| `space2` | 8 | Chip padding, small gaps |
| `space3` | 12 | Standard internal padding, list item gaps |
| `space4` | 16 | Card padding, section breathing room |
| `space5` | 20 | Screen horizontal margin |
| `space6` | 24 | Between major sections |
| `space8` | 32 | Large section separation |
| `space10` | 40 | Top spacing under nav |
| `space12` | 48 | Hero spacing |

**Screen horizontal padding**: always `space5` (20pt). No exceptions.

---

## 5. Corner radii

| Token | Value | Usage |
|---|---|---|
| `radiusSmall` | 8 | Chips, tags, small badges |
| `radiusMedium` | 12 | Inputs, buttons |
| `radiusLarge` | 16 | Secondary cards |
| `radiusXLarge` | 20 | Primary cards (e.g. featured match card) |
| `radiusFull` | 999 | Pills, avatar circles |

**Never** apply rounded corners to only one or two sides. Use full rounding or none.

---

## 6. Component anatomy

### 6.1 `PrimaryButton`

- Height: 52pt (full-width), 44pt (inline)
- Corner radius: `radiusMedium` (12)
- Background: `brand500`
- Text: `textOnBrand`, `titleSmall` weight `.semibold`
- Horizontal padding: `space5` (20)
- States:
  - Default: as above
  - Pressed: background `brand700`, scale 0.98
  - Disabled: background `bgTertiary`, text `textTertiary`
- Haptic on press: `.light`

### 6.2 `SecondaryButton`

- Same dimensions as PrimaryButton
- Background: transparent
- Border: 1pt `borderPrimary`
- Text: `textPrimary`
- Pressed: background `bgSecondary`

### 6.3 `MatchCard` (featured / top match)

- Background: `textPrimary` (inverted — uses ink color as fill so it stands out from white screen)
- Text: white throughout
- Corner radius: `radiusXLarge` (20)
- Padding: `space4` (16) all sides, increased to `space5` (20) on top and bottom for breathing room
- Internal layout:
  - Top row: label "TOP MATCH · 96%" (style: `label`, color `brand300`) + status dot (6pt circle, `successBg` fill on dark = `#10B981`)
  - Title: job title, `titleMedium`, white
  - Subtitle: company · location, `bodySmall`, `#9CA3AF`
  - Chips row: skill tags with `rgba(91,157,249,0.15)` background, `brand300` text, `radiusFull`

### 6.4 `JobCard` (secondary listings)

- Background: `bgSecondary`
- Corner radius: `radiusLarge` (16)
- Padding: `space3` ×`space4` (12×16)
- Layout: horizontal — job info left, match % right
- Match % uses color from section 2.4

### 6.5 `Chip` / `Tag`

- Height: 24pt
- Padding: 4 × 10pt
- Corner radius: `radiusFull`
- Text: `bodySmall` (13), `.medium`
- Default: `brand50` bg, `brand900` text
- On dark: `rgba(91,157,249,0.15)` bg, `brand300` text

### 6.6 `Avatar`

- Sizes: 28 (small), 32 (medium), 44 (large)
- Corner radius: `radiusFull`
- Default fill: `brand50`, text `brand500`
- Contains initials: `titleSmall` weight `.medium`, sized proportionally (40% of avatar size)

### 6.7 `TabBar`

- Height: 56pt + safe area bottom inset
- Background: `bgPrimary` with 0.5pt top border in `borderPrimary`
- Tab item: icon 22pt + label `caption`
- Active: `brand500`
- Inactive: `textTertiary`

### 6.8 `Header` (screen top)

- Height: 52pt (not counting status bar)
- Horizontal padding: `space5`
- Layout: logo mark (26pt) + "Falcon" wordmark on left, avatar on right
- Logo wordmark text: `titleSmall`, weight `.semibold`

---

## 7. Motion

- Default transition: `.spring(response: 0.35, dampingFraction: 0.8)`
- Tap feedback: scale `0.97` with spring response `0.2`
- Screen transitions: system default (`.navigationTransition`)
- **Never**: bouncy decorative animations, parallax, long-duration (>0.4s) transitions

---

## 8. File structure (iOS project)

```
Falcon/
├── Sources/
│   ├── DesignSystem/
│   │   ├── Tokens/
│   │   │   ├── Color+Tokens.swift
│   │   │   ├── Font+Tokens.swift
│   │   │   ├── Spacing.swift
│   │   │   └── CornerRadius.swift
│   │   ├── Components/
│   │   │   ├── PrimaryButton.swift
│   │   │   ├── SecondaryButton.swift
│   │   │   ├── MatchCard.swift
│   │   │   ├── JobCard.swift
│   │   │   ├── Chip.swift
│   │   │   ├── Avatar.swift
│   │   │   ├── FalconHeader.swift
│   │   │   └── FalconTabBar.swift
│   │   └── Assets.xcassets/
│   │       ├── Colors/   (one .colorset per token, light/dark variants)
│   │       └── Logo/     (imported from logo/ folder)
│   └── Features/
│       └── (feature modules use DesignSystem — never define tokens themselves)
```

---

## 9. Implementation rules (for Claude Code)

1. **Tokens over values.** Never write `.padding(16)` — write `.padding(Spacing.space4)`. Never write `Color(hex: "#1656D6")` — write `Color.brandPrimary`.
2. **Color Sets, not hex.** Every color lives in `Assets.xcassets` as a `.colorset` with light and dark variants. Swift extensions only reference `Color("TokenName", bundle: .main)`.
3. **Preview every component.** Each component file must include a `#Preview` that shows: default state, pressed state (where applicable), disabled state (where applicable), and light + dark side by side.
4. **No magic numbers.** If a value appears twice, it becomes a token. If it's visual, it goes in this spec first.
5. **Accessibility.** All interactive components must have `.accessibilityLabel` and meet 44×44pt minimum tap target.
6. **SF Symbols for icons.** Use Apple's SF Symbols library. Do not import icon SVGs unless absolutely necessary.
