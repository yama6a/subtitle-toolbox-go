# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/fixing, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

All files here are written for this repository. They hold no third-party text.

| File | Source | License |
|:--- |:--- |:--- |
| `ocr-en.ocr.srt`, `ocr-de.ocr.srt`, `ocr-fr.ocr.srt`, `ocr-es.ocr.srt` | The text that `generate.php` reads from the `.sup` file of the same name, OCR errors included | MIT |
| `text_1080p.ocr.srt` | The text that `generate.php` reads from `../pgs/text_1080p.sup`, OCR errors included | MIT |
| `web-errors.srt` | Written for this repository in the shape of a downloaded SubRip file: UTF-8 BOM, CR LF, `<i>`, `<b>` and `<font color>` tags. 12 cues with spacing, punctuation, dash and tag errors | MIT |
| `web-errors.fixed.srt`, `ocr-*.fixed.srt` | `CommonErrorFixer` output for the file of the same name, with the default options and its language | MIT |
| `text_1080p.fixed.srt` | `CommonErrorFixer` output for `text_1080p.ocr.srt`, language `en` | MIT |
| `optional-rules.srt` | Written for this repository: 4 cues with the errors of the `CommonErrorFixer` rules that are off by default | MIT |
| `optional-rules.fixed.srt` | `CommonErrorFixer` output for `optional-rules.srt`, language `en`, with every rule that is off by default turned on | MIT |
| `ad-cues.srt` | Written for this repository in the shape of a downloaded SubRip file: 9 cues with site ads in English, Spanish and German, and credits, web and e-mail addresses in dialogue | MIT |
| `ad-cues.fixed.srt` | `stbox convert ad-cues.srt --to srt --errors-fix --language en --errors-enable adCues` output | MIT |
| `ad-cues-test-set/*.srt` | Written for this repository: 50 SubRip files, 5 each in English, Spanish, German, French, Portuguese, Arabic, Hebrew, Chinese, Russian and Turkish. 1,760 cues. Ad and credit cues follow the shape of real download site cues in our own words. Dialogue holds web and e-mail addresses, credit phrases, repeated lines and dots that look like domains | MIT |
| `ad-cues-test-set/labels.json` | The kind of each cue of the files in `ad-cues-test-set/`, for example `ad-url` or `dlg-url-edge` | MIT |
| `latin1_unclosed_italic.srt` | Written for this repository in Windows-1252: one cue with an `<i>` tag without its end tag | MIT |
| `rtl-dialogue.srt` | Written for this repository in the shape of a Hebrew and Arabic SubRip file after `fixRightToLeft()`: no BOM, LF, U+202B and U+202C around each line, `<i>` tags, dialogue dashes without a space, a minus sign at a line start and a line with U+200F before the dash | MIT |
| `rtl-dialogue.fixed.srt` | `CommonErrorFixer` output for `rtl-dialogue.srt`, language `he` | MIT |
| `cjk-dialogue.fixed.srt` | `CommonErrorFixer` output for `../fixes/own_cjk_dialogue.srt`, language `zh`, with `dialogueOnOneLine` | MIT |
| `rule-rows.json` | The rows of the `ruleCases` provider of `tests/Fixing/CommonErrorFixerTest.php` in this repository whose text is not ASCII, as the name, the rule, the language and the lines before and after | MIT |
