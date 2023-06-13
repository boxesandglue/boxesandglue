package node

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"sort"
)

// Debug shows node list debug output
func Debug(n Node) {
	w := new(bytes.Buffer)
	enc := xml.NewEncoder(w)
	enc.Indent("", "    ")
	debugNode(n, enc)
	enc.Flush()
	w.WriteString("\n")
	w.WriteTo(os.Stdout)
}

// DebugToFile writes an XML file with the node list.
func DebugToFile(n Node, fn string) error {
	w, err := os.Create(fn)
	if err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "    ")
	debugNode(n, enc)
	enc.Flush()
	return w.Close()
}

type kv struct {
	key   string
	value any
}

func encodeAttributes(enc *xml.Encoder, start *xml.StartElement, attributes []kv, extraAttributes H) error {
	for _, attr := range attributes {
		start.Attr = append(start.Attr, xml.Attr{
			Name:  xml.Name{Local: attr.key},
			Value: fmt.Sprint(attr.value),
		})
	}
	keys := make([]string, len(extraAttributes))
	for k := range extraAttributes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		start.Attr = append(start.Attr, xml.Attr{
			Name:  xml.Name{Local: k},
			Value: fmt.Sprint(fmt.Sprintf("%v", extraAttributes[k])),
		})
	}
	return enc.EncodeToken(*start)
}

func debugNode(n Node, enc *xml.Encoder) {
	for e := n; e != nil; e = e.Next() {
		start := xml.StartElement{}
		start.Name = xml.Name{Local: e.Name()}
		var err error
		switch v := e.(type) {
		case *VList:
			err = encodeAttributes(enc, &start, []kv{
				{"id", v.ID},
				{"wd", v.Width},
				{"ht", v.Height},
				{"dp", v.Depth},
			}, v.Attributes)
			debugNode(v.List, enc)
		case *HList:
			err = encodeAttributes(enc, &start, []kv{
				{"id", v.ID},
				{"wd", v.Width},
				{"ht", v.Height},
				{"dp", v.Depth},
				{"r", v.GlueSet},
			}, v.Attributes)
			debugNode(v.List, enc)
		case *Disc:
			err = encodeAttributes(enc, &start, []kv{
				{"id", v.ID},
			}, v.Attributes)
		case *Glyph:
			var fontid int
			if fnt := v.Font; fnt != nil {
				fontid = fnt.Face.FaceID
			}
			err = encodeAttributes(enc, &start, []kv{
				{"id", v.ID},
				{"components", v.Components},
				{"wd", v.Width},
				{"ht", v.Height},
				{"dp", v.Depth},
				{"codepoint", v.Codepoint},
				{"face", fontid},
			}, v.Attributes)
		case *Glue:
			err = encodeAttributes(enc, &start, []kv{
				{"id", v.ID},
				{"wd", v.Width},
				{"stretch", v.Stretch},
				{"stretchorder", v.StretchOrder},
				{"shrink", v.Shrink},
				{"shrinkorder", v.ShrinkOrder},
				{"subtype", v.Subtype},
			}, v.Attributes)
		case *Image:
			var filename string
			if v.Img != nil && v.Img.ImageFile != nil {
				filename = v.Img.ImageFile.Filename
			} else {
				filename = "(image object not set)"
			}
			err = encodeAttributes(enc, &start, []kv{
				{"id", v.ID},
				{"filename", filename},
			}, v.Attributes)
		case *Kern:
			err = encodeAttributes(enc, &start, []kv{
				{"id", v.ID},
				{"kern", v.Kern},
			}, v.Attributes)
		case *Lang:
			var langname string
			if v.Lang != nil {
				langname = v.Lang.Name
			} else {
				langname = "-"
			}
			err = encodeAttributes(enc, &start, []kv{
				{"id", v.ID},
				{"lang", langname},
			}, v.Attributes)
		case *Penalty:
			err = encodeAttributes(enc, &start, []kv{
				{"id", v.ID},
				{"penalty", v.Penalty},
				{"width", v.Width},
			}, v.Attributes)
		case *Rule:
			err = encodeAttributes(enc, &start, []kv{
				{"id", v.ID},
				{"wd", v.Width},
				{"ht", v.Height},
				{"dp", v.Depth},
			}, v.Attributes)
		case *StartStop:
			startNode := "-"
			if v.StartNode != nil {
				startNode = fmt.Sprintf("%d", v.StartNode.ID)
			}
			err = encodeAttributes(enc, &start, []kv{
				{"id", v.ID},
				{"action", v.Action},
				{"start", startNode},
			}, v.Attributes)
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
