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
		start := xml.StartElement{Name: xml.Name{Local: e.Name()}}
		attrs, extra := e.DebugAttributes()
		if err := encodeAttributes(enc, &start, attrs, extra); err != nil {
			panic(err)
		}
		switch v := e.(type) {
		case *HList:
			debugNode(v.List, enc)
		case *VList:
			debugNode(v.List, enc)
		}
		if err := enc.EncodeToken(start.End()); err != nil {
			panic(err)
		}
	}
}
