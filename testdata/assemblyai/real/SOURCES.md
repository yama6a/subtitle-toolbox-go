# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/assemblyai/real, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

AssemblyAI transcripts of real recordings hold the words of their speakers. So every file here is written for this repository in the exact shape of the AssemblyAI transcript resource. The text is new neutral sample text.

| File | Source | License |
|:--- |:--- |:--- |
| `hike_utterances_speakers.json` | Written for this repository in the shape of the [Get transcript](https://www.assemblyai.com/docs/api-reference/transcripts/get) response for `speaker_labels=true`: the fields of the example in the order of the API, `words` and `utterances` with speakers `A` and `B`, times in milliseconds, language `en_us` | MIT |
| `market_words_french.json` | Written for this repository in the shape of the same response without speaker labels, so `"utterances": null`, with language detection. French text, a pause of 1.3 s and a sentence of 100 characters | MIT |
| `expected/*.json` | Generated for this repository. Expected parse result of the file with the same name, written by `tests/files/generate-read-goldens.php` Changed: the empty JSON objects `content_safety_labels.summary`, `content_safety_labels.severity_score_summary` and `iab_categories_result.summary` read as `{}` and not `[]`, because the Go reader keeps the type of an empty object (see docs/php-differences.md) | MIT |
| `expected/*.out.json`, `expected/*.to.*` | Written by the Go write golden tests with -update, reviewed by hand. Each file equals the PHP write golden of the same name at the pinned commit, except that the empty objects named in the Changed note above write as `{}` | MIT |
