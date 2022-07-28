package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
)

var filemapping = map[string]string{
	"bg":    "bg",
	"ca":    "ca",
	"cs":    "cs",
	"cy":    "cy",
	"da":    "da",
	"de":    "de-1996",
	"el":    "el-monoton",
	"en":    "en-gb",
	"en_gb": "en-gb",
	"en_us": "en-us",
	"eo":    "eo",
	"es":    "es",
	"et":    "et",
	"eu":    "eu",
	"fi":    "fi",
	"fr":    "fr",
	"ga":    "ga",
	"gl":    "gl",
	"grc":   "grc",
	"gu":    "gu",
	"hi":    "hi",
	"hr":    "hr",
	"hu":    "hu",
	"hy":    "hy",
	"id":    "id",
	"is":    "is",
	"it":    "it",
	"ku":    "kmr",
	"kn":    "kn",
	"lt":    "lt",
	"ml":    "ml",
	"lv":    "lv",
	"nb":    "nb",
	"nl":    "nl",
	"nn":    "nn",
	"no":    "nb",
	"pl":    "pl",
	"pt":    "pt",
	"ro":    "ro",
	"ru":    "ru",
	"sk":    "sk",
	"sl":    "sl",
	"sc":    "sr-cyrl",
	"sv":    "sv",
	"tr":    "tr",
	"uk":    "uk",
}

func langNameToVarName(lang string) string {
	ret := []string{}
	for _, ln := range strings.Split(lang, "_") {
		ret = append(ret, ln)
	}
	return strings.Join(ret, "")
}

// Download hyphenation patterns and put them into a map in the file
// document/hyphenationpatterns.go.
func createPatterns() error {
	languagesSorted := make([]string, 0, len(filemapping))
	for k := range filemapping {
		languagesSorted = append(languagesSorted, k)
	}

	sort.Strings(languagesSorted)

	out, err := os.Create("frontend/hyphenationpatterns.go")
	if err != nil {
		return err
	}

	fmt.Fprintln(out, "// Generated from go generate. Do not edit.")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "package frontend")
	fmt.Fprintln(out, "")

	fmt.Fprintln(out, "var hyphenationpatterns = map[string]string{")

	for _, langname := range languagesSorted {
		filename := filemapping[langname]
		url := "https://ftp.gwdg.de/pub/ctan/language/hyph-utf8/tex/generic/hyph-utf8/patterns/txt/hyph-" + filename + ".pat.txt"
		fmt.Println("Download from", url)
		resp, err := http.Get(url)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			fmt.Println(resp.Status)
		}
		defer resp.Body.Close()

		fmt.Fprintf(out, "\t%q: ", langNameToVarName(langname))
		fmt.Fprintln(out, "`")
		_, err = io.Copy(out, resp.Body)
		if err != nil {
			return err
		}
		fmt.Fprintln(out, "`,")
	}
	fmt.Fprintln(out, "}")
	return out.Close()
}
