# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/fixes, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `own_overlaps_and_short_cues.srt` | Written for this repository in the shape of a SubRip file from a subtitle editor: BOM, CR LF, italic and bold tags, overlaps and cues shorter than 0.5 s. | MIT |
| `own_overlaps_and_short_cues_fixed.srt` | Written for this repository. Expected `SubRipFormatter` output for `own_overlaps_and_short_cues.srt` after `fixOverlaps(0.083)`, `extendShortCues(0.833, 0.083)` and `wrapLines(42)`. | MIT |
| `own_dialogue_dashes.srt` | Written for this repository in the shape of a SubRip file from a subtitle editor: CR LF, italic tags, and cues with two or three dialogue turns, one of them on a single line. | MIT |
| `own_dialogue_dashes_wrapped.srt` | Written for this repository. Expected `SubRipFormatter` output for `own_dialogue_dashes.srt` after `wrapLines(42)`. | MIT |
| `own_dialogue_turns_wrapped.srt` | Written for this repository in the shape of a SubRip file from a subtitle editor: CR LF, italic tags, dialogue turns wrapped over 2 lines, a line that starts with a minus sign and a first line without a dash. | MIT |
| `own_dialogue_turns_unwrapped.srt` | Written for this repository. Expected `SubRipFormatter` output for `own_dialogue_turns_wrapped.srt` after `unwrapLines()`, without BOM. | MIT |
| `own_asr_tight_timing.srt` | Written for this repository in the shape of a SubRip file from speech-to-text: LF, cues that start and end on the speech, a 0.1 s gap, touching cues and a sound cue that overlaps dialogue. | MIT |
| `own_asr_tight_timing_lead.srt` | Written for this repository. Expected `SubRipFormatter` output for `own_asr_tight_timing.srt` after `addLeadInOut(0.2, 0.3, 0.083)`, with the BOM that the formatter writes. | MIT |
| `own_long_cues.srt` | Written for this repository in the shape of a SubRip file from speech-to-text: LF, a 25 s greeting, a 12.45 s cue that runs through silence and a cue of exactly 7 s. | MIT |
| `own_long_cues_limited.srt` | Written for this repository. Expected `SubRipFormatter` output for `own_long_cues.srt` after `limitLongCues(7)`, with the BOM that the formatter writes. | MIT |
| `own_zh_long_lines.srt` | Written for this repository in the shape of a SubRip file from speech-to-text in Simplified Chinese: LF, no BOM, cues of 13 to 29 characters on one line, full-width punctuation, curly quotes, a Latin word, a cue on 2 lines and a cue of only punctuation. | MIT |
| `own_zh_long_lines_wrapped.srt` | Written for this repository. Expected `SubRipFormatter` output for `own_zh_long_lines.srt` after `wrapLines(16)`, with the BOM that the formatter writes. | MIT |
| `own_ja_long_lines.srt` | Written for this repository in the shape of a SubRip file from machine translation into Japanese: LF, no BOM, cues of 2 to 26 characters on one line, corner brackets, small kana, the long vowel mark, a Latin word and a cue of only punctuation. | MIT |
| `own_ja_long_lines_wrapped.srt` | Written for this repository. Expected `SubRipFormatter` output for `own_ja_long_lines.srt` after `wrapLines(13)`, with the BOM that the formatter writes. | MIT |
| `own_cjk_dialogue.srt` | Written for this repository in the shape of a SubRip file from speech-to-text in Simplified Chinese and Japanese: LF, no BOM, 2 or 3 dialogue turns on one line after `？`, `！`, `。` and `｡`, and a minus sign after `。`. | MIT |
| `own_cjk_dialogue_wrapped.srt` | Written for this repository. Expected `SubRipFormatter` output for `own_cjk_dialogue.srt` after `wrapLines(16)`, with the BOM that the formatter writes. | MIT |
