# Contributing to boxes and glue

Thank you for taking the time to send a patch. Boxes and glue is
developed in a private mono repository together with htmlbag, glu and
XTS, and mirrored to this one. That is why pull requests are squash
merged and the commit that lands here does not carry your commit
history. Your name stays in the squashed commit.

## Before you start

Whether a change works is only half the question. The other half is
whether boxes and glue should carry it for years: htmlbag, glu, XTS and
other programs build on its exported API and on its output, and whatever
it offers has to be maintained. So some changes are welcome as a pull
request right away, and some need an agreement first.

**Send a pull request directly for:**

- **A bug fix.** Boxes and glue crashes, typesets or writes something
  wrong (compared to what TeX, the PDF specification or the font
  specifications say), or gives a different result from run to run or
  from platform to platform.
- **A faster or leaner implementation** that keeps the API and the
  output as they are.
- **Documentation and tests.**

**Open an issue first, and wait for an answer before you write the code,
for:**

- new exported API: types, fields, functions, methods, settings,
- a change of existing API or of the output of existing documents that
  is not a bug fix,
- a new typesetting feature (in line breaking, page and table building,
  fonts or the PDF output),
- a change that needs pull requests in more than one repository
  (boxes and glue, htmlbag, glu, XTS).

Describe the problem in the issue, not only the solution: what you want
to typeset, why the existing API cannot do it, and what you propose. It
is much cheaper to agree on the shape of an API before anyone
implements it. "No" and "not now" are possible answers, and they say
nothing about the quality of the idea.

The library is still under development and its API may change, but
changes to it are made on purpose, not along the way.

**Keep pull requests small**, one change each.

## What a change needs

1. **A test** that fails without the change, next to the code it tests.
   Where TeX has an answer (line breaking, glue setting, badness), the
   test and the code follow it; name the section of *TeX: The Program*
   in a comment when it helps.

2. **Doc comments** on every exported identifier you add or change.

3. **A word on the output.** If documents that do not use your change
   come out differently afterwards, say so in the pull request and why.
   The PDFs of glu, XTS and bagme are compared byte for byte before a
   release, so an unexplained change will be found, but it is easier to
   review when it is expected. When you touch fonts, run the programs
   under `qa/fonts` (`go run .` in each directory) and compare the
   `result.pdf` they write with the `reference.pdf` next to it.

## Building and testing

```
go vet ./...
go test ./...
```

Format the code with `gofmt`.

## Releases

Merged changes are collected and released together, not one release per
pull request.

## Reporting bugs

Open an issue with a minimal Go program (or a document for glu, XTS or
htmlbag) that shows the problem, the version you use, and what you
expected to see. A PDF or screenshot of the wrong output helps.
