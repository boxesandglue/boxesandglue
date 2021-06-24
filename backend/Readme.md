# Boxes and Glue

This is a repository for experiments with TeX's algorithms. It might serve as a typesetting backend.

TeX has each unit (glyph, image, heading, ...) in a rectangular box which can be packed into other rectangular boxes. A variable length called “glue” can be between each of these rectangles. This is why this repository is called “boxes and glue”.

Within this repository you will find functions to create and manipulate these boxes.
The smallest unit is a `Node` which can be chained together in linked lists, a `Nodelist`.

There are several types of nodes:

* Glyphs contain one or more visual entities such as the character `H` or a ligature `ﬁ`.
* Vertical lists point to a node list of vertically arranged elements (typically lines in a paragraph).
* Horizontal lists of items arranged next to each other.
* Glue nodes are spaces with a fixed width which can stretch or shrink.
* Discretionary nodes contain information about hyphenation points
* Language nodes contain information about the language to be used for hyphenation

## Status

This repository is not usable for any serious purpose yet.

## Contact

Patrick Gundlach, <gundlach@speedata.de>

