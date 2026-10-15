# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/encoding, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

All files are written for this repository. Each one started as a UTF-8 file with LF line endings. `sed 's/$/\r/'` added CR LF line endings, and `iconv -f UTF-8 -t <encoding>` wrote the encoding.

| File | Source | License |
|:--- |:--- |:--- |
| `french-windows-1252.srt` | Written for this repository. Windows-1252, CR LF, no BOM. Holds `é`, `€` and `œ`, which use bytes 0x80 to 0x9F | MIT |
| `russian-windows-1251.srt` | Written for this repository. Windows-1251, CR LF, no BOM | MIT |
| `japanese-shift_jis.srt` | Written for this repository. Shift_JIS, CR LF, no BOM. `ソ` and `表` have 0x5C as their second byte | MIT |
| `korean-cp949.smi` | Written for this repository in the shape of Korean SAMI files: a `KRCC` class and `<P>` without end tags. CP949, CR LF, no BOM. `똠` exists in CP949 but not in EUC-KR | MIT |
| `notepad-utf-16le.vtt` | Written for this repository in the shape that Windows Notepad saves as "UTF-16 LE": the BOM `FF FE` and CR LF | MIT |
| `arabic-utf-8.srt` | Written for this repository. UTF-8, CR LF, no BOM. Same cues as `arabic-windows-1256.srt` | MIT |
| `arabic-windows-1256.srt` | Written for this repository. Windows-1256, CR LF, no BOM | MIT |
| `utf-8-declared-utf-16.ttml` | Written for this repository. UTF-8, LF, no BOM, with `encoding="utf-16"` in the XML declaration, as some tools write it | MIT |
| `utf-16le-bom.ttml` | Written for this repository. The same file as `utf-8-declared-utf-16.ttml`, saved as UTF-16 LE with the BOM `FF FE` | MIT |
| `youtube-utf-16le-bom.srv1` | Written for this repository in the shape of a YouTube srv1 transcript. UTF-16 LE with the BOM `FF FE` and `encoding="utf-16"` | MIT |
| `czech-windows-1250.srt` | Written for this repository. Windows-1250, CR LF, no BOM | MIT |
| `polish-iso-8859-2.srt` | Written for this repository. ISO-8859-2, CR LF, no BOM | MIT |
| `greek-windows-1253.srt` | Written for this repository. Windows-1253, CR LF, no BOM | MIT |
| `turkish-windows-1254.srt` | Written for this repository. Windows-1254, CR LF, no BOM | MIT |
| `russian-koi8-r.srt` | Written for this repository. KOI8-R, CR LF, no BOM | MIT |
| `thai-windows-874.srt` | Written for this repository. Windows-874, CR LF, no BOM. Holds `…`, which is 0x85 in Windows-874 and missing in TIS-620 | MIT |
| `arabic-iso-8859-6.srt` | Written for this repository. ISO-8859-6, CR LF, no BOM | MIT |
| `chinese-gbk.srt` | Written for this repository. GBK, CR LF, no BOM | MIT |
| `chinese-big5.srt` | Written for this repository. CP950, the Windows form of Big5, CR LF, no BOM | MIT |
| `japanese-euc-jp.srt` | Written for this repository. EUC-JP, CR LF, no BOM | MIT |
| `japanese-cp932.ass` | Written for this repository. CP932, the Windows form of Shift_JIS, CR LF, no BOM. Holds `\N` and `{\i1}`, and `表` and `ソ` with 0x5C as their second byte | MIT |
| `thai-tis-620.srt` | Written for this repository. TIS-620, CR LF, no BOM. Also decodes as CP932 without an error | MIT |
| `estonian-windows-1257.srt` | Written for this repository. Windows-1257, CR LF, no BOM. Also decodes as GBK without an error | MIT |
