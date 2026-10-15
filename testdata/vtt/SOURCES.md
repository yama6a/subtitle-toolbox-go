# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/vtt, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `valid.vtt` | Written for this repository. Valid cues with times from 0 to over 1 hour | MIT |
| `missing_hours.vtt` | Written for this repository. Times without the hour field | MIT |
| `strip_xml.vtt` | Written for this repository. Cues with HTML and XML tags to strip | MIT |
| `all_xml_tags_stripped.vtt` | Written for this repository. Expected `WebVttFormatter` output for `strip_xml.vtt` after `stripTags()` | MIT |
| `with_styles_and_notes.vtt` | Written for this repository. A header title, `STYLE` blocks and `NOTE` comments | MIT |
| `with_styles_and_notes_formatted.vtt` | Written for this repository. Expected `WebVttFormatter` output for `with_styles_and_notes.vtt` | MIT |
| `exceeded_hours.vtt` | Written for this repository. A 4-digit hour field | MIT |
| `exceeded_minutes.vtt` | Written for this repository. A minute field of 99 | MIT |
| `exceeded_seconds.vtt` | Written for this repository. A second field of 99 | MIT |
| `exceeded_milli_accuracy.vtt` | Written for this repository. A 4-digit millisecond field | MIT |
| `missing_cue_number.vtt` | Written for this repository. A cue without an identifier line | MIT |
| `missing_text.vtt` | Written for this repository. Cues without text lines | MIT |
| `missing_timestamps.vtt` | Written for this repository. A cue without its time line | MIT |
| `missing_webvtt_header.vtt` | Written for this repository. No `WEBVTT` line | MIT |
| `no_empty_line_after_webvtt_header.vtt` | Written for this repository. The first cue starts on the line after the `WEBVTT` line | MIT |
