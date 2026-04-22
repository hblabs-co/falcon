#!/usr/bin/env python3
"""
Generate the Open Graph preview image (1200×630) that social platforms
render when someone shares a link to falcon.hblabs.co.

Run from the falcon-landing/ directory:

    python3 -m venv /tmp/ogvenv  && \
    /tmp/ogvenv/bin/pip install --quiet Pillow  && \
    /tmp/ogvenv/bin/python scripts/gen_og.py

Layout:
  ┌────────────────────────────────────────────┐
  │                                            │
  │      ┌────────┐    FALCON                  │
  │      │ Falcon │    Project Intelligence    │
  │      │ logo   │                            │
  │      │ (R=24) │                            │
  │      └────────┘    by Helmer Barcos        │
  │                    · HB Labs SAS [HB]       │
  │                                            │
  └────────────────────────────────────────────┘

Font choice: Helvetica (macOS system font, always available).
"""

from pathlib import Path
from base64 import b64decode
from PIL import Image, ImageDraw, ImageFont

# ─────────────────────────────────────────────────────────────
# Paths — resolved relative to this script so `python scripts/gen_og.py`
# works regardless of the current working directory.
# ─────────────────────────────────────────────────────────────
here       = Path(__file__).resolve().parent
root       = here.parent                    # falcon-landing/
static_dir = root / "landing" / "static"
logo_txt   = static_dir / "falcon_logo.txt" # base64 PNG
hblabs_png = static_dir / "hblabs.png"
out_png    = static_dir / "og-image.png"

# ─────────────────────────────────────────────────────────────
# Design constants
# ─────────────────────────────────────────────────────────────
W, H       = 1200, 630
BG         = (14, 14, 16)            # matches the dark-mode --bg in CSS
FG         = (243, 243, 245)         # primary text
MUTED      = (156, 163, 175)         # secondary text
ACCENT     = (96, 165, 250)          # dark-mode accent blue

# Font family: match the CSS stack in index.html.tmpl
# (-apple-system, "SF Pro Display", "Inter", ...). On macOS, installing
# "SF Pro" from developer.apple.com/fonts/ drops the OTFs into
# /Library/Fonts/. Falls back to Helvetica if SF Pro isn't present.
import os as _os
_SFPRO_BOLD    = "/Library/Fonts/SF-Pro-Display-Bold.otf"
_SFPRO_MEDIUM  = "/Library/Fonts/SF-Pro-Display-Medium.otf"
_SFPRO_REGULAR = "/Library/Fonts/SF-Pro-Display-Regular.otf"
FONT_BOLD     = _SFPRO_BOLD    if _os.path.exists(_SFPRO_BOLD)    else "/System/Library/Fonts/Helvetica.ttc"
FONT_MEDIUM   = _SFPRO_MEDIUM  if _os.path.exists(_SFPRO_MEDIUM)  else FONT_BOLD
FONT_REGULAR  = _SFPRO_REGULAR if _os.path.exists(_SFPRO_REGULAR) else "/System/Library/Fonts/Helvetica.ttc"

# Falcon logo: decode base64, resize, round the corners.
LOGO_SIZE    = 260
LOGO_RADIUS  = 48                    # ≈18% of size — matches the iOS app icon feel

# HB Labs logo: small chip next to the "HB Labs SAS" text.
HBLABS_SIZE  = 36


def rounded_icon(src_path: Path, size: int, radius: int) -> Image.Image:
    """Load `src_path`, scale to size×size, apply a rounded-rect mask."""
    img = Image.open(src_path).convert("RGBA")
    img = img.resize((size, size), Image.LANCZOS)

    # Build the alpha mask: filled rounded rect at full opacity.
    mask = Image.new("L", (size, size), 0)
    d = ImageDraw.Draw(mask)
    d.rounded_rectangle((0, 0, size, size), radius=radius, fill=255)

    out = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    out.paste(img, (0, 0), mask)
    return out


def main() -> None:
    # ─ Canvas ────────────────────────────────────────────────
    canvas = Image.new("RGB", (W, H), BG)
    draw = ImageDraw.Draw(canvas)

    # ─ Subtle accent ring bottom-left to hint at "this is a product" ─
    #   (purely decorative; keep it understated)
    for r, alpha in [(520, 14), (420, 20), (320, 28)]:
        ring = Image.new("RGBA", (W, H), (0, 0, 0, 0))
        ImageDraw.Draw(ring).ellipse(
            (-r, H - r, r, H + r),
            fill=(ACCENT[0], ACCENT[1], ACCENT[2], alpha),
        )
        canvas.paste(ring, (0, 0), ring)

    # ─ Falcon logo ───────────────────────────────────────────
    # Decode the embedded base64 PNG, then drop the icon in the
    # left third of the canvas with matching vertical centring.
    raw_png = b64decode("".join(logo_txt.read_text().splitlines()))
    tmp_logo = here / "_falcon_tmp.png"
    tmp_logo.write_bytes(raw_png)

    logo = rounded_icon(tmp_logo, LOGO_SIZE, LOGO_RADIUS)
    logo_x = 120
    logo_y = (H - LOGO_SIZE) // 2
    canvas.paste(logo, (logo_x, logo_y), logo)
    tmp_logo.unlink(missing_ok=True)

    # ─ Text block ────────────────────────────────────────────
    text_x = logo_x + LOGO_SIZE + 48
    title_font    = ImageFont.truetype(FONT_BOLD,    112)
    tagline_font  = ImageFont.truetype(FONT_MEDIUM,  36)
    credit_font   = ImageFont.truetype(FONT_REGULAR, 28)

    # "Falcon"
    draw.text((text_x, 170), "Falcon", font=title_font, fill=FG)

    # Tagline — dimmer, one step below. "Intelligent project search"
    # mirrors the DE "Intelligente Projektsuche" used in the iOS app.
    draw.text((text_x, 300), "Intelligent project search",
              font=tagline_font, fill=MUTED)

    # Single credit line separated by a mid-dot: "by Helmer Barcos ·
    # HB Labs SAS" + tiny HB Labs chip after "SAS" so the logo reads
    # as a visual cue, not a second sentence.
    credit_y = 400
    credit_text = "by Helmer Barcos · HB Labs SAS"
    draw.text((text_x, credit_y), credit_text, font=credit_font, fill=MUTED)

    bbox = draw.textbbox((text_x, credit_y), credit_text, font=credit_font)
    chip_x = bbox[2] + 12
    chip_y = credit_y - 3

    hb_chip = rounded_icon(hblabs_png, HBLABS_SIZE, 8)
    canvas.paste(hb_chip, (chip_x, chip_y), hb_chip)

    # ─ Footer — tiny domain label bottom-right ───────────────
    domain_font = ImageFont.truetype(FONT_REGULAR, 20)
    draw.text((W - 190, H - 44), "falcon.hblabs.co",
              font=domain_font, fill=MUTED)

    # ─ Save ──────────────────────────────────────────────────
    canvas.save(out_png, format="PNG", optimize=True)
    print(f"wrote {out_png} ({out_png.stat().st_size} bytes, {W}×{H})")


if __name__ == "__main__":
    main()
