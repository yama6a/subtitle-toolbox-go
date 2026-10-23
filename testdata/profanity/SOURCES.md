# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/profanity, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| keys.srt | self-written SubRip file with CR LF line endings, 7 cues, an italic cue, a two-line dialogue cue, matches in cue 7, the last cue | MIT |
| radio.vtt | self-written WebVTT file with cue identifiers and a word timestamp before each word, 4 cues, a match in the last word of the last cue | MIT |
| words.txt | self-written word list with a UTF-8 BOM, CR LF line endings and an empty last line | MIT |
| `keys_stars.srt`, `keys_first_letter.srt`, `keys_removed.srt` | Generated for this repository. Output of `subtitle-toolbox convert keys.srt --to srt --mask-words words.txt` with `--mask stars`, `first-letter` and `remove`, checked by hand | MIT |
| `radio.af` | Generated for this repository. Output of `subtitle-toolbox convert radio.vtt --mask-words words.txt --mask none --mute-filter radio.af --mute-padding 00:00:00,1`, checked by hand against the EDL times | MIT |
