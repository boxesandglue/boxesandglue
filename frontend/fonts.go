package frontend

import (
	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/fonts/camingocodebold"
	"github.com/speedata/boxesandglue/fonts/camingocodebolditalic"
	"github.com/speedata/boxesandglue/fonts/camingocodeitalic"
	"github.com/speedata/boxesandglue/fonts/camingocoderegular"
	"github.com/speedata/boxesandglue/fonts/crimsonprobold"
	"github.com/speedata/boxesandglue/fonts/crimsonprobolditalic"
	"github.com/speedata/boxesandglue/fonts/crimsonproitalic"
	"github.com/speedata/boxesandglue/fonts/crimsonproregular"
	"github.com/speedata/boxesandglue/fonts/texgyreherosbold"
	"github.com/speedata/boxesandglue/fonts/texgyreherosbolditalic"
	"github.com/speedata/boxesandglue/fonts/texgyreherositalic"
	"github.com/speedata/boxesandglue/fonts/texgyreherosregular"
)

var (
	tenpoint    = bag.MustSp("10pt")
	twelvepoint = bag.MustSp("12pt")
)

// LoadIncludedFonts creates the font families monospace, sans and serif for
// default fonts.
func (fe *Document) LoadIncludedFonts() error {
	var err error
	monospace := fe.NewFontFamily("monospace")
	if err = monospace.AddMember(&FontSource{Data: camingocoderegular.TTF, Name: "CamingoCode Regular"}, 400, FontStyleNormal); err != nil {
		return err
	}
	if err = monospace.AddMember(&FontSource{Data: camingocodebold.TTF, Name: "CamingoCode Bold"}, 700, FontStyleNormal); err != nil {
		return err
	}
	if err = monospace.AddMember(&FontSource{Data: camingocodeitalic.TTF, Name: "CamingoCode Italic"}, 400, FontStyleItalic); err != nil {
		return err
	}
	if err = monospace.AddMember(&FontSource{Data: camingocodebolditalic.TTF, Name: "CamingoCode Bold Italic"}, 700, FontStyleItalic); err != nil {
		return err
	}

	sans := fe.NewFontFamily("sans")
	if err = sans.AddMember(&FontSource{Data: texgyreherosregular.TTF, Name: "TeXGyreHeros Regular"}, 400, FontStyleNormal); err != nil {
		return err
	}
	if err = sans.AddMember(&FontSource{Data: texgyreherosbold.TTF, Name: "TeXGyreHeros Bold"}, 700, FontStyleNormal); err != nil {
		return err
	}
	if err = sans.AddMember(&FontSource{Data: texgyreherositalic.TTF, Name: "TeXGyreHeros Italic"}, 400, FontStyleItalic); err != nil {
		return err
	}
	if err = sans.AddMember(&FontSource{Data: texgyreherosbolditalic.TTF, Name: "TeXGyreHeros BoldItalic"}, 700, FontStyleItalic); err != nil {
		return err
	}
	serif := fe.NewFontFamily("serif")
	if err = serif.AddMember(&FontSource{Data: crimsonproregular.TTF, Name: "CrimsonPro Regular"}, 400, FontStyleNormal); err != nil {
		return err
	}
	if err = serif.AddMember(&FontSource{Data: crimsonprobold.TTF, Name: "CrimsonPro Bold"}, 700, FontStyleNormal); err != nil {
		return err
	}
	if err = serif.AddMember(&FontSource{Data: crimsonproitalic.TTF, Name: "CrimsonPro Italic"}, 400, FontStyleItalic); err != nil {
		return err
	}
	if err = serif.AddMember(&FontSource{Data: crimsonprobolditalic.TTF, Name: "CrimsonPro BoldItalic"}, 700, FontStyleItalic); err != nil {
		return err
	}
	return nil
}
