# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/speakers, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

No permissively licensed project ships subtitles with speaker tags and neutral text. So every file here is written for this repository in the shape of real files. The text is new neutral sample text.

| File | Source | License |
|:--- |:--- |:--- |
| `voices.vtt` | Written for this repository in the shape of WebVTT podcast transcripts and captions: `Kind` and `Language` headers, a `NOTE` comment, cue numbers, `<v>` tags with a class, two speakers in one cue and in one line, `<i>` over two lines, and a closing `</v>` | MIT |
| `voices_prefix.srt` | Written for this repository. Expected `SubRipFormatter` output without BOM for `voices.vtt` after `SpeakerLabels::apply()` with `style: SpeakerStyle::Prefix` | MIT |
| `voices_dashes.srt` | Written for this repository. Expected `SubRipFormatter` output without BOM for `voices.vtt` after `SpeakerLabels::apply()` with `style: SpeakerStyle::DialogueDashes` | MIT |
| `voices_colors.srt` | Written for this repository. Expected `SubRipFormatter` output without BOM for `voices.vtt` after `SpeakerLabels::apply()` with `style: SpeakerStyle::Colors` | MIT |
| `sdh_labels.srt` | Written for this repository in the shape of SubRip files with hearing-impaired annotations: CR LF line endings, upper case speaker labels, a label after a dialogue dash, `DR. O'NEIL:`, a mixed case `Note:`, a label inside `<i>`, a label on its own line and a sound description | MIT |
| `sdh_labels_voices.vtt` | Written for this repository. Expected `WebVttFormatter` output without BOM for `sdh_labels.srt` after `SpeakerLabels::apply()` with `speakerLabels: true` | MIT |
