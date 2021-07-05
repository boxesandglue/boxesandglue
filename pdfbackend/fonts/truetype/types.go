package truetype

import "io"

// Flags for compound glyphs.
//
// See https://www.microsoft.com/typography/OTSPEC/glyf.htm
const (
	flagArg1And2AreWords        = 1 << 0  // 0x0001
	flagArgsAreXYValues         = 1 << 1  // 0x0002
	flagRoundXYToGrid           = 1 << 2  // 0x0004
	flagWeHaveAScale            = 1 << 3  // 0x0008
	flagReserved4               = 1 << 4  // 0x0010
	flagMoreComponents          = 1 << 5  // 0x0020
	flagWeHaveAnXAndYScale      = 1 << 6  // 0x0040
	flagWeHaveATwoByTwo         = 1 << 7  // 0x0080
	flagWeHaveInstructions      = 1 << 8  // 0x0100
	flagUseMyMetrics            = 1 << 9  // 0x0200
	flagOverlapCompound         = 1 << 10 // 0x0400
	flagScaledComponentOffset   = 1 << 11 // 0x0800
	flagUnscaledComponentOffset = 1 << 12 // 0x1000
)

type tableOffsetLength struct {
	offset    uint32
	length    uint32
	name      string
	checksum  uint32
	tabledata []byte
}

// Glyph is a TrueType glyph. Since we are not interested in the glyph details,
// only the whole data is stored here
type Glyph []byte

// Font represents the font file for a TrueType font
type Font struct {
	r                   io.ReadSeeker
	sfntVersion         uint32
	tables              map[string]tableOffsetLength
	tablesRead          map[string]bool // list of tables that have been read
	GlyphNames          []string
	names               map[int]string
	glyphOffsets        []uint32
	advanceWidth        []uint16
	lsb                 []int16
	fpgm                []byte
	cvt                 []byte
	prep                []byte
	UnitsPerEM          uint16
	ToUni               map[int]rune // glyph id to unicode value
	ToCodepoint         map[rune]int
	subsetCodepoints    []int
	Hhea                Hhea
	Head                Head
	Maxp                Maxp
	Post                Post
	OS2                 OS2
	OS2AdditionalFields OS2AdditionalFields
	Glyph               []Glyph
	SubsetID            string
}

// Hhea Horizontal Header Table.
type Hhea struct {
	MajorVersion        uint16 // 1
	MinorVersion        uint16 // 0
	Ascender            int16  // see spec
	Descender           int16  // see spec
	LineGap             int16  // Negative LineGap values are treated as zero in some legacy platform implementations.
	AdvanceWidthMax     uint16
	MinLeftSideBearing  int16
	MinRightSideBearing int16
	XMaxExtent          int16
	CaretSlopeRise      int16
	CaretSlopeRun       int16
	CaretOffset         int16
	MetricDataFormat    int16
	NumberOfHMetrics    uint16
}

// Head Font header
type Head struct {
	MajorVersion       uint16 // 1
	MinorVersion       uint16 // 0
	FontRevision       uint32 // fixed
	ChecksumAdjustment uint32 // to be calculated
	MagicNumber        uint32 // 0x5F0F3CF5
	Flags              uint16
	UnitsPerEm         uint16
	Created            uint64
	Modified           uint64
	XMin               uint16
	YMin               uint16
	XMax               uint16
	YMax               uint16
	MacStyle           uint16
	LowestRecPPEM      uint16
	FontDirectionHint  int16
	IndexToLocFormat   int16
	GlyphDataFormat    int16
}

// Maxp table maxp
type Maxp struct {
	Version               uint32
	NumGlyphs             uint16
	MaxPoints             uint16
	MaxContours           uint16
	MaxCompositePoints    uint16
	MaxCompositeContours  uint16
	MaxZones              uint16
	MaxTwilightPoints     uint16
	MaxStorage            uint16
	MaxFunctionDefs       uint16
	MaxInstructionDefs    uint16
	MaxStackElements      uint16
	MaxSizeOfInstructions uint16
	MaxComponentElements  uint16
	MaxComponentDepth     uint16
}

