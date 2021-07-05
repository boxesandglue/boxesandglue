package truetype

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"unicode/utf16"
)

var (
	subsetGlyphIds chan int
)

func genIntegerSequence(ids chan int) {
	i := int(1)
	for {
		ids <- i
		i++
	}
}

func init() {
	subsetGlyphIds = make(chan int)
	go genIntegerSequence(subsetGlyphIds)
}

func (tt *Font) String() string {
	return "font"
}

func (tt *Font) read(data interface{}) {
	err := binary.Read(tt.r, binary.BigEndian, data)
	if err != nil {
		panic(err)
	}
}

func (tt *Font) write(w io.Writer, data interface{}) {
	err := binary.Write(w, binary.BigEndian, data)
	if err != nil {
		panic(err)
	}
}

func (tt *Font) readToTableEnd(name string) []byte {
	cur, _ := tt.r.Seek(0, os.SEEK_CUR)
	tbl := tt.tables[name]
	lastbytePos := tbl.offset + tbl.length
	l := int64(lastbytePos) - cur
	buf := make([]byte, l)
	tt.read(buf)
	return buf
}

func (tt *Font) fixed() float64 {
	var a int16
	var b uint16
	tt.read(&a)
	tt.read(&b)
	return float64(a) + float64(b)/65536
}

// ReadTableData does not interpret the bytes read.
func (tt *Font) ReadTableData(tbl string) ([]byte, error) {
	off := int64(tt.tables[tbl].offset)
	_, err := tt.r.Seek(off, os.SEEK_SET)
	if err != nil {
		return nil, err
	}

	l := int(tt.tables[tbl].length)
	buf := make([]byte, l)
	n, err := tt.r.Read(buf)
	if err != nil {
		return nil, err
	}
	if n != l {
		return nil, fmt.Errorf("not enough bytes read")
	}

	return buf, nil
}

func (tt *Font) readTable(tbl string) error {
	thistable := tt.tables[tbl]
	off := int64(thistable.offset)

	_, err := tt.r.Seek(off, os.SEEK_SET)
	if err != nil {
		return err
	}
	tt.tablesRead[tbl] = true
	switch tbl {
	case "head":
		if err = tt.readHead(thistable); err != nil {
			return err
		}
	case "loca":
		if err = tt.readLoca(thistable); err != nil {
			return err
		}
	case "hmtx":
		if err = tt.readHmtx(thistable); err != nil {
			return err
		}
	case "fpgm":
		if err = tt.readFpgm(thistable); err != nil {
			return err
		}
	case "cvt ":
		if err = tt.readCvt(thistable); err != nil {
			return err
		}
	case "prep":
		if err = tt.readPrep(thistable); err != nil {
			return err
		}
	case "glyf":
		if err = tt.readGlyf(thistable); err != nil {
			return err
		}
	case "maxp":
		if err = tt.readMaxp(thistable); err != nil {
			return err
		}
	case "post":
		if err = tt.readPost(thistable); err != nil {
			return err
		}
	case "OS/2":
		if err = tt.readOs2(thistable); err != nil {
			return err
		}
	case "cmap":
		if err = tt.readCmap(thistable); err != nil {
			return err
		}
	case "name":
		tt.readName(off)
	// case "kern":
	// 	// tt.readKern(off)
	case "hhea":
		tt.readHhea(off)
	default:
		// fmt.Printf("    skip table %s\n", tbl)
	}
	return nil
}

// WriteTable writes the table to w.
func (tt *Font) WriteTable(w io.Writer, tbl string) error {
	switch tbl {
	case "loca":
		tt.writeLoca(w)
	case "hhea":
		tt.writeHhea(w)
	case "head":
		tt.writeHead(w)
	case "maxp":
		tt.writeMaxp(w)
	case "hmtx":
		tt.writeHmtx(w)
	case "fpgm":
		tt.writeFpgm(w)
	case "cvt ":
		tt.writeCvt(w)
	case "prep":
		tt.writePrep(w)
	case "glyf":
		tt.writeGlyf(w)
	case "post":
		tt.writePost(w)
	case "OS/2":
		tt.writeOs2(w)
	default:
		// fmt.Printf("    skip write table %s\n", tbl)
	}
	return nil
}

