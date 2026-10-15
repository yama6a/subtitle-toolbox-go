# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/srt/real, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `language_subtitles_dots_tester.srt` | https://github.com/Alhadis/language-subtitles/blob/9acc0aa2c232a7b06aa0e8c1058ee5be81a98837/samples/srt/stress/Tester%20for%20dots%20instead%20of%20commas%20in%20timestamps.srt | ISC |
| `language_subtitles_ssa_extensions.srt` | https://github.com/Alhadis/language-subtitles/blob/9acc0aa2c232a7b06aa0e8c1058ee5be81a98837/samples/srt/stress/SSA%20Extensions.srt | ISC |
| `own_alignment_and_coordinates.srt` | Written for this repository. Copies the shape of the FFmpeg SubRip capability tester: CR LF, BOM, coordinates, alignment tags inside lines and font tags. | MIT |
| `own_alignment_and_coordinates_formatted.srt` | Written for this repository. Expected `SubRipFormatter` output for `own_alignment_and_coordinates.srt`. | MIT |
| `own_styled.srt` | Written for this repository in the shape of https://github.com/asticode/go-astisub/blob/58297a613f70cac3d29e5ce52a7215a3b4038b40/testdata/example-in-styled.srt | MIT |
| `own_cr_cr_lf.srt` | Written for this repository. Copies the CR CR LF line endings of a subtitle in the FFmpeg FATE suite. | MIT |
| `own_escaping.srt` | Written for this repository. Holds `<`, `>` and `&` as plain text next to SubRip tags. | MIT |
| `own_timestamp_without_millis.srt` | Written for this repository in the shape of https://github.com/asticode/go-astisub/blob/b6b18718ddb6ee0da08772d8ab310c9c3d2d0459/testdata/example-in.srt | MIT |
| `own_empty_cues.srt` | Written for this repository. Cue 2 has no text and one empty line after it, as in a user report of a naver.com file. Cue 4 has no text and two empty lines after it. | MIT |
| `own_missing_empty_line.srt` | Written for this repository. Cues 2 and 3 have no empty line before their cue number. | MIT |
| `own_angle_bracket_text.srt` | Written for this repository. Holds text in angle brackets that is no tag, next to SubRip tags and unknown tags. | MIT |
| `own_negative_start.srt` | Written for this repository. The output for a cue from -0.5 s to 1 s, set through the API. | MIT |
| `own_vtt_cue_settings.srt` | Written for this repository in the shape that `yt-dlp --convert-subs srt` writes for YouTube automatic captions. Cues 1 to 3 have WebVTT cue settings after the end time. Cue 4 has coordinates. | MIT |
| `own_font_colors.srt` | Written for this repository. `<font color>` as hex, short hex and a name, a color without a WebVTT class and a `<font>` without a color | MIT |
| `own_color_classes_from_vtt.srt` | Generated for this repository. Expected `SubRipFormatter` output for `vtt/real/own_color_classes.vtt`. | MIT |
| `swift_subtitle_kit_embedded_bom.srt` | https://github.com/dioKaratzas/swift-subtitle-kit/blob/728d62dac9fbda7fabd299eed3673ce36c85c267/Tests/SubtitleKitTests/Fixtures/embedded-bom-srt-fixture.srt. Two joined files: LF, then a UTF-8 BOM before cue 1 of the second file and CR LF | swift-subtitle-kit, MIT |
| `own_openai_whisper.srt` | Written for this repository in the shape of openai-whisper `--output_format srt` output, longer than 1 hour. Made with `WriteSRT` of https://github.com/openai/whisper/blob/86098128c0b4f24f0e2aa2994de830614b474227/whisper/utils.py from our own segments | MIT |
| `own_whisper_cpp_diarize.srt` | Written for this repository in the shape of whisper.cpp `-osrt -di` output: times in steps of 10 ms, a `(speaker N)` prefix before the text with its leading space. Shape from `output_srt` in https://github.com/ggml-org/whisper.cpp/blob/60c0be6ac8fa71b1a2ae2dd938a31a34a508e774/examples/cli/cli.cpp | MIT |
| `own_ffmpeg.srt` | Output of FFmpeg 7.0.2 for `ffmpeg -i ass/real/own_ffmpeg.ass own_ffmpeg.srt`, unchanged: CR LF between the text lines of a cue, LF elsewhere, `{\an8}`, `<font color>` | MIT |
| `expected/*.json` | Generated for this repository. Expected parse result of the file with the same name, written by `tests/files/generate-read-goldens.php` | MIT |
