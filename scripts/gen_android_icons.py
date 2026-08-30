#!/usr/bin/env python3
"""Generate the Android launcher icons from the brand icon (build/appicon.png).

The upstream Wails v3 template ships the default Wails "W" logo as launcher
icons; this script replaces them with the project brand so the APK launcher
icon matches the desktop icon. Outputs under
build/android/app/src/main/res/:

  - mipmap-{mdpi..xxxhdpi}/ic_launcher.png           legacy full-bleed square
  - mipmap-{mdpi..xxxhdpi}/ic_launcher_round.png     legacy circle-masked
  - mipmap-{mdpi..xxxhdpi}/ic_launcher_foreground.png adaptive-icon foreground
  - mipmap-anydpi-v26/ic_launcher{,_round}.xml       adaptive-icon wiring

The adaptive background is a flat brand color: add to values/colors.xml
    <color name="ic_launcher_background">#9287F6</color>
(average of the opaque brand art; printed at the end as a reminder).

Legacy API (<26) launchers use the PNG mipmaps directly; API 26+ launchers use
the adaptive icon (background color + centered white glyph). Requires Pillow.

Usage: python scripts/gen_android_icons.py
"""

from pathlib import Path

from PIL import Image, ImageDraw

REPO = Path(__file__).resolve().parent.parent
SRC = REPO / "build" / "appicon.png"
RES = REPO / "build" / "android" / "app" / "src" / "main" / "res"

# dp sizes per density bucket (standard Android launcher icon scales)
LEGACY_SIZES = {"mdpi": 48, "hdpi": 72, "xhdpi": 96, "xxhdpi": 144, "xxxhdpi": 192}
FOREGROUND_SIZES = {"mdpi": 108, "hdpi": 162, "xhdpi": 216, "xxhdpi": 324, "xxxhdpi": 432}

# The brand art is a rounded square with transparent corners; the full-bleed
# legacy/round tiles must have opaque corners, so the art is scaled up until
# the cropped canvas corners are covered by the rounded rect.
ART = 512


def white_glyph_mask(im: Image.Image) -> Image.Image:
    """Alpha mask of the white glyph: the art background is a purple gradient
    whose red/green channels stay in ~[130,160], the glyph is pure white —
    min(r,g) separates them with smooth anti-aliased edges. The rounded-square
    edge also carries a light fringe that passes the channel test, so the ramp
    is confined to the neighbourhood of the fully-opaque glyph core."""
    px = im.load()
    strict = (512, 512, 0, 0)
    for y in range(im.height):
        for x in range(im.width):
            r, g, _b, a = px[x, y][:4]
            if a >= 250 and min(r, g) >= 250:
                strict = (min(strict[0], x), min(strict[1], y), max(strict[2], x), max(strict[3], y))
    pad = 8
    region = (
        max(0, strict[0] - pad),
        max(0, strict[1] - pad),
        min(im.width, strict[2] + 1 + pad),
        min(im.height, strict[3] + 1 + pad),
    )

    mask = Image.new("L", im.size, 0)
    mp = mask.load()
    for y in range(region[1], region[3]):
        for x in range(region[0], region[2]):
            r, g, _b, a = px[x, y][:4]
            v = min(r, g)
            if v >= 245:
                m = 255
            elif v > 170:
                m = int((v - 170) * 255 / 75)
            else:
                m = 0
            mp[x, y] = m * a // 255
    return mask


def full_bleed_square(im: Image.Image) -> Image.Image:
    """Center-crop a scaled copy so every canvas pixel is opaque art."""
    for step in range(0, 46):
        s = 1.05 + step * 0.01
        side = round(ART * s)
        big = im.resize((side, side), Image.LANCZOS)
        off = (side - ART) // 2
        crop = big.crop((off, off, off + ART, off + ART))
        px = crop.load()
        if all(px[p][3] == 255 for p in ((0, 0), (ART - 1, 0), (0, ART - 1), (ART - 1, ART - 1))):
            return crop
    raise RuntimeError("could not find a corner-opaque scale for the brand art")


def circle_mask(size: int) -> Image.Image:
    """Anti-aliased circular alpha mask (supersampled 4x)."""
    big = Image.new("L", (size * 4, size * 4), 0)
    ImageDraw.Draw(big).ellipse((0, 0, size * 4 - 1, size * 4 - 1), fill=255)
    return big.resize((size, size), Image.LANCZOS)


def main() -> None:
    art = Image.open(SRC).convert("RGBA")
    square = full_bleed_square(art)
    glyph = white_glyph_mask(art)
    glyph = glyph.crop(glyph.getbbox())  # drop the empty margins around the glyph
    gw, gh = glyph.size

    for density, size in LEGACY_SIZES.items():
        out = RES / f"mipmap-{density}"
        out.mkdir(parents=True, exist_ok=True)
        square.resize((size, size), Image.LANCZOS).save(out / "ic_launcher.png")
        round_icon = square.resize((size, size), Image.LANCZOS)
        round_icon.putalpha(circle_mask(size))
        round_icon.save(out / "ic_launcher_round.png")

    for density, size in FOREGROUND_SIZES.items():
        # Adaptive canvas is 108dp; the safe zone is its central 66dp circle.
        # Cap the glyph bounding box at 0.43*size so its corners stay inside
        # the safe zone on every mask shape.
        scale = 0.43 * size / max(gw, gh)
        glyph_resized = glyph.resize((round(gw * scale), round(gh * scale)), Image.LANCZOS)
        canvas = Image.new("RGBA", (size, size), (0, 0, 0, 0))
        canvas.paste(
            (255, 255, 255, 255),
            ((size - glyph_resized.width) // 2, (size - glyph_resized.height) // 2),
            glyph_resized,
        )
        canvas.save(RES / f"mipmap-{density}" / "ic_launcher_foreground.png")

    anydpi = RES / "mipmap-anydpi-v26"
    anydpi.mkdir(parents=True, exist_ok=True)
    adaptive = (
        '<?xml version="1.0" encoding="utf-8"?>\n'
        '<adaptive-icon xmlns:android="http://schemas.android.com/apk/res/android">\n'
        '    <background android:drawable="@color/ic_launcher_background"/>\n'
        '    <foreground android:drawable="@mipmap/ic_launcher_foreground"/>\n'
        "</adaptive-icon>\n"
    )
    (anydpi / "ic_launcher.xml").write_text(adaptive, encoding="utf-8", newline="\n")
    (anydpi / "ic_launcher_round.xml").write_text(adaptive, encoding="utf-8", newline="\n")

    tot = [0, 0, 0]
    n = 0
    px = art.load()
    for y in range(0, ART, 4):
        for x in range(0, ART, 4):
            r, g, b, a = px[x, y]
            if a > 200:
                tot[0] += r
                tot[1] += g
                tot[2] += b
                n += 1
    print("generated launcher icons under", RES)
    print('adaptive background color: #%02X%02X%02X' % tuple(t // n for t in tot))


if __name__ == "__main__":
    main()
