package node

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"sort"
)

// Debug shows node list debug output.
func Debug(n Node) {
	w := new(bytes.Buffer)
	enc := xml.NewEncoder(w)
	enc.Indent("", "    ")
	debugNode(n, enc)
	enc.Flush()
	w.WriteString("\n")
	w.WriteTo(os.Stdout)
}

// DebugToString returns node list debug output.
func DebugToString(n Node) string {
	w := new(bytes.Buffer)
	enc := xml.NewEncoder(w)
	enc.Indent("", "    ")
	debugNode(n, enc)
	enc.Flush()
	w.WriteString("\n")
	return w.String()
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
	value any
	key   string
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
			Value: fmt.Sprintf("%v", extraAttributes[k]),
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
				{key: "id", value: v.ID},
				{key: "wd", value: v.Width},
				{key: "ht", value: v.Height},
				{key: "dp", value: v.Depth},
			}, v.Attributes)
			debugNode(v.List, enc)
		case *HList:
			err = encodeAttributes(enc, &start, []kv{
				{key: "id", value: v.ID},
				{key: "wd", value: v.Width},
				{key: "ht", value: v.Height},
				{key: "dp", value: v.Depth},
				{key: "r", value: v.GlueSet},
			}, v.Attributes)
			debugNode(v.List, enc)
		case *Disc:
			err = encodeAttributes(enc, &start, []kv{
				{key: "id", value: v.ID},
			}, v.Attributes)
		case *Glyph:
			var fontid int
			if fnt := v.Font; fnt != nil {
				fontid = fnt.Face.FaceID
			}
			err = encodeAttributes(enc, &start, []kv{
				{key: "id", value: v.ID},
				{key: "components", value: v.Components},
				{key: "wd", value: v.Width},
				{key: "ht", value: v.Height},
				{key: "dp", value: v.Depth},
				{key: "codepoint", value: v.Codepoint},
				{key: "face", value: fontid},
			}, v.Attributes)
		case *Glue:
			err = encodeAttributes(enc, &start, []kv{
				{key: "id", value: v.ID},
				{key: "wd", value: v.Width},
				{key: "stretch", value: v.Stretch},
				{key: "stretchorder", value: v.StretchOrder},
				{key: "shrink", value: v.Shrink},
				{key: "shrinkorder", value: v.ShrinkOrder},
				{key: "subtype", value: v.Subtype},
			}, v.Attributes)
		case *Image:
			var filename string
			if v.ImageFile != nil {
				filename = v.ImageFile.Filename
			} else {
				filename = "(image object not set)"
			}
			err = encodeAttributes(enc, &start, []kv{
				{key: "id", value: v.ID},
				{key: "filename", value: filename},
				{key: "wd", value: v.Width},
				{key: "ht", value: v.Height},
			}, v.Attributes)
		case *Kern:
			err = encodeAttributes(enc, &start, []kv{
				{key: "id", value: v.ID},
				{key: "kern", value: v.Kern},
			}, v.Attributes)
		case *Lang:
			var langname string
			if v.Lang != nil {
				langname = v.Lang.Name
			} else {
				langname = "-"
			}
			err = encodeAttributes(enc, &start, []kv{
				{key: "id", value: v.ID},
				{key: "lang", value: langname},
			}, v.Attributes)
		case *Penalty:
			err = encodeAttributes(enc, &start, []kv{
				{key: "id", value: v.ID},
				{key: "penalty", value: v.Penalty},
				{key: "width", value: v.Width},
			}, v.Attributes)
		case *Rule:
			err = encodeAttributes(enc, &start, []kv{
				{key: "id", value: v.ID},
				{key: "wd", value: v.Width},
				{key: "ht", value: v.Height},
				{key: "dp", value: v.Depth},
			}, v.Attributes)
		case *StartStop:
			startNode := "-"
			if v.StartNode != nil {
				startNode = fmt.Sprintf("%d", v.StartNode.ID)
			}
			err = encodeAttributes(enc, &start, []kv{
				{key: "id", value: v.ID},
				{key: "action", value: v.Action},
				{key: "start", value: startNode},
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
