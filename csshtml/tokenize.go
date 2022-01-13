package csshtml

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/speedata/css/scanner"
)

func (c *CSS) ParseCSSString(css string) (Tokenstream, error) {
	var tokens Tokenstream
	var err error
	c.dirstack = append(c.dirstack, "")
	tokens = parseCSSString(css)
	if err != nil {
		return nil, err
	}

	var finalTokens []*scanner.Token

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if tok.Type == scanner.AtKeyword && tok.Value == "import" {
			i++
			for {
				if tokens[i].Type == scanner.S {
					i++
				} else {
					break
				}
			}
			importvalue := tokens[i]
			toks, err := c.ParseCSSFile(importvalue.Value)
			if err != nil {
				return nil, err
			}
			// if the last token of the imported file is a space, remove it.
			lasttoc := toks[len(toks)-1]
			if lasttoc.Type == scanner.S {
				finalTokens = append(toks[:len(toks)-1], finalTokens...)
			} else {
				finalTokens = append(toks, finalTokens...)
			}
			// hopefully there is no keyword before the semicolon
			for {
				i++
				if i >= len(tokens) {
					break
				}
				if tokens[i].Value == ";" {
					break
				}
			}
		} else {
			finalTokens = append(finalTokens, tok)
		}
	}
	c.dirstack = c.dirstack[:len(c.dirstack)-1]
	return finalTokens, nil
}

func (c *CSS) ParseCSSFile(filename string) (Tokenstream, error) {
	if filename == "" {
		return nil, fmt.Errorf("parseCSSFile: no filename given")
	}
	var tokens Tokenstream
	var err error
	dir, fn := filepath.Split(filename)
	c.dirstack = append(c.dirstack, dir)
	dirs := strings.Join(c.dirstack, "")
	tokens, err = parseCSSBody(filepath.Join(dirs, fn))
	if err != nil {
		return nil, err
	}

	var finalTokens []*scanner.Token

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if tok.Type == scanner.AtKeyword && tok.Value == "import" {
			i++
			for {
				if tokens[i].Type == scanner.S {
					i++
				} else {
					break
				}
			}
			importvalue := tokens[i]
			toks, err := c.ParseCSSFile(importvalue.Value)
			if err != nil {
				return nil, err
			}
			// if the last token of the imported file is a space, remove it.
			lasttoc := toks[len(toks)-1]
			if lasttoc.Type == scanner.S {
				finalTokens = append(toks[:len(toks)-1], finalTokens...)
			} else {
				finalTokens = append(toks, finalTokens...)
			}
			// hopefully there is no keyword before the semicolon
			for {
				i++
				if i >= len(tokens) {
					break
				}
				if tokens[i].Value == ";" {
					break
				}
			}
		} else {
			finalTokens = append(finalTokens, tok)
		}
	}
	c.dirstack = c.dirstack[:len(c.dirstack)-1]
	return finalTokens, nil
}

func parseCSSBody(filename string) (Tokenstream, error) {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var tokens Tokenstream

	s := scanner.New(string(b))
	for {
		token := s.Next()
		if token.Type == scanner.EOF || token.Type == scanner.Error {
			break
		}
		switch token.Type {
		case scanner.Comment:
		default:
			tokens = append(tokens, token)
		}
	}
	return tokens, nil
}

func parseCSSString(contents string) Tokenstream {
	var tokens Tokenstream

	s := scanner.New(contents)
	for {
		token := s.Next()
		if token.Type == scanner.EOF || token.Type == scanner.Error {
			break
		}
		switch token.Type {
		case scanner.Comment:
			// ignore
		default:
			tokens = append(tokens, token)
		}
	}
	return tokens
}