func (tt *Font) readName(offset int64) error {
	var version, count, stringOffset uint16
	tt.read(&version)
	tt.read(&count)
	tt.read(&stringOffset)

	type nameentry struct {
		platformID, encodingID, languageID, nameID, length uint16
		offset                                             int64
	}
	var names []nameentry
	switch version {
	case 0:
		for i := uint16(0); i < count; i++ {
			ne := nameentry{}
			tt.read(&ne.platformID)
			tt.read(&ne.encodingID)
			tt.read(&ne.languageID)
			tt.read(&ne.nameID)
			tt.read(&ne.length)
			var o uint16
			tt.read(&o)
			ne.offset = int64(o)
			names = append(names, ne)
		}
		for _, ne := range names {
			tt.r.Seek(ne.offset+int64(stringOffset)+offset, os.SEEK_SET)
			if _, ok := tt.names[int(ne.nameID)]; ok {
				continue
			}
			if ne.platformID == 3 && ne.encodingID == 1 {
				var dec []uint16
				c := uint16(0)
				for {
					c += 2
					if c > ne.length {
						break
					}
					var char uint16
					tt.read(&char)
					dec = append(dec, char)
				}
				tt.names[int(ne.nameID)] = string(utf16.Decode(dec))
			} else {
				buf := make([]byte, ne.length)
				tt.r.Read(buf)
				tt.names[int(ne.nameID)] = string(buf)
			}
		}
	default:
		panic("not implemented yet: name version != 0")
	}
	return nil
}

func (tt *Font) readHhea(offset int64) error {
	hhea := Hhea{}
	var reserved int16

	tt.read(&hhea.MajorVersion)
	tt.read(&hhea.MinorVersion)
	tt.read(&hhea.Ascender)
	tt.read(&hhea.Descender)
	tt.read(&hhea.LineGap)

	tt.read(&hhea.AdvanceWidthMax)
	tt.read(&hhea.MinLeftSideBearing)
	tt.read(&hhea.MinRightSideBearing)
	tt.read(&hhea.XMaxExtent)
	tt.read(&hhea.CaretSlopeRise)
	tt.read(&hhea.CaretSlopeRun)
	tt.read(&hhea.CaretOffset)

	tt.read(&reserved)
	tt.read(&reserved)
	tt.read(&reserved)
	tt.read(&reserved)

	tt.read(&hhea.MetricDataFormat)
	tt.read(&hhea.NumberOfHMetrics)
	tt.Hhea = hhea
	return nil
}

func (tt *Font) writeHhea(w io.Writer) {
	tbl := tt.Hhea
	tt.write(w, tbl.MajorVersion)
	tt.write(w, tbl.MinorVersion)

	var reserved int16

	tt.write(w, tbl.Ascender)
	tt.write(w, tbl.Descender)
	tt.write(w, tbl.LineGap)

	tt.write(w, tbl.AdvanceWidthMax)
	tt.write(w, tbl.MinLeftSideBearing)
	tt.write(w, tbl.MinRightSideBearing)
	tt.write(w, tbl.XMaxExtent)
	tt.write(w, tbl.CaretSlopeRise)
	tt.write(w, tbl.CaretSlopeRun)
	tt.write(w, tbl.CaretOffset)

	tt.write(w, reserved)
	tt.write(w, reserved)
	tt.write(w, reserved)
	tt.write(w, reserved)

	tt.write(w, tbl.MetricDataFormat)
	tt.write(w, tbl.NumberOfHMetrics)
}

func (tt *Font) readHmtx(tbl tableOffsetLength) error {
	numMetrics := tt.Hhea.NumberOfHMetrics
	// todo: if numMetrics < numGlyphs { read lsb }
	// numGlyphs := tt.Maxp.NumGlyphs
	numLSB := tt.Maxp.NumGlyphs - numMetrics
	tt.advanceWidth = make([]uint16, numMetrics)
	tt.lsb = make([]int16, numMetrics+numLSB)
	for i := 0; i < int(numMetrics); i++ {
		tt.read(&tt.advanceWidth[i])
		tt.read(&tt.lsb[i])
	}
	for i := 0; i < int(numLSB); i++ {
		tt.read(&tt.lsb[i+int(numMetrics)])
	}
	return nil
}

func (tt *Font) writeHmtx(w io.Writer) {
	l := len(tt.advanceWidth)
	for i := 0; i < l; i++ {
		tt.write(w, tt.advanceWidth[i])
		tt.write(w, tt.lsb[i])
	}
	lLsb := len(tt.lsb) - l

	for i := 0; i < lLsb; i++ {
		tt.write(w, tt.lsb[i+l])
	}
}

