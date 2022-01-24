[![Go reference documentation](https://img.shields.io/badge/doc-go%20reference-73FA79)](https://pkg.go.dev/github.com/speedata/boxesandglue)&nbsp;[![Fund the development](https://img.shields.io/badge/Sponsor-Fund%20development-yellow)](https://github.com/sponsors/speedata)

# Boxes and Glue

This is a PDF typesetting library/backend in the spirit of TeX's algorithms. TeX is a typesetting system which is well known for its superb output quality.

TeX packs each unit (glyph, image, heading, ...) in a rectangular box which can be packed into other rectangular boxes.
A variable length called “glue” can be between each of these rectangles.
This is why this repository is called “boxes and glue”.

## API

The API has two layers, a high level frontend and a low level backend. Each layer is useful when using the library. I suggest to start using the high level API and switch to the backend when you need more control over the typesetting output.


![bagstructure](https://user-images.githubusercontent.com/209434/150811091-1432ac91-ef3d-44be-9953-7556ce254874.png)

### Frontend

The frontend has high level methods to create a PDF document, load fonts, insert text which can be broken into lines and output objects at exact positions.

### Backend

The backend has the small building blocks that are used to create documents. These building blocks are called “nodes” which can be chained together in linked lists, a node list.

See the [architecture overview](https://github.com/speedata/boxesandglue/discussions/2) for a more detailled description.

## Status

This library is still under development.

## Contact

Patrick Gundlach, <gundlach@speedata.de><br>
[@speedata](https://twitter.com/speedata), [@boxesandglue](https://twitter.com/boxesandglue)

## Sample code

```go
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/node"
	"github.com/speedata/boxesandglue/document"
)

var (
	str = `In olden times when wishing still helped one, there lived a king whose daughters
	were all beautiful; and the youngest was so beautiful that the sun itself, which
	has seen so much, was astonished whenever it shone in her face.
	Close by the king's castle lay a great dark forest, and under an old lime-tree in the forest
	was a well, and when the day was very warm, the king's child went out into the
	forest and sat down by the side of the cool fountain; and when she was bored she
	took a golden ball, and threw it up on high and caught it; and this ball was her
	favorite plaything.`
)

func dothings() error {
	outfilename := "sample.pdf"
	w, err := os.Create(outfilename)
	if err != nil {
		return err
	}
	d := document.NewDocument(w)
	d.Title = "The frog king"
	d.Filename = outfilename
	if d.DefaultLanguage, err = d.GetLanguage("en"); err != nil {
		return err
	}

	// Load a font, define a font family, and add this font to the family.
	cpr := &document.FontSource{Source: "fonts/CrimsonPro-Regular.ttf"}
	ff := d.NewFontFamily("text")
	ff.AddMember(cpr, document.FontWeight400, document.FontStyleNormal)
	// Create a recursive data structure for typesetting and create nodes.
	te := &document.TypesettingElement{
		Settings: document.TypesettingSettings{
			document.SettingFontFamily: ff,
			document.SettingSize:       12 * bag.Factor,
		},
		Items: []interface{}{str},
	}
	var nl, tail node.Node
	if nl, tail, err = d.Mknodes(te); err != nil {
		return err
	}

	// Hyphenation is optional.
	d.Hyphenate(nl)
	node.AppendLineEndAfter(tail)

	// Break into lines
	settings := node.NewLinebreakSettings()
	settings.HSize = 130 * bag.Factor
	settings.LineHeight = 14 * bag.Factor
	vlist, _ := node.Linebreak(nl, settings)

	// output the text and finish the page and the PDF file
	d.OutputAt(bag.MustSp("1cm"), bag.MustSp("26cm"), vlist)
	d.CurrentPage.Shipout()
	if err = d.Finish(); err != nil {
		return err
	}

	w.Close()
	return nil
}

func main() {
	starttime := time.Now()
	err := dothings()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("finished in ", time.Now().Sub(starttime))
}
```


To get a PDF/UA (universal accessibility) document, insert the following lines before `d.OutputAt...`

```go
	d.RootStructureElement = &document.StructureElement{
		Role: "Document",
	}

	p := &document.StructureElement{
		Role:       "P",
		ActualText: strings.Join(strings.Fields(str), " "),
	}

	d.RootStructureElement.AddChild(p)

	vlist.Attibutes = node.H{
		"tag": p,
	}
	```
