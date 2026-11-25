"""Build colr-nospace.ttf, a minimal COLRv0 font for the tests: U+E900 is a
color glyph of two layers, and there is no space glyph, as in many color
emoji fonts. Run with fontTools installed: python3 make-colr-nospace.py
"""
from fontTools.fontBuilder import FontBuilder
from fontTools.pens.ttGlyphPen import TTGlyphPen
from fontTools.ttLib.tables import otTables
from fontTools.colorLib.builder import buildCOLR, buildCPAL


def square(x0, y0, x1, y1):
    pen = TTGlyphPen(None)
    pen.moveTo((x0, y0))
    pen.lineTo((x0, y1))
    pen.lineTo((x1, y1))
    pen.lineTo((x1, y0))
    pen.closePath()
    return pen.glyph()


order = [".notdef", "uniE900", "layer0", "layer1"]
fb = FontBuilder(1000, isTTF=True)
fb.setupGlyphOrder(order)
fb.setupCharacterMap({0xE900: "uniE900"})
fb.setupGlyf({
    ".notdef": square(50, 0, 450, 700),
    "uniE900": square(0, 0, 1000, 1000),
    "layer0": square(0, 0, 1000, 1000),
    "layer1": square(250, 250, 750, 750),
})
fb.setupHorizontalMetrics({g: (1000 if g != ".notdef" else 500, 0) for g in order})
fb.setupHorizontalHeader(ascent=800, descent=-200)
fb.setupNameTable({"familyName": "ColrNoSpace", "styleName": "Regular"})
fb.setupOS2(sTypoAscender=800, sTypoDescender=-200, usWinAscent=800, usWinDescent=200)
fb.setupPost()
fb.font["COLR"] = buildCOLR({"uniE900": [("layer0", 0), ("layer1", 1)]}, version=0)
fb.font["CPAL"] = buildCPAL([[(1, 0, 0, 1), (0, 0, 1, 1)]])
fb.save("colr-nospace.ttf")
