package csshtml

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ReadHTMLWithStyles opens an HTML file and read linked stylesheets.
func (c *CSS) ReadHTMLWithStyles(filename string) (*goquery.Document, error) {
	dir, fn := filepath.Split(filename)
	c.PushDir(dir)

	filename, err := c.FindFile(fn)
	if err != nil {
		return nil, err
	}

	r, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, err
	}
	var errcond error
	doc.Find(":root > head link").Each(func(i int, sel *goquery.Selection) {
		if stylesheetfile, attExists := sel.Attr("href"); attExists {
			block, err := c.tokenizeCSSFile(stylesheetfile)
			if err != nil {
				errcond = err
			}
			parsedStyles := consumeBlock(block, false)
			c.Stylesheet = append(c.Stylesheet, parsedStyles)
		}
	})
	if errcond != nil {
		return nil, errcond
	}
	_, err = c.ApplyCSS(doc)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

// ParseHTMLFragmentWithCSS takes the HTML text and the CSS text and returns
// goquery selection.
func (c *CSS) ParseHTMLFragmentWithCSS(htmltext, csstext string) (*goquery.Document, error) {
	var err error
	if err = c.AddCSSText(csstext); err != nil {
		return nil, err
	}
	doc, err := c.ReadHTMLChunk(htmltext)
	if err != nil {
		return nil, err
	}
	return c.ApplyCSS(doc)
}

// ParseHTMLFragment takes the HTML text and the CSS text and returns goquery selection.
func (c *CSS) ParseHTMLFragment(htmltext string) (*goquery.Document, error) {
	doc, err := c.ReadHTMLChunk(htmltext)
	if err != nil {
		return nil, err
	}
	return c.ApplyCSS(doc)
}

// ReadHTMLChunk reads the HTML text. If there are linked style sheets (<link
// href=...) these are also read. After reading the HTML and CSS the HTML is
// stored in c.document.
func (c *CSS) ReadHTMLChunk(htmltext string) (*goquery.Document, error) {
	var err error
	r := strings.NewReader(htmltext)
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, err
	}
	var errcond error
	doc.Find(":root > head link").Each(func(i int, sel *goquery.Selection) {
		if stylesheetfile, attExists := sel.Attr("href"); attExists {
			block, err := c.tokenizeCSSFile(stylesheetfile)
			if err != nil {
				errcond = err
			}
			parsedStyles := consumeBlock(block, false)
			c.processAtRules(parsedStyles)
			c.Stylesheet = append(c.Stylesheet, parsedStyles)
		}
	})
	return doc, errcond
}

// AddCSSText parses CSS text and appends the rules to the previously
// read rules.
func (c *CSS) AddCSSText(fragment string) error {
	toks, err := c.tokenizeAndApplyImport(fragment)
	if err != nil {
		return err
	}
	block := consumeBlock(toks, false)
	c.processAtRules(block)
	c.Stylesheet = append(c.Stylesheet, block)
	return nil
}