// Font program
func (tt *Font) readFpgm(tbl tableOffsetLength) error {
	tt.fpgm = make([]byte, tt.tables["fpgm"].length)
	_, err := tt.r.Read(tt.fpgm)
	if err != nil {
		return err
	}
	return nil
}

func (tt *Font) writeFpgm(w io.Writer) {
	w.Write(tt.fpgm)
}

// Font program
func (tt *Font) readCvt(tbl tableOffsetLength) error {
	tt.cvt = make([]byte, tt.tables["cvt "].length)
	tt.r.Read(tt.cvt)
	return nil
}

func (tt *Font) writeCvt(w io.Writer) {
	w.Write(tt.cvt)
}

// Font program
func (tt *Font) readPrep(tbl tableOffsetLength) error {
	tt.prep = make([]byte, tt.tables["prep"].length)
	tt.r.Read(tt.prep)
	return nil
}

func (tt *Font) writePrep(w io.Writer) {
	w.Write(tt.prep)
}

func (tt *Font) readLoca(tbl tableOffsetLength) error {
	version := tt.Head.IndexToLocFormat
	// one extra offset at the end
	numGlyphs := tt.Maxp.NumGlyphs
	tt.glyphOffsets = make([]uint32, numGlyphs+1)

	switch version {
	case 0:
		var offset uint16
		for i := 0; i <= int(numGlyphs); i++ {
			tt.read(&offset)
			tt.glyphOffsets[i] = uint32(offset) * 2
		}
	case 1:
		var offset uint32
		for i := 0; i <= int(numGlyphs); i++ {
			tt.read(&offset)
			tt.glyphOffsets[i] = offset
		}
	}

	return nil
}

func (tt *Font) readOs2(tbl tableOffsetLength) error {
	os2tbl := OS2{}
	tt.read(&os2tbl)
	add := OS2AdditionalFields{}

	if os2tbl.Version > 0 {
		tt.read(&add.UlCodePageRange1)
		tt.read(&add.UlCodePageRange2)
	}

	if os2tbl.Version > 1 {
		tt.read(&add.SxHeight)
		tt.read(&add.SCapHeight)
		tt.read(&add.UsDefaultChar)
		tt.read(&add.UsBreakChar)
		tt.read(&add.UsMaxContext)
	}

	if os2tbl.Version > 4 {
		tt.read(&add.UsLowerOpticalPointSize)
		tt.read(&add.UsUpperOpticalPointSize)
	}
	tt.OS2 = os2tbl
	tt.OS2AdditionalFields = add

	return nil
}

func (tt *Font) writeOs2(w io.Writer) {
	tt.write(w, tt.OS2)
}

func (tt *Font) readHead(tbl tableOffsetLength) error {
	head := Head{}

	tt.read(&head.MajorVersion)
	tt.read(&head.MinorVersion)
	tt.read(&head.FontRevision)
	tt.read(&head.ChecksumAdjustment)
	tt.read(&head.MagicNumber)
	tt.read(&head.Flags)
	tt.read(&head.UnitsPerEm)
	tt.read(&head.Created)
	tt.read(&head.Modified)
	tt.read(&head.XMin)
	tt.read(&head.YMin)
	tt.read(&head.XMax)
	tt.read(&head.YMax)
	tt.read(&head.MacStyle)
	tt.read(&head.LowestRecPPEM)
	tt.read(&head.FontDirectionHint)
	tt.read(&head.IndexToLocFormat)
	tt.read(&head.GlyphDataFormat)

	tt.Head = head
	return nil
}

func (tt *Font) readMaxp(tbl tableOffsetLength) error {
	maxp := Maxp{}

	tt.read(&maxp.Version)
	tt.read(&maxp.NumGlyphs)

	switch maxp.Version {
	case 0x10000:
		tt.read(&maxp.MaxPoints)
		tt.read(&maxp.MaxContours)
		tt.read(&maxp.MaxCompositePoints)
		tt.read(&maxp.MaxCompositeContours)
		tt.read(&maxp.MaxZones)
		tt.read(&maxp.MaxTwilightPoints)
		tt.read(&maxp.MaxStorage)
		tt.read(&maxp.MaxFunctionDefs)
		tt.read(&maxp.MaxInstructionDefs)
		tt.read(&maxp.MaxStackElements)
		tt.read(&maxp.MaxSizeOfInstructions)
		tt.read(&maxp.MaxComponentElements)
		tt.read(&maxp.MaxComponentDepth)
	default:
		// version 0.5 only has NumGlyphs
	}

	tt.Maxp = maxp
	return nil
}

