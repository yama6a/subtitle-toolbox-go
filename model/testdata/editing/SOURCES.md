# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/editing, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `harbour_tour.vtt` | Written for this repository. WebVTT in the shape of an HLS stream: an `X-TIMESTAMP-MAP` header, a cue repeated across a segment boundary, `NOTE` comments and cue settings. | MIT |
| `harbour_tour_shifted_back.vtt` | Written for this repository. Expected `WebVttFormatter` output after `shift(-12.5, 17.5)`. The last cue moves before the third cue. | MIT |
| `own_ferry_drift.vtt` | Written for this repository. WebVTT with a cue before, at, across and after 3 sync points, and word timestamps in the cue across 600 s. | MIT |
| `own_ferry_drift_synced.vtt` | Written for this repository. Expected `WebVttFormatter` output for `own_ferry_drift.vtt` after `syncByPoints()` with 10 s to 12 s, 600 s to 610 s and 1200 s to 1205 s. | MIT |
| `own_ferry_drift_shifted.vtt` | Written for this repository. Expected `WebVttFormatter` output for `own_ferry_drift.vtt` after `shift()` by 2 s from 6 s to 600 s. | MIT |
| `own_ferry_drift_shifted_all.vtt` | Written for this repository. Expected `WebVttFormatter` output for `own_ferry_drift.vtt` after `shift(-1.5)`. | MIT |
| `own_ferry_drift_scaled.vtt` | Written for this repository. Expected `WebVttFormatter` output for `own_ferry_drift.vtt` after `scale(1.001)`. The word timestamps move too. | MIT |
