package node

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
)

// Debug shows node list debug output
func Debug(n Node) {
	w := new(bytes.Buffer)
	enc := xml.NewEncoder(w)
	enc.Indent("", "    ")
	debugNode(n, enc, 0)
	enc.Flush()
	w.WriteTo(os.Stdout)
}

type kv struct {
	key   string
	value any
}

func encodeAttributes(enc *xml.Encoder, start *xml.StartElement, attributes []kv) {
	for _, attr := range attributes {
		start.Attr = append(start.Attr, xml.Attr{
			Name:  xml.Name{Local: attr.key},
			Value: fmt.Sprint(attr.value),
		})
	}
}

func debugNode(n Node, enc *xml.Encoder, level int) {
	for e := n; e != nil; e = e.Next() {
		start := xml.StartElement{}
		start.Name = xml.Name{Local: e.Name()}
		var err error
		switch v := e.(type) {
		case *VList:
			attr := []kv{
				{"id", v.ID},
				{"wd", v.Width},
				{"ht", v.Height},
				{"dp", v.Depth},
			}
			for k, v := range v.Attributes {
				attr = append(attr, kv{k, v})
			}
			encodeAttributes(enc, &start, attr)
			err = enc.EncodeToken(start)
			debugNode(v.List, enc, level+1)
		case *HList:
			attr := []kv{
				{"id", v.ID},
				{"wd", v.Width},
				{"ht", v.Height},
				{"dp", v.Depth},
				{"r", v.GlueSet},
			}
			for k, v := range v.Attributes {
				attr = append(attr, kv{k, v})
			}

			encodeAttributes(enc, &start, attr)
			err = enc.EncodeToken(start)
			debugNode(v.List, enc, level+1)
		case *Disc:
			encodeAttributes(enc, &start, []kv{
				{"id", v.ID},
			})
			err = enc.EncodeToken(start)
		case *Glyph:
			var fontid int
			if fnt := v.Font; fnt != nil {
				fontid = fnt.Face.FaceID
			}
			encodeAttributes(enc, &start, []kv{
				{"id", v.ID},
				{"components", v.Components},
				{"wd", v.Width},
				{"ht", v.Height},
				{"dp", v.Depth},
				{"codepoint", v.Codepoint},
				{"face", fontid},
			})
			err = enc.EncodeToken(start)
		case *Glue:
			attr := []kv{
				{"id", v.ID},
				{"wd", v.Width},
				{"stretch", v.Stretch},
				{"stretchorder", v.StretchOrder},
				{"shrink", v.Shrink},
				{"shrinkorder", v.ShrinkOrder},
				{"subtype", v.Subtype},
			}
			for k, v := range v.Attributes {
				attr = append(attr, kv{k, v})
			}
			encodeAttributes(enc, &start, attr)
			err = enc.EncodeToken(start)
		case *Image:
			var filename string
			if v.Img != nil && v.Img.ImageFile != nil {
				filename = v.Img.ImageFile.Filename
			} else {
				filename = "(image object not set)"
			}
			encodeAttributes(enc, &start, []kv{
				{"id", v.ID},
				{"filename", filename},
			})
			err = enc.EncodeToken(start)
		case *Kern:
			encodeAttributes(enc, &start, []kv{
				{"id", v.ID},
				{"kern", v.Kern},
			})
			err = enc.EncodeToken(start)
		case *Lang:
			var langname string
			if v.Lang != nil {
				langname = v.Lang.Name
			} else {
				langname = "-"
			}
			encodeAttributes(enc, &start, []kv{
				{"id", v.ID},
				{"lang", langname},
			})
			err = enc.EncodeToken(start)
		case *Penalty:
			encodeAttributes(enc, &start, []kv{
				{"id", v.ID},
				{"penalty", v.Penalty},
				{"width", v.Width},
			})
			err = enc.EncodeToken(start)
		case *Rule:
			encodeAttributes(enc, &start, []kv{
				{"id", v.ID},
				{"wd", v.Width},
				{"ht", v.Height},
				{"dp", v.Depth},
			})
			err = enc.EncodeToken(start)
		case *StartStop:
			encodeAttributes(enc, &start, []kv{
				{"id", v.ID},
			})
			err = enc.EncodeToken(start)
		default:
			err = enc.EncodeToken(start)
			panic("unhandled token")
		}
		if err != nil {
			panic(err)
		}
		err = enc.EncodeToken(start.End())
		if err != nil {
			panic(err)
		}
	}
}
