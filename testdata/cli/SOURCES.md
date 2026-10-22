# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/cli, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| trip.srt | self-written, CR LF line endings, an overlap, a short cue, a long line and hearing-impaired annotations | MIT |
| shop.vtt | self-written | MIT |
| broken.srt | self-written, cue 2 has a `=>` arrow | MIT |
| frames.sub | self-written MicroDVD without a `{1}{1}<fps>` line | MIT |
| latin1.srt | self-written in Windows-1252 | MIT |
| notes.txt | self-written text that is not a subtitle | MIT |
| frames_23976.vtt | expected `convert frames.sub --input-fps 25 --from-fps 25 --to-fps 23.976 --to vtt` output | MIT |
| trip_info.txt, trip_info.json | Generated for this repository. Output of `subtitle-toolbox info trip.srt` without and with `--json`, checked by hand | MIT |
| expected/info_trip.txt, expected/info_trip.json, expected/frames_23976.vtt | Written by the Go write golden tests with -update, reviewed by hand. `frames_23976.vtt` equals `../frames_23976.vtt` | MIT |
