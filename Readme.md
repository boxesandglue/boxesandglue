[![Twitter URL](https://img.shields.io/twitter/url?url=https%3A%2F%2Ftwitter.com%2Fboxesandglue)](https://twitter.com/intent/tweet?text=Wow:&url=https%3A%2F%2Ftwitter.com%2Fboxesandglue)&nbsp;[![Go reference documentation](https://img.shields.io/badge/doc-go%20reference-73FA79)](https://pkg.go.dev/github.com/speedata/boxesandglue)&nbsp;[![Fund the development](https://img.shields.io/badge/Sponsor-Fund%20development-yellow)](https://github.com/sponsors/speedata)&nbsp;[![Homepage](https://img.shields.io/badge/homepage-boxesandglue.dev-blue)](https://boxesandglue.dev)


# Boxes and Glue

This is a PDF typesetting library/backend in the spirit of TeX's algorithms. TeX is a typesetting system which is well known for its superb output quality.

TeX packs each unit (glyph, image, heading, ...) in a rectangular box which can be packed into other rectangular boxes.
A variable length called “glue” can be between each of these rectangles.
This is why this repository is called “boxes and glue”.

## Features

* High speed (“Ludicrous mode”). A simple document is created within 10ms on a macBook. This includes loading an OpenType font, typesetting text and writing the PDF file.
* High output quality. Boxes and glue uses TeX's line breaking algorithm to create the optimal line breaks.
* Extensibility: See the API section below. Boxes and glue is split into a high level frontend and a low level backend to provide the API you need.
* OpenType features and font shaping with harfbuzz. Harfbuzz is well known for its awesome language support.

## API

The API has two layers, a high level frontend and a low level backend. Each layer is useful when using the library. I suggest to start using the high level API and switch to the backend when you need more control over the typesetting output.


![bagstructure](https://user-images.githubusercontent.com/209434/150811091-1432ac91-ef3d-44be-9953-7556ce254874.png)

### Frontend

The frontend has high level methods to create a PDF document, load fonts, insert text which can be broken into lines and output objects at exact positions.

### Backend

The backend has the small building blocks that are used to create documents. These building blocks are called “nodes” which can be chained together in linked lists, a node list.

See the [architecture overview](https://github.com/speedata/boxesandglue/discussions/2) for a more detailed description.

## Status

This library is still under development. Expect API changes.

## Contact

Patrick Gundlach, <gundlach@speedata.de><br>
[@speedata](https://twitter.com/speedata), [@boxesandglue](https://twitter.com/boxesandglue)

## Sample code

```go
package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/frontend"
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

func typesetSample() error {
	f, err := frontend.New("sample.pdf")
	if err != nil {
		return err
	}

	f.Doc.Title = "The frog king"

	if f.Doc.DefaultLanguage, err = frontend.GetLanguage("en"); err != nil {
		return err
	}

	// Load a font, define a font family, and add this font to the family.
	ff := f.NewFontFamily("text")
	ff.AddMember(
		&frontend.FontSource{Source: "fonts/CrimsonPro-Regular.ttf"},
		frontend.FontWeight400,
		frontend.FontStyleNormal,
	)

	// Create a recursive data structure for typesetting initialized with the
	// text from the top (but with space normalized).
	para := frontend.NewText()
	para.Items = []any{strings.Join(strings.Fields(str), " ")}

	// Format the text into a paragraph. Some of these settings (font family and
	// font size) can be part of the typesetting element.
	vlist, _, err := f.FormatParagraph(para, bag.MustSp("125pt"),
		frontend.Leading(bag.MustSp("14pt")),
		frontend.FontSize(bag.MustSp("12pt")),
		frontend.Family(ff),
	)
	if err != nil {
		return err
	}

	// Output the text and finish the page and the PDF file.
	p := f.Doc.NewPage()
	p.OutputAt(bag.MustSp("1cm"), bag.MustSp("26cm"), vlist)
	p.Shipout()
	if err = f.Doc.Finish(); err != nil {
		return err
	}
	return nil
}

func main() {
	starttime := time.Now()
	err := typesetSample()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("finished in ", time.Now().Sub(starttime))
}
```


To get a PDF/UA (universal accessibility) document, insert the following lines before `.OutputAt...` and add `"github.com/speedata/boxesandglue/backend/document"` to the import section.

```go
	f.Doc.RootStructureElement = &document.StructureElement{
		Role: "Document",
	}

	para := &document.StructureElement{
		Role:       "P",
		ActualText: str,
	}
	f.Doc.RootStructureElement.AddChild(para)

	vlist.Attributes = node.H{
		"tag": para,
	}
```


The result is

<img src="https://i.imgur.com/cwGQTzQ.png" alt="typeset text from the frog king" width="200"/>