func (tt *Font) writeLoca(w io.Writer) {
	if tt.Head.IndexToLocFormat == -1 {
		tt.readTable("head")
	}
	version := tt.Head.IndexToLocFormat
	switch version {
	case 0:
		var offset uint16
		for _, off := range tt.glyphOffsets {
			offset = uint16(off / 2)
			tt.write(w, offset)
		}
	case 1:
		for _, off := range tt.glyphOffsets {
			tt.write(w, off)
		}
	}
}

func (tt *Font) writeMaxp(w io.Writer) {
	tbl := tt.Maxp
	tt.write(w, tbl.Version)
	tt.write(w, tbl.NumGlyphs)

	switch tbl.Version {
	case 0x10000:
		tt.write(w, tbl.MaxPoints)
		tt.write(w, tbl.MaxContours)
		tt.write(w, tbl.MaxCompositePoints)
		tt.write(w, tbl.MaxCompositeContours)
		tt.write(w, tbl.MaxZones)
		tt.write(w, tbl.MaxTwilightPoints)
		tt.write(w, tbl.MaxStorage)
		tt.write(w, tbl.MaxFunctionDefs)
		tt.write(w, tbl.MaxInstructionDefs)
		tt.write(w, tbl.MaxStackElements)
		tt.write(w, tbl.MaxSizeOfInstructions)
		tt.write(w, tbl.MaxComponentElements)
		tt.write(w, tbl.MaxComponentDepth)
	default:
		// version 0.5 only has NumGlyphs
	}
}

func (tt *Font) writeHead(w io.Writer) {
	tt.write(w, tt.Head)
}

func (tt *Font) getGlyphComponentIds(codepoint int) (components []int) {

	if codepoint == 0 {
		return
	}
	g := tt.Glyph[codepoint]

	if len(g) == 0 {
		return
	}
	var numberOfContours int16
	saveR := tt.r
	defer func() {
		tt.r = saveR
	}()
	tt.r = bytes.NewReader(g)
	tt.read(&numberOfContours)
	if numberOfContours >= 0 {
		return
	}

	tt.r.Seek(8, os.SEEK_CUR)
	var flags uint16
	var componentIndex uint16

	for {
		tt.read(&flags)
		tt.read(&componentIndex)

		components = append(components, int(componentIndex))
		components = append(components, tt.getGlyphComponentIds(int(componentIndex))...)
		if flags&flagMoreComponents == 0 {
			break
		}
		if flags&flagArg1And2AreWords != 0 {
			tt.r.Seek(4, os.SEEK_CUR)
		} else {
			tt.r.Seek(2, os.SEEK_CUR)
		}

		switch {
		case flags&flagWeHaveAScale != 0:
			tt.r.Seek(2, os.SEEK_CUR)
		case flags&flagWeHaveAnXAndYScale != 0:
			tt.r.Seek(4, os.SEEK_CUR)
		case flags&flags&flagWeHaveATwoByTwo != 0:
			tt.r.Seek(8, os.SEEK_CUR)
		}
	}
	return
}

func (tt *Font) readGlyf(tbl tableOffsetLength) error {
	if len(tt.glyphOffsets) == 0 {
		tt.readTable("loca")
	}
	if tt.Maxp.NumGlyphs == 0 {
		tt.readTable("maxp")
	}

	data, err := tt.ReadTableData("glyf")
	if err != nil {
		return err
	}
	c := uint32(0)
	numGlyphs := tt.Maxp.NumGlyphs
	tt.Glyph = make([]Glyph, numGlyphs)
	for i := 0; i < int(numGlyphs); i++ {
		tt.Glyph[i] = data[c:tt.glyphOffsets[i+1]]
		c = tt.glyphOffsets[i+1]
	}
	return nil
}

func (tt *Font) writeGlyf(w io.Writer) {
	glyphOffsets := make([]uint32, len(tt.Glyph))
	c := uint32(0)
	for i, g := range tt.Glyph {
		w.Write(g)
		glyphOffsets[i] = c
		lg := uint32(len(g))
		c += lg
	}
	glyphOffsets = append(glyphOffsets, c)
	tt.Maxp.NumGlyphs = uint16(len(tt.Glyph))
	tt.glyphOffsets = glyphOffsets
}

