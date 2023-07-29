package csshtml

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/speedata/css/scanner"
	"golang.org/x/exp/slog"
)

// tokenizeAndApplyImport converts a CSS string to a Tokenstream. Also read linked (@import) stylesheets.
func (c *CSS) tokenizeAndApplyImport(css string) (tokenstream, error) {
	var tokens tokenstream
	var err error
	tokens = tokenizeCSSString(css)
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
			toks, err := c.tokenizeCSSFile(importvalue.Value)
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

	return finalTokens, nil
}

// tokenizeCSSFile converts a CSS file into a Tokenstream and applies import
// statements.
func (c *CSS) tokenizeCSSFile(filename string) (tokenstream, error) {
	if filename == "" {
		return nil, fmt.Errorf("tokenizeCSSFile: no filename given")
	}
	var tokens tokenstream
	var err error
	dir, fn := filepath.Split(filename)
	c.PushDir(dir)
	loc, err := c.FindFile(fn)
	if err != nil {
		return nil, err
	}
	tokens, err = parseCSSBody(loc)
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
			toks, err := c.tokenizeCSSFile(importvalue.Value)
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
			fs, err := c.FindFile(tok.Value)
			if err != nil {
				return nil, err
			}
			tok.Value = fs
			finalTokens = append(finalTokens, tok)
		} else {
			finalTokens = append(finalTokens, tok)
		}
	}
	c.PopDir()
	return finalTokens, nil
}

func parseCSSBody(filename string) (tokenstream, error) {
	slog.Debug("parse CSS file", "filename", filename)
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var tokens tokenstream

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

// tokenizeCSSString converts a string into a Tokenstream.
func tokenizeCSSString(contents string) tokenstream {
	var tokens tokenstream

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
