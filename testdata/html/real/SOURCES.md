# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/html/real, MIT. In the rows below, "this repository" means subtitle-toolbox-php. The PHP write goldens `expected/*.out.html` and `expected/*.to.*` stay there.

The example files of the spec and of podcast-transcript-convert hold real podcast dialogue. So every file here is written for this repository in the shape of a real file. The text is new neutral sample text. The shape of `spec_example_shape.html` comes from the Podcasting 2.0 spec of Podcastindex-org/podcast-namespace, CC0-1.0. The credit in `THIRD_PARTY_NOTICES.md` is a courtesy.

| File | Source | License |
|:--- |:--- |:--- |
| `spec_example_shape.html` | Written for this repository in the shape of the spec example [`example.html`](https://github.com/Podcastindex-org/podcast-namespace/blob/c0ff5caa3729610362ee93f8034454fa41f3c493/docs/examples/transcripts/example.html): `<cite>`, `<time>` and `<p>` per paragraph, times such as `1:02`, and `&#39;`. One paragraph spans several indented lines and one name has a space before the colon, as in the snippet of [`transcripts.md`](https://github.com/Podcastindex-org/podcast-namespace/blob/c0ff5caa3729610362ee93f8034454fa41f3c493/docs/examples/transcripts/transcripts.md) | MIT |
| `hhmmss_empty_paragraphs.html` | Written for this repository in the shape of the HTML test fixtures of [podcast-transcript-convert](https://github.com/hbmartin/podcast-transcript-convert/tree/8271304fe248b3b15902adfc953542edf93aa874/tests/fixtures): times as `00:00:00`, a `Speaker 1:` name on each paragraph, and an empty `<p></p>` after each paragraph | MIT |
| `expected/*.json` | Generated for this repository. Expected parse result of the file with the same name, written by `tests/files/generate-read-goldens.php` | MIT |
| `expected/*.out.html` | Written by the Go write golden tests with -update, reviewed by hand. The output of `WriteHTML` with zero options after a read of the file with the same name. It equals the PHP golden of the same name | MIT |
| `expected/spec_example_shape.html.to.*` | Written by the Go write golden tests with -update, reviewed by hand. Each file equals the PHP write golden of the same name at the pinned commit | MIT |