func (tt *Font) readKern(tbl tableOffsetLength) error {
	var version, nTables uint16
	tt.read(&version)
	tt.read(&nTables)

	for i := uint16(0); i < nTables; i++ {
		tt.read(&version)
	}
	return nil
}

func (tt *Font) readCmap(tbl tableOffsetLength) error {
	var version uint16
	var subtables uint16
	tt.read(&version)
	tt.read(&subtables)

	type cmaptbl struct {
		platform, encoding uint16
	}

	cmaptables := make(map[uint32]cmaptbl)
	var offsetCMap uint32

	for i := uint16(0); i < subtables; i++ {
		ct := cmaptbl{}
		tt.read(&ct.platform)
		tt.read(&ct.encoding)
		tt.read(&offsetCMap)

		cmaptables[offsetCMap] = ct
	}
	rawTable, err := tt.ReadTableData("cmap")
	if err != nil {
		return err
	}
	for offsetCMap := range cmaptables {
		tt.r.Seek(int64(tbl.offset)+int64(offsetCMap), os.SEEK_SET)
		var format uint16
		tt.read(&format)

		tt.ToCodepoint = make(map[rune]int, tt.Maxp.NumGlyphs)
		tt.ToUni = make(map[int]rune, tt.Maxp.NumGlyphs)
		switch format {
		case 4:
			// Segment mapping to delta values
			var length uint16
			var language uint16
			var segCount uint16
			var searchRange uint16
			var entrySelector uint16
			var rangeShift uint16
			var reservedPad uint16
			// 16
			var endCode, startCode, idRangeOffsets []uint16
			var idDelta []int16
			// 16 + 4 * 2 * segCountX2 / 2 = 16 + 4 * segCountX2
			tt.read(&length)
			tt.read(&language)
			tt.read(&segCount)
			segCount /= 2
			tt.read(&searchRange)
			tt.read(&entrySelector)
			tt.read(&rangeShift)

			endCode = make([]uint16, segCount)
			startCode = make([]uint16, segCount)
			idRangeOffsets = make([]uint16, segCount)
			idDelta = make([]int16, segCount)
			pos := offsetCMap + 16 + uint32(8*segCount)

			tt.read(endCode)
			tt.read(&reservedPad)
			tt.read(startCode)
			tt.read(idDelta)
			tt.read(idRangeOffsets)
			for i := 0; i < int(segCount); i++ {
				s := startCode[i]
				e := endCode[i]
				delta := idDelta[i]
				ro := uint32(idRangeOffsets[i])

				if s == 0xffff {
					break
				}
				if ro == 0 {
					for j := s; j <= e; j++ {
						tt.ToUni[int(j)+int(delta)] = rune(j)
						tt.ToCodepoint[rune(j)] = int(j) + int(delta)
					}
				} else {
					for j := s; j <= e; j++ {
						offset := uint32(ro) + 2*uint32(i-int(segCount)+int(j-s))
						cp := int(rawTable[pos+offset])<<8 + int(rawTable[pos+offset+1])
						tt.ToUni[cp] = rune(j)
						tt.ToCodepoint[rune(j)] = cp
					}
				}
			}
		case 6:
			// Trimmed table mapping
			// ignore
		case 12:
			// Segmented coverage
			var zero uint16
			var length, language, ngroups uint32
			var startCharCode, endCharCode, startGlyphID uint32
			tt.read(&zero)
			tt.read(&length)
			tt.read(&language)
			tt.read(&ngroups)

			for i := uint32(0); i < ngroups; i++ {
				tt.read(&startCharCode)
				tt.read(&endCharCode)
				tt.read(&startGlyphID)
				for i, c := 0, startCharCode; c <= endCharCode; i, c = i+1, c+1 {
					tt.ToUni[int(startGlyphID)+i] = rune(c)
					tt.ToCodepoint[rune(c)] = int(startGlyphID) + i
				}
			}
		default:
			return fmt.Errorf("format %d not supported in cmap", format)
		}
	}

	return nil
}

