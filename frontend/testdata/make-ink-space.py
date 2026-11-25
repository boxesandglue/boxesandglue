"""Build ink-space.ttf, a minimal COLRv0 font for the tests whose spaces
paint, as in color fonts that run a stripe through the word spaces:
U+0020, U+00A0 and U+2009 are color glyphs of two layers, b and c are plain
squares. Run with fontTools installed: python3 make-ink-space.py
"""
from fontTools.fontBuilder import FontBuilder
from fontTools.pens.ttGlyphPen import TTGlyphPen
from fontTools.colorLib.builder import buildCOLR, buildCPAL


def square(x0, y0, x1, y1):
    pen = TTGlyphPen(None)
    pen.moveTo((x0, y0))
    pen.lineTo((x0, y1))
    pen.lineTo((x1, y1))
    pen.lineTo((x1, y0))
    pen.closePath()
    return pen.glyph()


def empty():
    return TTGlyphPen(None).glyph()


order = [".notdef", "space", "uni2009", "b", "c", "stripe0", "stripe1", "thin0", "thin1"]
fb = FontBuilder(1000, isTTF=True)
fb.setupGlyphOrder(order)
fb.setupCharacterMap({0x20: "space", 0xA0: "space", 0x2009: "uni2009", 0x62: "b", 0x63: "c"})
fb.setupGlyf({
    ".notdef": square(50, 0, 450, 700),
    "space": empty(),
    "uni2009": empty(),
    "b": square(50, 0, 450, 700),
    "c": square(50, 0, 450, 500),
    "stripe0": square(0, 300, 250, 400),
    "stripe1": square(0, 400, 250, 500),
    "thin0": square(0, 300, 100, 400),
    "thin1": square(0, 400, 100, 500),
})
widths = {".notdef": 500, "space": 250, "uni2009": 100, "b": 500, "c": 500,
          "stripe0": 250, "stripe1": 250, "thin0": 100, "thin1": 100}
fb.setupHorizontalMetrics({g: (widths[g], 0) for g in order})
fb.setupHorizontalHeader(ascent=800, descent=-200)
fb.setupNameTable({"familyName": "InkSpace", "styleName": "Regular"})
fb.setupOS2(sTypoAscender=800, sTypoDescender=-200, usWinAscent=800, usWinDescent=200)
fb.setupPost()
fb.font["COLR"] = buildCOLR({
    "space": [("stripe0", 0), ("stripe1", 1)],
    "uni2009": [("thin0", 0), ("thin1", 1)],
}, version=0)
fb.font["CPAL"] = buildCPAL([[(1, 0, 0, 1), (0, 0, 1, 1)]])
fb.save("ink-space.ttf")
