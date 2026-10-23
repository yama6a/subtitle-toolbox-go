# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/sami/real, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `mantas_smi.smi` | Written for this repository in the shape of https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/tests/files/smi.smi | MIT |
| `mantas_smi_formatted.smi` | Written for this repository in the shape of https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/tests/files/smi_formatted.smi | MIT |
| `multi_language.smi` | Written for this repository. UTF-8 with BOM, CR LF line endings, Korean, English and French classes | MIT |
| `pysubs2_source_id.smi` | Written for this repository in the shape of `SAMI_INPUT1` in https://github.com/tkarabela/pysubs2/blob/ae3c520a8aa13130f36d84df02ce15a843bc9bdb/tests/formats/test_sami.py | MIT |
| `subsrt_sample.smi` | Written for this repository in the shape of https://github.com/papnkukn/subsrt/blob/478e54390b74859b3643b80ff988113557ff80d8/test/fixtures/sample.smi | MIT |
| `undefined_class.smi` | Written for this repository in the shape of the SAMI example in issue #355. Its `<P>` classes `ENCC` and `FRCC` are not in the STYLE block | MIT |
| `negative_start_bare_nbsp.smi` | Written for this repository in the shape of the SAMI example in issue #397. It has a negative `Start` and `&nbsp` without a semicolon | MIT |
| `unused_first_class.smi` | Written for this repository in the shape of the SAMI example in issue #508. The STYLE block defines `FRFRCC` before `ENUSCC`, and every `<P>` uses `ENUSCC` | MIT |
| `own_windows_media.smi` | Written for this repository in the shape of Microsoft SAMI 1.0 for Windows Media Player: upper-case tags, `<SAMIParam>` with `Media`, `Metrics`, `Spec` and `Length`, `<STYLE TYPE="text/css"><!-- ... --></STYLE>`, a `#Source` speaker paragraph, `&nbsp;` clear lines, CR LF | MIT |
| `own_korean_cp949.smi` | Written for this repository in the shape of a Korean SMI from a broadcast download site: CP949 bytes, the `KRCC` class, `<SYNC Start=...><P Class=KRCC>` without closing tags, CR LF. It needs `ReadOptions::$encoding` | MIT |
| `own_subtitle_edit_modern.smi` | Written for this repository in the shape of the "SAMI modern" export of Subtitle Edit, `ToText` in https://github.com/SubtitleEdit/subtitleedit/blob/2dc7ebda900489e2dea80c36220bcc2b86205aac/src/libse/SubtitleFormats/Sami.cs: quoted attributes, `</P></SYNC>` after each paragraph, `<br />`, the 2 `<-- ... -->` lines after `<BODY>`, CR LF, no final line end | MIT |
| `expected/*.json` | Generated for this repository. Expected parse result of the file with the same name, written by `tests/files/generate-read-goldens.php`. `own_korean_cp949.smi` is read as CP949 | MIT |
| `expected/*.out.*`, `expected/*.to.*` | Written by the Go write golden tests with -update, reviewed by hand | MIT |
