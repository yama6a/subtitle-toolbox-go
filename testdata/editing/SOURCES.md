# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/editing, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `film_part1.srt` | Written for this repository. Part 1 of a film as SubRip, with UTF-8 BOM and CR LF line endings. | MIT |
| `film_part2.srt` | Written for this repository. Part 2 of the same film, without BOM, with LF line endings and an `{\an8}` tag. | MIT |
| `film_merged.srt` | Written for this repository. Expected `SubRipFormatter` output after merging part 2 into part 1 at 00:52:10.000. | MIT |
| `harbour_tour.vtt` | Written for this repository. WebVTT in the shape of an HLS stream: an `X-TIMESTAMP-MAP` header, a cue repeated across a segment boundary, `NOTE` comments and cue settings. | MIT |
| `harbour_tour_deduplicated.vtt` | Written for this repository. Expected `WebVttFormatter` output after `removeDuplicateCues()`. | MIT |
| `harbour_tour_split.vtt` | Written for this repository. Expected `WebVttFormatter` output after `splitCue(2, 9.5, 1)`. | MIT |
| `harbour_tour_joined.vtt` | Written for this repository. Expected `WebVttFormatter` output after `joinCues(3, 4)`. | MIT |
| `harbour_tour_slice.vtt` | Written for this repository. Expected `WebVttFormatter` output after `withSlice(6, 16, true)`. | MIT |
| `harbour_tour_shifted_back.vtt` | Written for this repository. Expected `WebVttFormatter` output after `shift(-12.5, 17.5)`. The last cue moves before the third cue. | MIT |
| `harbour_tour_start_edited.vtt` | Written for this repository. Expected `WebVttFormatter` output after the start of cue 3 moves from 13 s to 19 s. The cue and the comment before it move after the cue at 18 s. | MIT |
| `own_duplicate_cues.srt` | Written for this repository. SubRip with an exact duplicate, an overlapping duplicate, a touching duplicate and a same-text cue 0.5 s later. | MIT |
| `own_duplicate_cues_deduplicated.srt` | Written for this repository. Expected `SubRipFormatter` output for `own_duplicate_cues.srt` after `removeDuplicateCues()`. | MIT |
| `own_ferry_drift.vtt` | Written for this repository. WebVTT with a cue before, at, across and after 3 sync points, and word timestamps in the cue across 600 s. | MIT |
| `own_ferry_drift_synced.vtt` | Written for this repository. Expected `WebVttFormatter` output for `own_ferry_drift.vtt` after `syncByPoints()` with 10 s to 12 s, 600 s to 610 s and 1200 s to 1205 s. | MIT |
| `own_ferry_drift_shifted.vtt` | Written for this repository. Expected `WebVttFormatter` output for `own_ferry_drift.vtt` after `shift()` by 2 s from 6 s to 600 s. | MIT |
| `own_ferry_drift_shifted_all.vtt` | Written for this repository. Expected `WebVttFormatter` output for `own_ferry_drift.vtt` after `shift(-1.5)`. | MIT |
| `own_ferry_drift_scaled.vtt` | Written for this repository. Expected `WebVttFormatter` output for `own_ferry_drift.vtt` after `scale(1.001)`. The word timestamps move too. | MIT |
| `own_glow_duplicates.ass` | Written for this repository. ASS with a `Glow` event on layer 0 under a `Default` event on layer 1 with the same text and times, and 2 touching `Default` events with the same text. | MIT |
| `own_glow_duplicates_deduplicated.ass` | Written for this repository. Expected `AssFormatter` output for `own_glow_duplicates.ass` after `removeDuplicateCues()`. | MIT |
| `own_same_time_speakers.ass` | Written for this repository. ASS with 2 dialogue events of 2 speakers at the same times, each with its own style, and a later event. | MIT |
| `own_same_time_speakers_merged.srt` | Written for this repository. Expected `SubRipFormatter` output for `own_same_time_speakers.ass` after `mergeSameTimeCues()`. | MIT |
