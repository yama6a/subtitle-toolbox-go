# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/itt/real, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `avscript_testing.itt` | https://github.com/avscript-tv/avscript/blob/e65bc4e1623e2ccbea2bb9db1a488073f60a00f0/xml%20exchange/svegatoga_iTT_English.itt | avscript, MIT |
| `fcp_23976_styles.itt` | Written for this repository in the shape of the Final Cut Pro iTT export, with a top and a bottom region at 23.976 fps | MIT |
| `bom_2997_crlf.itt` | Written for this repository with a UTF-8 BOM, CRLF line endings, a `ttm:title` and times past one hour at 29.97 fps | MIT |
| `expected/*.json` | Generated for this repository. Expected parse result of the file with the same name, written by `tests/files/generate-read-goldens.php` | MIT |
| `expected/*.out.*`, `expected/*.to.*` | Written by the Go write golden tests with -update, reviewed by hand. Each file equals the PHP write golden of the same name at the pinned commit | MIT |
