# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/short-cues, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `own_speech_to_text.srt` | Written for this repository in the shape of SubRip output from a speech-to-text tool: LF line endings, no BOM, many cues under 1 s, dialogue dashes and an italic tag. | MIT |
| `own_speech_to_text_merged.srt` | Written for this repository. Expected `SubRipFormatter` output for `own_speech_to_text.srt` after `mergeShortCues()` with the default options. | MIT |
| `own_interview.vtt` | Written for this repository in the shape of a WebVTT interview transcript with `<v>` speaker tags. | MIT |
| `own_interview_merged.vtt` | Written for this repository. Expected `WebVttFormatter` output for `own_interview.vtt` after `mergeShortCues()` with `mergeSameSpeakerAnyDuration: true`. | MIT |
| `merge_rtl.json` | The cues and the expected text of `testMergedLinesGetOneRightToLeftEmbedding` in tests/ShortCueMergingTest.php of yama6a/subtitle-toolbox-php at `c0adecd5375820d5313f1583fc7e67942d514600`. | MIT |
