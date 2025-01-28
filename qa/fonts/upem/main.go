package main

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
	"github.com/boxesandglue/boxesandglue/frontend"
)

var str = `A wonderful serenity has taken possession of my entire soul,
 	  like these sweet mornings of spring which I enjoy with my whole heart.
 	  I am alone, and feel the charm of existence in this spot, which was created
	  for the bliss of souls like mine.`

var fonts = []string{
	"ArugulaLAB20241206-Regular.ttf",
	"ArugulaLAB20231005-Regular.otf",
	"CrimsonPro-Regular2048.ttf",
	"CrimsonPro-Regular.ttf",
	"texgyreheros-regular.otf",
}

func typesetSample() error {
	f, err := frontend.New("result.pdf")
	if err != nil {
		return err
	}
	f.Doc.DefaultLanguage, err = frontend.GetLanguage("en")
	if err != nil {
		return err
	}
	f.Doc.Title = "The Sorrows of Young Werther"

	p := f.Doc.NewPage()

	if f.Doc.DefaultLanguage, err = frontend.GetLanguage("en"); err != nil {
		return err
	}
	onecm := bag.MustSP("1cm")
	paperheight := bag.MustSP("297mm")
	sep := onecm / 2
	textwd := bag.MustSP("125pt")
	textht := bag.MustSP(("6cm")) //approx

	locX := []bag.ScaledPoint{sep}
	locY := []bag.ScaledPoint{paperheight - sep}
	curx := locX[0]
	for {
		curx += textwd + sep
		if curx+textwd > 21*onecm {
			break
		}
		locX = append(locX, curx)
	}

	cury := locY[0]
	for {
		cury -= textht + sep
		if cury < textht+onecm {
			break
		}
		locY = append(locY, cury)
	}

	for i, fnt := range fonts {
		ff := f.NewFontFamily("text")
		ff.AddMember(
			&frontend.FontSource{Location: filepath.Join("fonts", fnt)},
			frontend.FontWeight400,
			frontend.FontStyleNormal,
		)
		para := frontend.NewText()
		para.Items = []any{strings.Join(strings.Fields(str), " ")}

		vlist, _, err := f.FormatParagraph(para, textwd,
			frontend.Leading(bag.MustSP("14pt")),
			frontend.FontSize(bag.MustSP("12pt")),
			frontend.Family(ff),
		)
		if err != nil {
			return err
		}
		x := i % len(locX)
		y := i / len(locX)
		p.OutputAt(locX[x], locY[y], vlist)
	}
	rulewidth := bag.MustSP("0.5pt")
	for _, x := range locX {
		r := node.NewRule()
		r.Height = -paperheight
		r.Width = rulewidth
		vl := node.Vpack(r)
		p.OutputAt(x, 0, vl)
		p.OutputAt(x+textwd, 0, vl)

	}
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
