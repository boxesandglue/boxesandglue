# Math engine test fonts

The font-gated integration tests in this package need an OpenType math font
present at `testdata/latinmodern-math.otf`. The default choice is
**Latin Modern Math** by Bogusław Jackowski, Janusz M. Nowacki & Piotr Strzelczyk
— it is the de-facto reference OT-MATH font and ships with most TeX
distributions.

The font is not checked into the repository (it is ~720 KB of OpenType
binary; the project ships only the source). It is also excluded by
`.gitignore` so an accidental commit is prevented.

## Where to get it

- **CTAN tarball** (recommended): <https://ctan.org/pkg/lm-math>
  Download the latest `lm-math.zip`, unpack, and copy
  `otf/latinmodern-math.otf` to this directory.
- **TeX Live**: a copy already lives at
  `texmf-dist/fonts/opentype/public/lm-math/latinmodern-math.otf` — a symlink
  works.
- **GUST website**: <https://www.gust.org.pl/projects/e-foundry/lm-math>

The license is the GUST Font License (a free-software license compatible
with the LaTeX Project Public License).

## What happens without the font

The integration tests (`TestSimpleOrdOrd`, `TestSubscriptShiftDown`,
`TestFractionDisplayCascade`, …) call `t.Skip` and report

    math font not available at testdata/latinmodern-math.otf — see testdata/README.md

The pure-logic tests (`TestInterAtomSpace_*`, `TestRewriteBinToOrd_*`,
`TestClassOf_*`) do not need any font and always run.

## Adding a second math font

To exercise the engine against, e.g., STIX Two Math or XITS Math, drop the
`.otf` next to `latinmodern-math.otf` and add a test that calls a
modified `loadMathFont` helper with the appropriate file name. The engine
is font-agnostic — every metric comes from the MATH table — so any
spec-conformant font should produce green tests.
