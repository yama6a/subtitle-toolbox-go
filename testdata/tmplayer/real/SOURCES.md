# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/tmplayer/real, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

Every file is written for this repository. No permissively licensed real TMPlayer file was found.

| File | Source | License |
|:--- |:--- |:--- |
| `tmplayer_crlf.txt` | Written for this repository in the shape of the TMPlayer files that https://github.com/SubtitleEdit/subtitleedit/blob/ffefc2f410ba584a9a34dd87e9d2ccb5a8939781/src/libse/SubtitleFormats/TMPlayer.cs reads: `hh:mm:ss:`, empty lines that end a cue, CR LF | MIT |
| `tmplayer_plus.txt` | Written for this repository in the shape of TMPlayer+ files: `h:mm:ss=`, LF | MIT |
| `tmplayer_multiline_bom.txt` | Written for this repository in the shape of TMPlayer+ files with line numbers: `hh:mm:ss,1=` and `hh:mm:ss,2=`, UTF-8 BOM, CR LF | MIT |
| `tmplayer_out_of_order.txt` | Written for this repository in the shape of `tmplayer_crlf.txt`, with the line at 7 s before the line at 4 s, CR LF | MIT |
| `expected/*.json` | Generated for this repository. Expected parse result of the file with the same name, written by `tests/files/generate-read-goldens.php` | MIT |
| `expected/*.out.*`, `expected/*.to.*` | Written by the Go write golden tests with -update, reviewed by hand | MIT |
