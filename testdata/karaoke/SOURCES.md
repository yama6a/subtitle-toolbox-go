# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/karaoke, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `hebrew.lrc` | Written for this repository. Enhanced LRC in Hebrew with word timestamps, a UTF-8 BOM and LF line endings, in the shape that `LyricsFormatter` writes. | MIT |
| `hebrew_word.srt` | Written for this repository. Expected `SubRipFormatter` output of `WordHighlight::apply()` on `hebrew.lrc` with the default options. | MIT |
| `whisper_word.srt` | Written for this repository. Expected output on `../whisper/real/openai_whisper_word_timestamps.json` with the default options. | MIT |
| `whisper_one_word.vtt` | Written for this repository. Expected `WebVttFormatter` output on the same file with style `b` and 1 word per cue. | MIT |
| `whisper_kf.ass` | Written for this repository. Expected `AssFormatter` output of the same file with `AssWriteOptions(karaokeTag: AssKaraokeTag::Fill)`, the `kf` tag. | MIT |
| `lrc_cumulative.srt` | Written for this repository. Expected output on `../lrc/real/handwritten-enhanced.lrc` in mode `cumulative` with style `font color="#ffff00"`. | MIT |
| `aegisub_window.srt` | Written for this repository. Expected output on `../ass/real/own_aegisub.ass` with 3 words per cue. | MIT |
| `lrc_yellow.srt` | Generated for this repository. Output of `subtitle-toolbox convert ../lrc/real/handwritten-enhanced.lrc --to srt --karaoke --karaoke-style 'font color="#ffff00"'`, checked by hand | MIT |
