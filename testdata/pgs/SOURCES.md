# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/pgs, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

Real `.sup` files hold bitmaps of film dialogue, so all files here are written for that repository. The `shapes` files show simple shapes. `text_1080p.sup` shows text that is written for that repository.

| File | Source | License |
|:--- |:--- |:--- |
| `shapes_1080p.sup` | Written for this repository. 1920x1080, six cues: one object, two objects in two windows, a forced object split over several object segments, a palette update, a cropped object with an acquisition point and a segment of unknown type, and a last cue without a clearing display set | MIT |
| `shapes_576p.sup` | Written for this repository. 720x576, two cues, for the BT.601 colour matrix | MIT |
| `text_1080p.sup` | Written for this repository. 1920x1080, 12 cues of 1 or 2 lines in Liberation Sans, 44 to 60 px, with a black outline and 16 levels of anti-aliasing. Yellow and italic text, and one cue at the top. The font is SIL OFL 1.1 and is used only as drawn pixels | MIT |
| `text_cyrillic_1080p.sup` | Written for this repository. The cues of `text_1080p.sup` in Russian, drawn the same way | MIT |
| `authoring_layout_1080p.sup` | Written for this repository in the segment layout of a Blu-ray authoring tool: every display set is an epoch start (composition state `0x80`), also the one that clears the screen, and every palette defines all 256 entries. 1920x1080, three cues: a line of blocks, a bar that uses each of the 256 entries, and the text `Gate 12 closes at 9:40.` drawn as in `text_1080p.sup` | MIT |
| `fade_layout_1080p.sup` | Written for this repository in the segment layout of BDSup2Sub and SUPer: two objects in one composition, windows larger than the objects, and a fade out of 3 display sets in the normal state that only update the palette. 1920x1080, two cues of two lines of blocks | MIT |
| `expected/*.sup.json` | The pixel golden of each `.sup` file: the times, position, size, forced flag, alignment and the SHA-256 of the RGBA bytes of each cue. Written by the PHP library | MIT |
| `expected/*.sup.png` | The PNG of the first cue of each `.sup` file, for a reviewer to look at | MIT |
