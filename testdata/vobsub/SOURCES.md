# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/vobsub, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

Real VobSub files hold the bitmaps of film subtitles, which no permissive license covers. So `generate.php` in subtitle-toolbox-php wrote every file here. The script has its own run-length encoder and MPEG-2 packer, separate from the reader. It packs each subpicture unit into 2048-byte packs with a PTS in the first PES packet and padding packets, as DVD rips have. The `.idx` comment and setting lines follow the shape of the files that VSFilter writes (`CVobSubFile::WriteIdx` in [MPC-HC](https://github.com/clsid2/mpc-hc/blob/develop/src/Subtitles/VobSubFile.cpp)).

| File | Source | License |
|:--- |:--- |:--- |
| `two-tracks-pal.idx`, `two-tracks-pal.sub` | Written for this repository by `generate.php`. 720x576, CR LF, tracks `en` (index 0, 5 units) and `de` (index 1, 2 units, a `delay` line). One forced unit, one unit with a start delay, two units without a stop command, one unit over 6 packs, one time above 1 hour | MIT |
| `custom-colors-ntsc.idx`, `custom-colors-ntsc.sub` | Written for this repository by `generate.php`. 720x480, LF, `custom colors: ON`, tracks `ja` (index 0) and `fr` (index 2, sub-stream 0x22), one forced unit | MIT |
| `text-pal.idx`, `text-pal.sub` | Written for this repository by `generate.php`. 720x576, CR LF, track `en`, 6 units of 1 or 2 lines in Liberation Sans, 24 to 30 px. 4 colors: fill, fill edge, black outline and background | MIT |
| `one-packet-pal.idx`, `one-packet-pal.sub` | Written for this repository by `generate.php` in the layout of Subtitle Edit and the ffmpeg `dvdsub` muxer: one PES packet per unit without padding packets, and an `.idx` with only the size and palette lines and no `custom colors` line. 720x576, LF, track `en`, 3 units of text drawn as in `text-pal.sub`. The second unit stores the bottom field first, so its `SET_DSPXA` command points to the second field first | MIT |
| `expected/*.idx.*.json` | The pixel golden of each track: the times, position, size, forced flag, alignment and the SHA-256 of the RGBA bytes of each cue. Written by `tests/files/generate-image-goldens.php` with the PHP library | MIT |
| `expected/*.idx.*.png` | The PNG of the first cue of each track, for a reviewer to look at | MIT |
| `expected/*.idx.*.to.sup` | The PGS file that `stbox.Write` writes for each track. Written by `go test -update`. The PHP library writes the same bytes | MIT |
