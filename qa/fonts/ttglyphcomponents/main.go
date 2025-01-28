package main

import (
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/frontend"
)

// var str = `A wonderful serenity has taken possession of my entire soul,
//  	  like these sweet mornings of spring which I enjoy with my whole heart.
//  	  I am alone, and feel the charm of existence in this spot, which was created
// 	  for the bliss of souls like mine.`

var str = `“Hello, world”`

func typesetSample() error {
	f, err := frontend.New("result.pdf")
	if err != nil {
		return err
	}
	if f.Doc.DefaultLanguage, err = frontend.GetLanguage("en"); err != nil {
		return err
	}
	f.Doc.Title = "A test document"
	f.Doc.DefaultPageHeight = bag.MustSP("5cm")
	f.Doc.DefaultPageWidth = bag.MustSP("5cm")
	p := f.Doc.NewPage()
	onecm := bag.MustSP("1cm")
	ff := f.NewFontFamily("text")
	ff.AddMember(
		&frontend.FontSource{Location: filepath.Join("fonts", "ArugulaLAB20241206-Regular.ttf")},
		frontend.FontWeight400,
		frontend.FontStyleNormal,
	)
	para := frontend.NewText()
	para.Items = []any{str}

	vlist, _, err := f.FormatParagraph(para, onecm,
		frontend.Leading(bag.MustSP("14pt")),
		frontend.FontSize(bag.MustSP("12pt")),
		frontend.Family(ff),
	)
	if err != nil {
		return err
	}
	p.OutputAt(onecm, 4*onecm, vlist)

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
