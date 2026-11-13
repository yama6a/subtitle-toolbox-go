# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/chapters/podcast/real, MIT, except where a row names another licence. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `spec_basic_example.json` | The "Basic example" of https://github.com/Podcastindex-org/podcast-namespace/blob/c0ff5caa3729610362ee93f8034454fa41f3c493/docs/examples/chapters/jsonChapters.md | CC0-1.0, see THIRD_PARTY_NOTICES.md |
| `spec_complex_example.json` | The "More complex example" of the same file, with a silent `toc: false` chapter and a `location` | CC0-1.0, see THIRD_PARTY_NOTICES.md |
| `hosting_export.json` | Written for this repository in the shape of a podcast host export: minified, `endTime` on every chapter, fractional seconds, a chapter without a title | MIT |
| `expected/*.json` | Generated for this repository. Expected parse result of the file with the same name, written by `tests/files/generate-read-goldens.php` | MIT |
