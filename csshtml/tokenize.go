package csshtml

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/speedata/css/scanner"
)

// ParseCSSString converts a CSS string to a Tokenstream
func (c *CSS) ParseCSSString(css string) (Tokenstream, error) {
	var tokens Tokenstream
	var err error
	c.Dirstack = append(c.Dirstack, "")
	tokens = ParseCSSString(css)
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
	c.Dirstack = c.Dirstack[:len(c.Dirstack)-1]
	return finalTokens, nil
}

// ParseCSSFile converts a CSS file into a Tokenstream
func (c *CSS) ParseCSSFile(filename string) (Tokenstream, error) {
	if filename == "" {
		return nil, fmt.Errorf("parseCSSFile: no filename given")
	}
	var tokens Tokenstream
	var err error
	dir, fn := filepath.Split(filename)
	c.Dirstack = append(c.Dirstack, dir)
	dirs := filepath.Join(c.Dirstack...)
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
		} else if tok.Type == scanner.URI {
			var loc string
			if strings.HasPrefix(tok.Value, "http") {
				loc = tok.Value
			} else {
				joinedStack := filepath.Join(c.Dirstack...)
				loc = filepath.Join(joinedStack, tok.Value)
			}
			tok.Value = loc

			finalTokens = append(finalTokens, tok)
		} else {
			finalTokens = append(finalTokens, tok)
		}
	}
	c.Dirstack = c.Dirstack[:len(c.Dirstack)-1]
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
			// ignore
		case scanner.S:
			if len(tokens) > 0 && tokens[len(tokens)-1].Type == scanner.S {
				// ignore
			} else {
				tokens = append(tokens, token)
			}

		default:
			tokens = append(tokens, token)
		}
	}
	return tokens, nil
}

// ParseCSSString converts a string into a Tokenstream
func ParseCSSString(contents string) Tokenstream {
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
		case scanner.S:
			if len(tokens) > 0 && tokens[len(tokens)-1].Type == scanner.S {
				// ignore
			} else {
				tokens = append(tokens, token)
			}
		default:
			tokens = append(tokens, token)
		}
	}
	return tokens
}
