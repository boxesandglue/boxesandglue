# Boxes and Glue

This is a repository for experiments with TeX's algorithms. It might serve as a typesetting backend.

TeX has each unit (glyph, image, heading, ...) in a rectangular box which can be packed into other rectangular boxes. A variable length called “glue” can be between each of these rectangles. This is why this repository is called “boxes and glue”.

Within this repository you will find functions to create and manipulate these boxes.
The smallest unit is a `Node` which can be chained together in linked lists, a `Nodelist`.

There are several types of nodes:

* Glyphs contain one or more visual entities such as the character `H` or a ligature `ﬁ`.
* Vertical lists point to a node list of vertically arranged elements (typically lines in a paragraph).
* Horizontal lists of items arranged next to each other.
* Glue nodes are spaces with a fixed width which can stretch or shrink.
* Discretionary nodes contain information about hyphenation points
* Language nodes contain information about the language to be used for hyphenation

## Status

This repository is not usable for any serious purpose yet.

## Contact

Patrick Gundlach, <gundlach@speedata.de>

## Sample code

```go
package main

import (
	"log"
	"os"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/node"
	"github.com/speedata/boxesandglue/document"
)

func dothings() error {
	bag.SetLogLevel(bag.TraceLevel)
	bag.LogDebug("main/setup")

	w, err := os.Create("sample.pdf")
	if err != nil {
		return err
	}
	d := document.NewDocument(w)
	l, err := d.LoadPatternFile("hyphenationpatterns/hyph-en-us.pat.txt")
	if err != nil {
		return err
	}
	l.Name = "en-US"

	face, err := d.LoadFace("fonts/rws.otf", 0)
	if err != nil {
		return err
	}
	font := d.CreateFont(face, 10*bag.Factor)

	nodelist := node.NewNodelist()
	ln := node.NewLangWithContents(&node.Lang{Lang: l})
	nodelist.AppendNode(ln)

	var str string
	str = "A wonderful serenity has taken possession of my entire soul. "

	var lastglue *node.Node
	for _, r := range font.Shape(str) {
		if r.Glyph == 32 {
			if lastglue == nil {
				g := node.NewGlue()
				g.Width = font.Space
				lastglue = nodelist.AppendNode(g)
			}
		} else {
			n := node.NewGlyph()
			n.Hyphenate = r.Hyphenate
			n.Codepoint = r.Codepoint
			n.Components = r.Components
			n.Face = face
			n.Width = r.Advance
			nodelist.AppendNode(n)
			lastglue = nil
		}
	}
	if lastglue != nil {
		nodelist.Remove(lastglue)
	}

	settings := node.LinebreakSettings{
		HSize:      200 * bag.Factor,
		LineHeight: 12 * bag.Factor,
	}
	vlist := node.Linebreak(nodelist, settings)

	d.OutputAt(bag.MustSp("4cm"), bag.MustSp("20cm"), vlist)
	d.CurrentPage.Shipout()

	if err = d.Finish(); err != nil {
		return err
	}
	w.Close()
	return nil
}

func main() {
	err := dothings()
	if err != nil {
		log.Fatal(err)
	}
}
```
