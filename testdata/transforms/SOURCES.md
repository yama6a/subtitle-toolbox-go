# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/transforms, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `own_cea608_caps.vtt` | Written for this repository in the shape of WebVTT that a CEA-608 caption converter emits: all upper case, `>>` speaker changes, unescaped `&`, cue settings, `Kind` and `Language` headers, `NOTE` comments and a sound cue in brackets. | MIT |
| `own_cea608_caps_sentence.vtt` | Written for this repository. Expected `WebVttFormatter` output for `own_cea608_caps.vtt` after `changeCase("sentence")`. | MIT |
| `own_cea608_caps_cleaned.vtt` | Written for this repository. Expected `WebVttFormatter` output for `own_cea608_caps.vtt` after the bracket and dot replacements and `stripFormatting()`. | MIT |
| `own_cea608_run_on.vtt` | Written for this repository in the shape of `own_cea608_caps.vtt`. Sentences that run across cues, the English pronoun `I` with contractions, a gap of 2 s, and sentence ends before a closing quote, a closing bracket and `…`. | MIT |
| `own_cea608_run_on_sentence.vtt` | Written for this repository. Expected `WebVttFormatter` output for `own_cea608_run_on.vtt` after `changeCase("sentence")`. | MIT |
| `own_cea608_sound_cues.vtt` | Written for this repository in the shape of `own_cea608_caps.vtt`. Sound cues in brackets and parentheses, music cues with `♪`, a speaker label cue `JOHN:`, and italic tags around some of them. | MIT |
| `own_cea608_sound_cues_sentence.vtt` | Written for this repository. Expected `WebVttFormatter` output for `own_cea608_sound_cues.vtt` after `changeCase("sentence")`. | MIT |
| `own_multilingual_caps.srt` | Written for this repository. SubRip with UTF-8 BOM and CR LF line endings. Upper case Greek, German and Turkish text, bold, italic and colour tags, and `<` and `&` in the text. | MIT |
| `own_rtl.srt` | Written for this repository. SubRip with Arabic and Hebrew lines, italic and bold tags, a `{\an8}` override, an ASS drawing and a left-to-right line. | MIT |
| `own_rtl_fixed.srt` | Written for this repository. Expected `SubRipFormatter` output for `own_rtl.srt` after `fixRightToLeft()`. | MIT |
| `own_cea608_caps_stripped.vtt` | Generated for this repository. Output of `subtitle-toolbox convert own_cea608_caps.vtt --to vtt --strip-tags`, checked by hand | MIT |
| `own_multilingual_caps_upper.srt` | Generated for this repository. Output of `subtitle-toolbox convert own_multilingual_caps.srt --to srt --case upper`, checked by hand: the text was already upper case | MIT |
| `own_multilingual_caps_lower_tr.srt` | Generated for this repository. Output of `subtitle-toolbox convert own_multilingual_caps.srt --to srt --case lower --language tr`, checked by hand: the Turkish rules turn every `I` into `ı`, and the Greek word ends with `ς` | MIT |
| `own_multilingual_caps_replaced.srt` | Generated for this repository. Output of `subtitle-toolbox convert own_multilingual_caps.srt --to srt --replace uhr=Uhr --replace-ignore-case`, checked by hand | MIT |
| `own_rtl_reversed.srt` | Written for this repository. SubRip with Hebrew and Arabic lines that start with their sentence-end punctuation, as files made for players with a left-to-right base direction store them. Also lines with tags, an override, dialogue dashes, a leading ellipsis, punctuation at both ends and a left-to-right line. | MIT |
| `own_rtl_reversed_fixed.srt` | Written for this repository. Expected `SubRipFormatter` output for `own_rtl_reversed.srt` after `fixRightToLeft()`. | MIT |