func (tt *Font) readPost(tbl tableOffsetLength) error {

	post := Post{}
	tt.read(&post.Version)
	tt.read(&post.ItalicAngle)
	tt.read(&post.UnderlinePosition)
	tt.read(&post.UnderlineThickness)
	tt.read(&post.IsFixedPitch)
	tt.read(&post.MinMemType42)
	tt.read(&post.MaxMemType42)
	tt.read(&post.MinMemType1)
	tt.read(&post.MaxMemType1)

	switch post.Version {
	case 0x10000:
		// no more fields
	case 0x20000:
		var numGlyphs uint16
		tt.read(&numGlyphs)
		glyphNameIndex := make([]uint16, numGlyphs)
		tt.read(glyphNameIndex)
		data := tt.readToTableEnd("post")
		fontGylphNames := make([]string, 0, len(glyphNameIndex))
		c := 0
		for i := 0; c < len(data); i++ {
			l := int(data[c])
			fontGylphNames = append(fontGylphNames, string(data[c+1:c+1+l]))
			c = c + l + 1
		}
		tt.GlyphNames = make([]string, 0, len(glyphNameIndex))
		for _, idx := range glyphNameIndex {
			if idx <= 257 {
				tt.GlyphNames = append(tt.GlyphNames, macGlyphNames[idx])
			} else {
				tt.GlyphNames = append(tt.GlyphNames, fontGylphNames[idx-258])
			}
		}
	case 0x25000:
		panic("not implemented yet (POST version 2.5)")
	case 0x30000:
		// no more fields
	case 0x40000:
		// AAT
		panic("not implemented yet (POST version 4)")
	}

	tt.Post = post
	return nil
}

func (tt *Font) writePost(w io.Writer) {
	tbl := tt.Post

	tt.write(w, tbl.Version)
	tt.write(w, tbl.ItalicAngle)
	tt.write(w, tbl.UnderlinePosition)
	tt.write(w, tbl.UnderlineThickness)
	tt.write(w, tbl.IsFixedPitch)
	tt.write(w, tbl.MinMemType42)
	tt.write(w, tbl.MaxMemType42)
	tt.write(w, tbl.MinMemType1)
	tt.write(w, tbl.MaxMemType1)
	tt.write(w, tt.Maxp.NumGlyphs)

	fontGlyphNames := make([]string, 0, tt.Maxp.NumGlyphs)
	glyphIndex := make([]uint16, tt.Maxp.NumGlyphs)

	for i, n := range tt.GlyphNames {
		if idx, ok := macGlyphNameIndex[n]; ok {
			glyphIndex[i] = idx
		} else {
			fontGlyphNames = append(fontGlyphNames, n)
			glyphIndex[i] = uint16(len(fontGlyphNames)) + 258
		}
	}
	tt.write(w, glyphIndex)
	for _, n := range fontGlyphNames {
		tt.write(w, byte(len(n)))
		tt.write(w, []byte(n))
	}
}

// LoadFace loads a truetype font
func LoadFace(filename string) (*Font, error) {
	r, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	return Open(r)
}

// Open initializes the TrueType font
func Open(r io.ReadSeeker) (*Font, error) {
	tt := &Font{}
	tt.r = r
	tt.tables = make(map[string]tableOffsetLength)
	tt.tablesRead = make(map[string]bool)
	tt.names = make(map[int]string)
	// to mark that the Head table is not read yet
	tt.Head.IndexToLocFormat = -1
	tt.read(&tt.sfntVersion)

	switch tt.sfntVersion {
	case 65536:
		// OK
	default:
		return nil, fmt.Errorf("unknown magic %v", tt.sfntVersion)
	}

	var numtables uint16
	tt.read(&numtables)

	for i := uint16(0); i < numtables; i++ {
		ol := tableOffsetLength{}
		pos := int64(16*i + 12)
		r.Seek(pos, os.SEEK_SET)
		tbl := make([]byte, 4)
		n, err := r.Read(tbl)
		if n != 4 {
			return nil, errors.New("n should be 4")
		}
		if err != nil {
			return nil, err
		}
		tblname := string(tbl)
		ol.name = tblname
		// checksum ignored
		r.Seek(4, os.SEEK_CUR)
		tt.read(&ol.offset)
		tt.read(&ol.length)
		tt.tables[tblname] = ol
	}

	return tt, nil
}

// ReadTables reads all tables from the font file
func (tt *Font) ReadTables() {
	interestingTables := []string{"head", "hhea", "maxp", "loca", "hmtx", "fpgm", "cvt ", "prep", "glyf", "post", "OS/2", "name", "cmap"}
	for _, tblname := range interestingTables {
		tt.readTable(tblname)
	}

}

