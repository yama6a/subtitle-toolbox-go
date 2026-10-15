# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/vtt/real, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `astisub_carriage_return.vtt` | Written for this repository in the shape of https://github.com/asticode/go-astisub/blob/721d3fc8258bcc5912423c551da15ad39ebcc553/testdata/example-in-carriage-return.vtt | MIT |
| `astisub_html_entities.vtt` | Written for this repository in the shape of https://github.com/asticode/go-astisub/blob/f285923a0c8d5b2ecce67d8eaf9ed26cce16e5b0/testdata/example-in-html-entities.vtt | MIT |
| `mantas_styles.vtt` | https://github.com/mantas-done/subtitles/blob/e41f318224dc9c38339c2bfcb0199e5bb788abce/tests/files/vtt_with_styles.vtt | mantas-done/subtitles, MIT |
| `own_note_before_earlier_cue.vtt` | Written for this repository. Cues out of time order, and a `NOTE` before a cue that starts earlier than the cue before it | MIT |
| `own_arrow_in_text.vtt` | Written for this repository. The output for a cue with the lines `a --> b`, an empty line and `c`, set through the API | MIT |
| `w3c_chapters.vtt` | https://www.w3.org/TR/webvtt1/, section 1.6, chapters example | W3C Software and Document License, https://www.w3.org/copyright/software-license-2023/ |
| `w3c_comments.vtt` | https://www.w3.org/TR/webvtt1/, section 1.5, comments in WebVTT | W3C Software and Document License, https://www.w3.org/copyright/software-license-2023/ |
| `w3c_cue_settings.vtt` | https://www.w3.org/TR/webvtt1/, section 1.4, cue settings example | W3C Software and Document License, https://www.w3.org/copyright/software-license-2023/ |
| `w3c_identifiers.vtt` | https://www.w3.org/TR/webvtt1/, section 1.4, cue identifier example | W3C Software and Document License, https://www.w3.org/copyright/software-license-2023/ |
| `w3c_regions.vtt` | https://www.w3.org/TR/webvtt1/, section 1.4, region example | W3C Software and Document License, https://www.w3.org/copyright/software-license-2023/ |
| `w3c_styles.vtt` | https://www.w3.org/TR/webvtt1/, section 1.3, styling captions | W3C Software and Document License, https://www.w3.org/copyright/software-license-2023/ |
| `w3c_timestamps.vtt` | https://www.w3.org/TR/webvtt1/, section 8.1, `:past` and `:future` example | W3C Software and Document License, https://www.w3.org/copyright/software-license-2023/ |
| `w3c_voices.vtt` | Written for this repository in the shape of https://www.w3.org/TR/webvtt1/, section 1.1, a simple caption file | MIT |
| `webvttpy_comments.vtt` | Written for this repository in the shape of https://github.com/glut23/webvtt-py/blob/6a92fd3fb428dd2367c9d06e7a406d0aa9c0655b/tests/samples/comments.vtt | MIT |
| `webvttpy_netflix.vtt` | Written for this repository in the shape of https://github.com/glut23/webvtt-py/blob/fdbf129c514328ab1344565174ffb46162c8535d/tests/samples/netflix_chicas_del_cable.vtt, first 30 cues | MIT |
| `webvttpy_youtube.vtt` | Written for this repository in the shape of https://github.com/glut23/webvtt-py/blob/fdbf129c514328ab1344565174ffb46162c8535d/tests/samples/youtube_dl.vtt | MIT |
| `own_empty_cues.vtt` | Written for this repository. Cue 2 has an identifier and no text, as in a user report of a naver.com file. Cue 4 has `align:middle line:90%` settings, no text and two empty lines after it, as in user reports of vendor files. | MIT |
| `own_ytdlp_auto_captions.vtt` | Written for this repository in the shape of YouTube automatic captions that `yt-dlp --write-auto-subs` saves. Each cue holds a line with one space. | MIT |
| `own_ytdlp_rolling.vtt` | Written for this repository in the shape of yt-dlp `--write-auto-subs` output. Rolling cues with `<c>` word timestamps, a 10 ms repeat cue after each, and a line of one word | MIT |
| `own_hour_digits.vtt` | Written for this repository. One cue with 1 hour digit, then the times of https://github.com/captioning/captioning/blob/27d0e86693f4d9bc2046102f74aa54febcd902dc/tests/Fixtures/Webvtt/long-hours.vtt with hours of 2 to 4 digits and leading zeros | MIT |
| `own_settings_without_space.vtt` | Written for this repository in the shape of the vtt.js test file `file-layout/no-space-cue-times-cue-settings.vtt`, https://github.com/hartman/vtt-vivid/tree/1b9a9c1c1072cea68dec1c7e136e32c90578592a/tests/integration/data/file-layout. Cue settings directly after the end time | MIT |
| `own_angle_bracket_text_from_srt.vtt` | Generated for this repository. Expected `WebVttFormatter` output for `srt/real/own_angle_bracket_text.srt`. | MIT |
| `own_max_hours.vtt` | Written for this repository. The last cue ends at 99999:59:59.999, the latest time that the parsers accept | MIT |
| `own_max_word_timestamp.vtt` | Written for this repository. The last cue has a word timestamp at 99999:59:59.000 and ends at 99999:59:59.999 | MIT |
| `own_srt_cue_settings_converted.vtt` | Written for this repository. Expected `WebVttFormatter` output for `srt/real/own_vtt_cue_settings.srt`. | MIT |
| `own_color_classes.vtt` | Written for this repository. The 8 color classes of https://www.w3.org/TR/webvtt1/#default-text-color, a background class and a class without a color | MIT |
| `own_font_colors_from_srt.vtt` | Generated for this repository. Expected `WebVttFormatter` output for `srt/real/own_font_colors.srt`. | MIT |
| `own_openai_whisper.vtt` | Written for this repository in the shape of openai-whisper `--output_format vtt` output: no cue identifiers, `MM:SS.mmm` times below 1 hour and `HH:MM:SS.mmm` from 1 hour. Made with `WriteVTT` of https://github.com/openai/whisper/blob/86098128c0b4f24f0e2aa2994de830614b474227/whisper/utils.py from our own segments | MIT |
| `own_openai_whisper_highlight_words.vtt` | Written for this repository in the shape of openai-whisper `--output_format vtt --highlight_words True` output: one cue per word with the word in `<u>`, and a cue without `<u>` for a gap between 2 words. Made with `SubtitlesWriter.iterate_result` of https://github.com/openai/whisper/blob/86098128c0b4f24f0e2aa2994de830614b474227/whisper/utils.py from our own word timings | MIT |
| `own_ffmpeg.vtt` | Output of FFmpeg 7.0.2 for `ffmpeg -i ass/real/own_ffmpeg.ass own_ffmpeg.vtt`, unchanged: no cue identifiers, no hours below 1 hour, a bare `&`, no colours and no alignment | MIT |
| `expected/*.json` | Generated for this repository. Expected parse result of the file with the same name, written by `tests/files/generate-read-goldens.php` | MIT |
| `expected/*.out.*` | Written by the Go write golden tests with -update, reviewed by hand. Each file equals the PHP write golden of the same name at the pinned commit | MIT |
