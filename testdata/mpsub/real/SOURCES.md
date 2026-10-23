# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/mpsub/real, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

No real MPSub file with a license that allows copying into this MIT repository was found. The files below are written for this repository. They follow the format notes in MPlayer's sample file `DOCS/tech/mpsub.sub`, but hold new text. MPlayer is GPL and nothing of it is copied.

| File | Source | License |
|:--- |:--- |:--- |
| `mplayer_doc_time.sub` | Written from the format notes in https://github.com/pigoz/mplayer-svn/blob/7f6453553628729b151490144f13eae290a0fd67/DOCS/tech/mpsub.sub | MIT, part of this repository |
| `mplayer_doc_frames.sub` | Written from the same format notes, with `FORMAT=25` and fractional frame counts | MIT, part of this repository |
| `expected/*.json` | Generated for this repository. Expected parse result of the file with the same name, written by `tests/files/generate-read-goldens.php` | MIT |
| `expected/*.out.*`, `expected/*.to.*` | Written by the Go write golden tests with -update, reviewed by hand | MIT |