// WriteSubset writes a valid font to w that is suitable for including in PDF
func (tt *Font) WriteSubset(w io.Writer) error {
	var err error

	var fontfile bytes.Buffer
	tt.Head.ChecksumAdjustment = 0
	interestingTables := []string{"cvt ", "glyf", "head", "hhea", "hmtx", "loca", "maxp", "prep"}
	tablesForPDF := []tableOffsetLength{}

	// put only those tables in PDF which are present in the font file
	for _, tblname := range interestingTables {
		if tbl, ok := tt.tables[tblname]; ok {
			tbl.name = tblname
			tablesForPDF = append(tablesForPDF, tbl)
		}
	}
	// tables start at 12 (header) + table toc
	tableOffset := uint32(12 + 16*len(tablesForPDF))

	var newTables []tableOffsetLength
	for _, tbl := range tablesForPDF {
		var tabledata bytes.Buffer
		err = tt.WriteTable(&tabledata, tbl.name)
		if err != nil {
			return err
		}
		l := tabledata.Len()
		nt := tableOffsetLength{
			length: uint32(l),
			name:   tbl.name,
			offset: tableOffset,
		}

		switch l & 3 {
		case 0:
			// ok, no alignment
		case 1:
			tt.write(&tabledata, uint16(0))
			tt.write(&tabledata, uint8(0))
			l += 3
		case 2:
			tt.write(&tabledata, uint16(0))
			l += 2
		case 3:
			tt.write(&tabledata, uint8(0))
			l++
		}
		nt.tabledata = tabledata.Bytes()
		tableOffset += uint32(len(nt.tabledata))
		nt.checksum = calcChecksum(nt.tabledata)
		newTables = append(newTables, nt)
	}

	tt.write(&fontfile, tt.sfntVersion)
	cTablesRead := float64(len(newTables))
	searchRange := (math.Pow(2, math.Floor(math.Log2(cTablesRead))) * 16)
	entrySelector := math.Floor(math.Log2(cTablesRead))
	rangeShift := (cTablesRead * 16.0) - searchRange

	tt.write(&fontfile, uint16(cTablesRead))
	tt.write(&fontfile, uint16(searchRange))
	tt.write(&fontfile, uint16(entrySelector))
	tt.write(&fontfile, uint16(rangeShift))

	checksumAdjustmentOffset := 0
	for _, tbl := range newTables {
		tt.write(&fontfile, []byte(tbl.name))
		tt.write(&fontfile, tbl.checksum)
		tt.write(&fontfile, tbl.offset)
		tt.write(&fontfile, tbl.length)
		if tbl.name == "head" {
			checksumAdjustmentOffset = int(tbl.offset) + 8
		}
	}

	for _, tbl := range newTables {
		tt.write(&fontfile, tbl.tabledata)
	}

	b := fontfile.Bytes()
	checksumFontFile := calcChecksum(b)
	binary.BigEndian.PutUint32(b[checksumAdjustmentOffset:], checksumFontFile-0xB1B0AFBA)
	w.Write(b)

	return nil
}

// Subset removes all data from the font except the one needed for the given code points. Subset should be only called once.
func (tt *Font) Subset(codepoints []int) (string, error) {
	tt.SubsetID = getCharTag(codepoints)
	codepointsMap := make(map[int]struct{}, len(codepoints))
	for _, cp := range codepoints {
		codepointsMap[cp] = struct{}{}
	}

	additionalGlyphs := []int{}
	for _, cp := range codepoints {
		additionalGlyphs = append(additionalGlyphs, tt.getGlyphComponentIds(cp)...)
	}
	for _, cp := range additionalGlyphs {
		codepointsMap[cp] = struct{}{}
	}
	codepoints = codepoints[:0]
	for cp := range codepointsMap {
		codepoints = append(codepoints, cp)
	}

	sort.Ints(codepoints)
	// now the codepoints slice should have all codepoints necessary for the fonts.
	// The ones that are requested and those who require other glyphs of the font
	// for example: ö could be a compound glyph of o and dieresis
	// so the glyphs that are to be placed are ö, o and dieresis
	maxCP := codepoints[len(codepoints)-1] + 1

	glyphs := make([]Glyph, maxCP)
	emptyGlyph := Glyph{}
	for i, c := 0, 0; i < maxCP; i++ {
		if i == codepoints[c] {
			glyphs[i] = tt.Glyph[i]
			c++
		} else {
			tt.advanceWidth[i] = 0
			tt.lsb[i] = 0
			glyphs[i] = emptyGlyph
		}
	}
	tt.Glyph = glyphs
	tt.advanceWidth = tt.advanceWidth[:maxCP]
	tt.lsb = tt.lsb[:maxCP]
	tt.Maxp.NumGlyphs = uint16(maxCP)
	tt.Head.IndexToLocFormat = 1
	tt.Hhea.NumberOfHMetrics = uint16(maxCP)
	tt.subsetCodepoints = codepoints
	return "", nil
}

