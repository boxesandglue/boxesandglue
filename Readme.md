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

This repository is not usable for any serious purpose yet. It is used for experiments for a successor of LuaTeX as a backend for the [speedata Publisher](https://github.com/speedata/publisher/).

## Contact

Patrick Gundlach, <gundlach@speedata.de>

## Sample code

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/node"
	"github.com/speedata/boxesandglue/document"
)

func dothings() error {
	outfilename := "sample.pdf"
	w, err := os.Create(outfilename)
	if err != nil {
		return err
	}
	d := document.NewDocument(w)
	d.Filename = outfilename
	face, err := d.LoadFace("fonts/CrimsonPro-Regular.ttf", 0)
	if err != nil {
		return err
	}
	font := d.CreateFont(face, 10*bag.Factor)
	indent := node.NewGlue()
	indent.Width = 18 * bag.Factor

	var head, cur node.Node
	head = indent
	cur = head

	str := `In olden times when wishing still helped one, there lived a king whose daughters
	were all beautiful; and the youngest was so beautiful that the sun itself, which
	has seen so much, was astonished whenever it shone in her face.
	Close by the king's castle lay a great dark forest, and under an old lime-tree in the forest
	was a well, and when the day was very warm, the king's child went out into the
	forest and sat down by the side of the cool fountain; and when she was bored she
	took a golden ball, and threw it up on high and caught it; and this ball was her
	favorite plaything.`

	var lastglue node.Node
	for _, r := range font.Shape(str) {
		if r.Glyph == 32 {
			if lastglue == nil {
				g := node.NewGlue()
				g.Width = font.Space
				g.Stretch = font.SpaceStretch
				g.Shrink = font.SpaceShrink
				node.InsertAfter(head, cur, g)
				cur = g
				lastglue = g
			}
		} else {
			n := node.NewGlyph()
			n.Hyphenate = r.Hyphenate
			n.Codepoint = r.Codepoint
			n.Components = r.Components
			n.Font = font
			n.Width = r.Advance
			node.InsertAfter(head, cur, n)
			cur = n
			lastglue = nil
		}
	}
	if lastglue != nil && lastglue.Prev() != nil {
		p := lastglue.Prev()
		p.SetNext(nil)
		lastglue.SetPrev(nil)
	}

	node.AppendLineEndAfter(cur)
	settings := node.NewLinebreakSettings()
	settings.HSize = 200 * bag.Factor
	settings.LineHeight = 12 * bag.Factor
	vlist, info := node.Linebreak(head, settings)
	// information about the line breaks, not strictly necessary:
	for _, line := range info {
		fmt.Println(line.Line, line.Demerits, line.R)
	}

	d.OutputAt(bag.MustSp("1cm"), bag.MustSp("26cm"), vlist)

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
