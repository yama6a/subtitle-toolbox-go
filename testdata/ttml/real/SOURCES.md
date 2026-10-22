# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/ttml/real, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `astisub_breaklines.ttml` | https://github.com/asticode/go-astisub/blob/7671e72e9d47287d6f8ded80a004560b63a2ead5/testdata/example-in-breaklines.ttml | go-astisub, MIT |
| `astisub_merging_style.ttml` | https://github.com/asticode/go-astisub/blob/7671e72e9d47287d6f8ded80a004560b63a2ead5/testdata/example-in-merging-style.ttml | go-astisub, MIT |
| `astisub_smpte.ttml` | Written for this repository in the shape of https://github.com/asticode/go-astisub/blob/7671e72e9d47287d6f8ded80a004560b63a2ead5/testdata/example-in.ttml | MIT |
| `blank_lines_before_xml.ttml` | Written for this repository. Two empty lines and an indent come before the XML declaration, as in files that an editor saved | MIT |
| `bbc_ebu_tt_d.ttml` | Written for this repository in the shape of the EBU-TT-D files in https://www.bbc.co.uk/accessibility/forproducts/guides/subtitles/ | MIT |
| `flash_ttaf1_2006_04.xml` | Written for this repository in the shape of the Timed Text files of the Adobe FLVPlaybackCaptioning component, https://help.adobe.com/en_US/as3/components/WS5b3ccc516d4fbf351e63e3d118a9c65b32-7ee5.html | MIT |
| `mantas_dfxp_br.dfxp` | https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/tests/files/dfxp_with_different_br.dfxp | mantas-done/subtitles, MIT |
| `mantas_duplicated_ids.ttml` | https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/tests/files/ttml_with_duplicated_element_ids.ttml | mantas-done/subtitles, MIT |
| `mantas_fps_multiplier.ttml` | https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/tests/files/ttml_with_fps_and_multiplier_given.ttml | mantas-done/subtitles, MIT |
| `mantas_netflix_ticks.dfxp` | Written for this repository in the shape of https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/tests/files/dfxp.dfxp | MIT |
| `mantas_ttml2.ttml` | https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/tests/files/ttml2.ttml | mantas-done/subtitles, MIT |
| `pysubs2_regions.ttml` | Written for this repository in the shape of https://github.com/tkarabela/pysubs2/blob/ae3c520a8aa13130f36d84df02ce15a843bc9bdb/tests/data/ttml_example2.ttml | MIT |
| `w3c_dfxp_timing.dfxp` | https://www.w3.org/TR/2006/CR-ttaf1-dfxp-20061116/, section 9.3.3, elaborated example | W3C Software and Document License, https://www.w3.org/copyright/software-license-2023/ |
| `w3c_imsc11_frames.ttml` | https://www.w3.org/TR/ttml-imsc1.1/, annex I.4, SMPTE-TT example | W3C Software and Document License, https://www.w3.org/copyright/software-license-2023/ |
| `w3c_imsc11_line_gaps.ttml` | https://www.w3.org/TR/ttml-imsc1.1/, section 7.8.6, first itts:fillLineGap example | W3C Software and Document License, https://www.w3.org/copyright/software-license-2023/ |
| `w3c_imsc11_paragraphs.ttml` | https://www.w3.org/TR/ttml-imsc1.1/, section 7.8.6, second itts:fillLineGap example | W3C Software and Document License, https://www.w3.org/copyright/software-license-2023/ |
| `w3c_imsc11_regions.ttml` | https://www.w3.org/TR/ttml-imsc1.1/, section 7.8.5, ittp:activeArea example | W3C Software and Document License, https://www.w3.org/copyright/software-license-2023/ |
| `w3c_ttml1_cells.ttml` | https://www.w3.org/TR/ttml1/, annex O.2, roll-up caption example | W3C Software and Document License, https://www.w3.org/copyright/software-license-2023/ |
| `w3c_ttml1_timed_spans.ttml` | https://www.w3.org/TR/ttml1/, annex O.3, paint-on caption example | W3C Software and Document License, https://www.w3.org/copyright/software-license-2023/ |
| `w3c_ttml1_timing.ttml` | https://www.w3.org/TR/ttml1/, section 9.3.5, elaborated example | W3C Software and Document License, https://www.w3.org/copyright/software-license-2023/ |
| `smpte_drop_ntsc.ttml` | Written for this repository with SMPTE drop-frame times at 29.97 fps across minute 1, minute 10 and one hour | MIT |
| `style_inheritance.ttml` | Written for this repository with styles on a region, `<body>`, `<div>` and `<p>`, and spans that reset them | MIT |
| `word_timed_spans.ttml` | Written for this repository with word times on `<span>` elements, with and without times on the `<p>` | MIT |
| `own_netflix_dfxp.dfxp` | Written for this repository in the shape of a Netflix DFXP delivery as the Netflix Timed Text Style Guide describes it: `ttp:tickRate="10000000"`, tick times such as `20020000t`, a top and a bottom region, italics through a style, the `dfxp-ls-sdh` profile | MIT |
| `own_youtube_ttml.ttml` | Written for this repository in the shape of the YouTube timed text download with `fmt=ttml`: the XML on one line, the SDP-US profile, clock times, `<br />` line breaks | MIT |
| `own_ttconv_imsc11.ttml` | Output of ttconv 1.1.1 for `tt convert -i srt/real/own_styled.srt -o own_ttconv_imsc11.ttml`, unchanged: IMSC 1.1 text on one line without an XML declaration, nested `<span>` styles, an empty `<span />` | MIT |
| `ttconv_referential_styling.ttml` | https://github.com/sandflow/ttconv/blob/1fed6abf555ca1d5ae2c839ad3c1625f15c9a96c/src/test/resources/ttml/referential_styling.ttml, with `begin` and `end` added to each `<p>`. Chains of `style` references on styles, regions and `<div>` | sandflow/ttconv, BSD-2-Clause |
| `ttconv_lwsp_default.ttml` | https://github.com/sandflow/ttconv/blob/1fed6abf555ca1d5ae2c839ad3c1625f15c9a96c/src/test/resources/ttml/lwsp_default.ttml, with `begin` and `end` added to each `<p>`. White space in the default `xml:space` mode | sandflow/ttconv, BSD-2-Clause |
| `ttconv_lwsp_preserve.ttml` | https://github.com/sandflow/ttconv/blob/1fed6abf555ca1d5ae2c839ad3c1625f15c9a96c/src/test/resources/ttml/lwsp_preserve.ttml, with `begin` and `end` added to the `<p>`. White space in `xml:space="preserve"` | sandflow/ttconv, BSD-2-Clause |
| `expected/*.json` | Generated for this repository. Expected parse result of the file with the same name, written by `tests/files/generate-read-goldens.php` | MIT |
| `expected/*.to.*` | Written by the Go write golden tests with -update, reviewed by hand. Each file equals the PHP write golden of the same name at the pinned commit. | MIT |