// Post table
type Post struct {
	Version            uint32
	ItalicAngle        int32
	UnderlinePosition  int16
	UnderlineThickness int16
	IsFixedPitch       uint32
	MinMemType42       uint32
	MaxMemType42       uint32
	MinMemType1        uint32
	MaxMemType1        uint32
	NumGlyphs          uint16
}

// OS2 OS/2 font table
type OS2 struct {
	Version             uint16
	XAvgCharWidth       int16
	UsWeightClass       uint16
	UsWidthClass        uint16
	FsType              uint16
	YSubscriptXSize     int16
	YSubscriptYSize     int16
	YSubscriptXOffset   int16
	YSubscriptYOffset   int16
	YSuperscriptXSize   int16
	YSuperscriptYSize   int16
	YSuperscriptXOffset int16
	YSuperscriptYOffset int16
	YStrikeoutSize      int16
	YStrikeoutPosition  int16
	SFamilyClass        int16
	Panose              [10]uint8
	UlUnicodeRange1     uint32
	UlUnicodeRange2     uint32
	UlUnicodeRange3     uint32
	UlUnicodeRange4     uint32
	AchVendID           uint32
	FsSelection         uint16
	UsFirstCharIndex    uint16
	UsLastCharIndex     uint16
	STypoAscender       int16
	STypoDescender      int16
	STypoLineGap        int16
	UsWinAscent         uint16
	UsWinDescent        uint16
}

// OS2AdditionalFields has version > 0 fields
type OS2AdditionalFields struct {
	UlCodePageRange1        uint32
	UlCodePageRange2        uint32
	SxHeight                int16
	SCapHeight              int16
	UsDefaultChar           uint16
	UsBreakChar             uint16
	UsMaxContext            uint16
	UsLowerOpticalPointSize uint16
	UsUpperOpticalPointSize uint16
}

var macGlyphNames = []string{
	".notdef", ".null", "nonmarkingreturn", "space", "exclam", "quotedbl",
	"numbersign", "dollar", "percent", "ampersand", "quotesingle",
	"parenleft", "parenright", "asterisk", "plus", "comma", "hyphen",
	"period", "slash", "zero", "one", "two", "three", "four", "five",
	"six", "seven", "eight", "nine", "colon", "semicolon", "less",
	"equal", "greater", "question", "at", "A", "B", "C", "D", "E", "F",
	"G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S",
	"T", "U", "V", "W", "X", "Y", "Z", "bracketleft", "backslash",
	"bracketright", "asciicircum", "underscore", "grave", "a", "b",
	"c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o",
	"p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z", "braceleft",
	"bar", "braceright", "asciitilde", "Adieresis", "Aring",
	"Ccedilla", "Eacute", "Ntilde", "Odieresis", "Udieresis", "aacute",
	"agrave", "acircumflex", "adieresis", "atilde", "aring",
	"ccedilla", "eacute", "egrave", "ecircumflex", "edieresis",
	"iacute", "igrave", "icircumflex", "idieresis", "ntilde", "oacute",
	"ograve", "ocircumflex", "odieresis", "otilde", "uacute", "ugrave",
	"ucircumflex", "udieresis", "dagger", "degree", "cent", "sterling",
	"section", "bullet", "paragraph", "germandbls", "registered",
	"copyright", "trademark", "acute", "dieresis", "notequal", "AE",
	"Oslash", "infinity", "plusminus", "lessequal", "greaterequal",
	"yen", "mu", "partialdiff", "summation", "product", "pi",
	"integral", "ordfeminine", "ordmasculine", "Omega", "ae", "oslash",
	"questiondown", "exclamdown", "logicalnot", "radical", "florin",
	"approxequal", "Delta", "guillemotleft", "guillemotright",
	"ellipsis", "nonbreakingspace", "Agrave", "Atilde", "Otilde", "OE",
	"oe", "endash", "emdash", "quotedblleft", "quotedblright",
	"quoteleft", "quoteright", "divide", "lozenge", "ydieresis",
	"Ydieresis", "fraction", "currency", "guilsinglleft",
	"guilsinglright", "fi", "fl", "daggerdbl", "periodcentered",
	"quotesinglbase", "quotedblbase", "perthousand", "Acircumflex",
	"Ecircumflex", "Aacute", "Edieresis", "Egrave", "Iacute",
	"Icircumflex", "Idieresis", "Igrave", "Oacute", "Ocircumflex",
	"apple", "Ograve", "Uacute", "Ucircumflex", "Ugrave", "dotlessi",
	"circumflex", "tilde", "macron", "breve", "dotaccent", "ring",
	"cedilla", "hungarumlaut", "ogonek", "caron", "Lslash", "lslash",
	"Scaron", "scaron", "Zcaron", "zcaron", "brokenbar", "Eth", "eth",
	"Yacute", "yacute", "Thorn", "thorn", "minus", "multiply",
	"onesuperior", "twosuperior", "threesuperior", "onehalf",
	"onequarter", "threequarters", "franc", "Gbreve", "gbreve",
	"Idotaccent", "Scedilla", "scedilla", "Cacute", "cacute", "Ccaron",
	"ccaron", "dcroat",
}

