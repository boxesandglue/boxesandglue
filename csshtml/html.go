package csshtml

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// OpenHTMLFile opens an HTML file
func (c *CSS) OpenHTMLFile(filename string) (*goquery.Document, error) {
	dir, fn := filepath.Split(filename)
	c.Dirstack = append(c.Dirstack, dir)
	dirs := strings.Join(c.Dirstack, "")
	r, err := os.Open(filepath.Join(dirs, fn))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	c.document, err = goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, err
	}
	var errcond error
	c.document.Find(":root > head link").Each(func(i int, sel *goquery.Selection) {
		if stylesheetfile, attExists := sel.Attr("href"); attExists {
			block, err := c.ParseCSSFile(stylesheetfile)
			if err != nil {
				errcond = err
			}
			parsedStyles := ConsumeBlock(block, false)
			c.Stylesheet = append(c.Stylesheet, parsedStyles)
		}
	})
	if errcond != nil {
		return nil, errcond
	}
	c.processAtRules()
	_, err = c.ApplyCSS()
	if err != nil {
		return nil, err
	}
	return c.document, nil
}

// ParseHTMLFragment takes the HTML text and the CSS text and returns goquery selection.
func (c *CSS) ParseHTMLFragment(htmltext, csstext string) (*goquery.Selection, error) {
	c.Stylesheet = append(c.Stylesheet, ConsumeBlock(ParseCSSString(CSSdefaults), false))
	c.Stylesheet = append(c.Stylesheet, ConsumeBlock(ParseCSSString(csstext), false))
	err := c.ReadHTMLChunk(htmltext)
	if err != nil {
		return nil, err
	}
	return c.ApplyCSS()
}

// ReadHTMLChunk reads the HTML text. If there are linked style sheets (<link
// href=...) these are also read. After reading the HTML and CSS the
func (c *CSS) ReadHTMLChunk(htmltext string) error {
	var err error
	r := strings.NewReader(htmltext)
	c.document, err = goquery.NewDocumentFromReader(r)
	if err != nil {
		return err
	}
	var errcond error
	c.document.Find(":root > head link").Each(func(i int, sel *goquery.Selection) {
		if stylesheetfile, attExists := sel.Attr("href"); attExists {
			block, err := c.ParseCSSFile(stylesheetfile)
			if err != nil {
				errcond = err
			}
			parsedStyles := ConsumeBlock(block, false)
			c.Stylesheet = append(c.Stylesheet, parsedStyles)
		}
	})
	return errcond
}

// AddCSSText parses CSS text and saves the rules for later.
func (c *CSS) AddCSSText(fragment string) error {
	toks, err := c.ParseCSSString(fragment)
	if err != nil {
		return err
	}
	c.Stylesheet = append(c.Stylesheet, ConsumeBlock(toks, false))
	return nil
}
