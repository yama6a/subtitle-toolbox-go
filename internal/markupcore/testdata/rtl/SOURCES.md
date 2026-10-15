# Sources

| File | Source | License |
|:--- |:--- |:--- |
| `embed.json` | The rows "Arabic", "tags stay outside", "override stays out", "already fixed", "marks inside the text", "left to right", "Arabic digits only", "drawing", "tags only", "punctuation first", "space after it", "tag after it", "earlier fix" and "left-to-right first" come from `rightToLeftProvider` in tests/TextTransformsTest.php of yama6a/subtitle-toolbox-php at `c0adecd5375820d5313f1583fc7e67942d514600`. The other rows are written for this repository. Every `want` value is the result of `Markup::embedRightToLeft` of that commit. | MIT |
| `holds.json` | Written for this repository. Every `want` value is the result of `Markup::holdsRightToLeftEmbedding` of that commit. | MIT |
| `letters.json` | Written for this repository. | MIT |
| `embed_lf.json` | Written for this repository. `Markup::embedRightToLeft` drops the final LF of these lines (yama6a/subtitle-toolbox-php#1055), so each `want` value is the PHP result plus the LF after the closing tags. | MIT |
