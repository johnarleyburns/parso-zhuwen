#!/usr/bin/env python3
"""Generate the Zhuwen app icon: periodic-table tile for Zn (Zinc).
Cinnabar (#C3272B) on a dark radial-gradient field, matching the design system.
"""
from PIL import Image, ImageDraw, ImageFont
import math
import glob
import sys

SIZE = 1024
BG_TOP = (26, 21, 18)      # warm ink with cinnabar hint
BG_BOT = (8, 7, 6)         # near-black
CINNABAR = (195, 39, 43)   # #C3272B
CINNABAR_DIM = (154, 31, 35)
INK = (242, 244, 246)      # ink (white-ish)
INK3 = (128, 133, 140)     # ink3 (dim)

def find_font(name):
    base = "/System/Library/Fonts/"
    for p in glob.glob(base + "*" + name + "*"):
        if not p.endswith(".ttc"):
            return p
    # Fall back to Helvetica .ttc with faces
    for p in glob.glob(base + "HelveticaNeue.ttc"):
        return p
    return "/System/Library/Fonts/HelveticaNeue.ttc"

def ttf(size, bold=False, idx=0):
    p = find_font("HelveticaNeue")
    try:
        return ImageFont.truetype(p, size, index=idx)
    except:
        try:
            return ImageFont.truetype(p, size)
        except:
            return ImageFont.load_default()

img = Image.new("RGB", (SIZE, SIZE), BG_BOT)
draw = ImageDraw.Draw(img)

# Vertical gradient.
for y in range(SIZE):
    t = y / SIZE
    r = int(BG_TOP[0] * (1 - t) + BG_BOT[0] * t)
    g = int(BG_TOP[1] * (1 - t) + BG_BOT[1] * t)
    b = int(BG_TOP[2] * (1 - t) + BG_BOT[2] * t)
    draw.line([(0, y), (SIZE, y)], fill=(r, g, b))

# Radial cinnabar glow.
glow = Image.new("L", (SIZE, SIZE), 0)
gd = ImageDraw.Draw(glow)
cx, cy = int(SIZE * 0.28), int(SIZE * 0.08)
maxr = int(SIZE * 0.80)
for rr in range(maxr, 0, -3):
    a = int(55 * (1 - rr / maxr) ** 2.2)
    gd.ellipse([cx - rr, cy - rr, cx + rr, cy + rr], fill=a)
glow_col = Image.new("RGB", (SIZE, SIZE), CINNABAR)
img = Image.composite(glow_col, img, glow)
draw = ImageDraw.Draw(img)

margin = int(SIZE * 0.115)

# Atomic number "30" — top-left.
f_num = ttf(size=130, idx=1)
draw.text((margin + 65, margin + 48), "30", font=f_num, fill=INK)

# "Zn" symbol — large, centered.
sym = "Zn"
f_sym = ttf(size=420, idx=1)
bbox = draw.textbbox((0, 0), sym, font=f_sym)
sw = bbox[2] - bbox[0]
sh = bbox[3] - bbox[1]
sx = (SIZE - sw) // 2 - bbox[0]
sy = (SIZE - sh) // 2 - bbox[1] - 35
draw.text((sx, sy), sym, font=f_sym, fill=INK)

# "Zhuwen" — below symbol.
f_name = ttf(size=78, idx=0)
name = "Zhuwen"
nb = draw.textbbox((0, 0), name, font=f_name)
nw = nb[2] - nb[0]
draw.text(((SIZE - nw) // 2 - nb[0], SIZE - margin - 200), name, font=f_name, fill=INK)

# "65.38" — atomic weight at bottom.
f_wt = ttf(size=60, idx=0)
wt = "65.38"
wb_d = draw.textbbox((0, 0), wt, font=f_wt)
ww = wb_d[2] - wb_d[0]
draw.text(((SIZE - ww) // 2 - wb_d[0], SIZE - margin - 130), wt, font=f_wt, fill=INK)

out = sys.argv[1] if len(sys.argv) > 1 else "AppIcon-1024.png"
img.save(out, "PNG")
print("wrote", out)