var macGlyphNameIndex = map[string]uint16{
	".notdef":          0,
	".null":            1,
	"nonmarkingreturn": 2,
	"space":            3,
	"exclam":           4,
	"quotedbl":         5,
	"numbersign":       6,
	"dollar":           7,
	"percent":          8,
	"ampersand":        9,
	"quotesingle":      10,
	"parenleft":        11,
	"parenright":       12,
	"asterisk":         13,
	"plus":             14,
	"comma":            15,
	"hyphen":           16,
	"period":           17,
	"slash":            18,
	"zero":             19,
	"one":              20,
	"two":              21,
	"three":            22,
	"four":             23,
	"five":             24,
	"six":              25,
	"seven":            26,
	"eight":            27,
	"nine":             28,
	"colon":            29,
	"semicolon":        30,
	"less":             31,
	"equal":            32,
	"greater":          33,
	"question":         34,
	"at":               35,
	"A":                36,
	"B":                37,
	"C":                38,
	"D":                39,
	"E":                40,
	"F":                41,
	"G":                42,
	"H":                43,
	"I":                44,
	"J":                45,
	"K":                46,
	"L":                47,
	"M":                48,
	"N":                49,
	"O":                50,
	"P":                51,
	"Q":                52,
	"R":                53,
	"S":                54,
	"T":                55,
	"U":                56,
	"V":                57,
	"W":                58,
	"X":                59,
	"Y":                60,
	"Z":                61,
	"bracketleft":      62,
	"backslash":        63,
	"bracketright":     64,
	"asciicircum":      65,
	"underscore":       66,
	"grave":            67,
	"a":                68,
	"b":                69,
	"c":                70,
	"d":                71,
	"e":                72,
	"f":                73,
	"g":                74,
	"h":                75,
	"i":                76,
	"j":                77,
	"k":                78,
	"l":                79,
	"m":                80,
	"n":                81,
	"o":                82,
	"p":                83,
	"q":                84,
	"r":                85,
	"s":                86,
	"t":                87,
	"u":                88,
	"v":                89,
	"w":                90,
	"x":                91,
	"y":                92,
	"z":                93,
	"braceleft":        94,
	"bar":              95,
	"braceright":       96,
	"asciitilde":       97,
	"Adieresis":        98,
	"Aring":            99,
	"Ccedilla":         100,
	"Eacute":           101,
	"Ntilde":           102,
	"Odieresis":        103,
	"Udieresis":        104,
	"aacute":           105,
	"agrave":           106,
	"acircumflex":      107,
	"adieresis":        108,
	"atilde":           109,
	"aring":            110,
	"ccedilla":         111,
	"eacute":           112,
	"egrave":           113,
	"ecircumflex":      114,
	"edieresis":        115,
	"iacute":           116,
	"igrave":           117,
	"icircumflex":      118,
	"idieresis":        119,
	"ntilde":           120,
	"oacute":           121,
	"ograve":           122,
	"ocircumflex":      123,
	"odieresis":        124,
	"otilde":           125,
	"uacute":           126,
	"ugrave":           127,
	"ucircumflex":      128,
	"udieresis":        129,
	"dagger":           130,
	"degree":           131,
	"cent":             132,
	"sterling":         133,
	"section":          134,
	"bullet":           135,
	"paragraph":        136,
	"germandbls":       137,
	"registered":       138,
	"copyright":        139,
	"trademark":        140,
	"acute":            141,
	"dieresis":         142,
	"notequal":         143,
	"AE":               144,
	"Oslash":           145,
	"infinity":         146,
	"plusminus":        147,
	"lessequal":        148,
	"greaterequal":     149,
	"yen":              150,
	"mu":               151,
	"partialdiff":      152,
	"summation":        153,
	"product":          154,
	"pi":               155,
	"integral":         156,
	"ordfeminine":      157,
	"ordmasculine":     158,
	"Omega":            159,
	"ae":               160,
	"oslash":           161,
	"questiondown":     162,
	"exclamdown":       163,
	"logicalnot":       164,
	"radical":          165,
	"florin":           166,
	"approxequal":      167,
	"Delta":            168,
	"guillemotleft":    169,
	"guillemotright":   170,
	"ellipsis":         171,
	"nonbreakingspace": 172,
	"Agrave":           173,
	"Atilde":           174,
	"Otilde":           175,
	"OE":               176,
	"oe":               177,
	"endash":           178,
	"emdash":           179,
	"quotedblleft":     180,
	"quotedblright":    181,
	"quoteleft":        182,
	"quoteright":       183,
	"divide":           184,
	"lozenge":          185,
	"ydieresis":        186,
	"Ydieresis":        187,
	"fraction":         188,
	"currency":         189,
	"guilsinglleft":    190,
	"guilsinglright":   191,
	"fi":               192,
	"fl":               193,
	"daggerdbl":        194,
	"periodcentered":   195,
	"quotesinglbase":   196,
	"quotedblbase":     197,
	"perthousand":      198,
	"Acircumflex":      199,
	"Ecircumflex":      200,
	"Aacute":           201,
	"Edieresis":        202,
	"Egrave":           203,
	"Iacute":           204,
	"Icircumflex":      205,
	"Idieresis":        206,
	"Igrave":           207,
	"Oacute":           208,
	"Ocircumflex":      209,
	"apple":            210,
	"Ograve":           211,
	"Uacute":           212,
	"Ucircumflex":      213,
	"Ugrave":           214,
	"dotlessi":         215,
	"circumflex":       216,
	"tilde":            217,
	"macron":           218,
	"breve":            219,
	"dotaccent":        220,
	"ring":             221,
	"cedilla":          222,
	"hungarumlaut":     223,
	"ogonek":           224,
	"caron":            225,
	"Lslash":           226,
	"lslash":           227,
	"Scaron":           228,
	"scaron":           229,
	"Zcaron":           230,
	"zcaron":           231,
	"brokenbar":        232,
	"Eth":              233,
	"eth":              234,
	"Yacute":           235,
	"yacute":           236,
	"Thorn":            237,
	"thorn":            238,
	"minus":            239,
	"multiply":         240,
	"onesuperior":      241,
	"twosuperior":      242,
	"threesuperior":    243,
	"onehalf":          244,
	"onequarter":       245,
	"threequarters":    246,
	"franc":            247,
	"Gbreve":           248,
	"gbreve":           249,
	"Idotaccent":       250,
	"Scedilla":         251,
	"scedilla":         252,
	"Cacute":           253,
	"cacute":           254,
	"Ccaron":           255,
	"ccaron":           256,
	"dcroat":           257,
}