// Codepoints returns the codepoints for each rune
func (tt *Font) Codepoints(runes []rune) []int {
	ret := make([]int, 0, len(runes))
	for _, r := range runes {
		ret = append(ret, tt.ToCodepoint[r])
	}
	return ret
}

// CMap returns a CMap string to be used in a PDF file
func (tt *Font) CMap() string {
	var b strings.Builder
	b.WriteString(`/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CIDSystemInfo << /Registry (Adobe)/Ordering (UCS)/Supplement 0>> def
/CMapName /Adobe-Identity-UCS def /CMapType 2 def
1 begincodespacerange
`)
	fmt.Fprintf(&b, "<0001><%04X>\n", tt.Maxp.NumGlyphs)
	b.WriteString("endcodespacerange\n")
	fmt.Fprintf(&b, "%d beginbfchar\n", tt.Maxp.NumGlyphs)
	for _, cp := range tt.subsetCodepoints {
		fmt.Fprintf(&b, "<%04X><%04X>\n", cp, tt.ToUni[cp])
	}
	b.WriteString(`endbfchar
endcmap CMapName currentdict /CMap defineresource pop end end`)
	return b.String()
}

// Widths returns a widths string to be used in a PDF file
func (tt *Font) Widths() string {
	var b strings.Builder
	b.WriteString("[")
	for _, cp := range tt.subsetCodepoints {
		fmt.Fprintf(&b, "%d[%d]", cp, tt.advanceWidth[cp])
	}
	b.WriteString("]")
	return b.String()
}

// PDFName returns the font name with the subset id.
func (tt *Font) PDFName() string {
	return fmt.Sprintf("/%s-%s", tt.SubsetID, tt.names[6])
}

// Ascender returns the /Ascent value for the PDF file
func (tt *Font) Ascender() int {
	return int(tt.Hhea.Ascender)
}

// Descender returns the /Descent value for the PDF file
func (tt *Font) Descender() int {
	return int(tt.Hhea.Descender)
}

// CapHeight returns the /CapHeight value for the PDF file
func (tt *Font) CapHeight() int {
	ch := int(tt.OS2AdditionalFields.SCapHeight)
	return ch
}

// BoundingBox returns the /FontBBox value for the PDF file
func (tt *Font) BoundingBox() string {
	return fmt.Sprintf("[%d %d %d %d]", 0, tt.Hhea.Descender, 1000, tt.Hhea.Ascender)
}

// Flags returns the /Flags value for the PDF file
func (tt *Font) Flags() int {
	return 4
}

// ItalicAngle returns the /ItalicAngle value for the PDF file
func (tt *Font) ItalicAngle() int {
	return int(tt.Post.ItalicAngle)
}

// StemV returns the /StemV value for the PDF file
func (tt *Font) StemV() int {
	return 0
}

// XHeight returns the /XHeight value for the PDF file
func (tt *Font) XHeight() int {
	xh := int(tt.OS2AdditionalFields.SxHeight)
	return xh
}

func calcChecksum(data []byte) uint32 {
	sum := uint32(0)
	c := 0
	for c < len(data) {
		sum += uint32(data[c])<<3 + uint32(data[c+1])<<2 + uint32(data[c+2])<<1 + uint32(data[c+3])
		c += 4
	}
	return sum
}

// Return a string of length 6 based on the characters in runelist.
// All returned characters are in the range A-Z.
func getCharTag(codepoints []int) string {
	data := make([]byte, len(codepoints)*2)
	for i, r := range codepoints {
		data[i*2] = byte((r >> 8) & 0xff)
		data[i*2+1] = byte(r & 0xff)
	}

	sum := md5.Sum(data)
	ret := make([]rune, 6)
	for i := 0; i < 6; i++ {
		ret[i] = rune(sum[2*i]+sum[2*i+1])/26 + 'A'
	}
	return string(ret)
}
