# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/deepgram/real, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

Deepgram responses for real recordings hold the words of their speakers. So every file here is written for this repository in the exact shape of the Deepgram pre-recorded audio API response. The text is new neutral sample text.

| File | Source | License |
|:--- |:--- |:--- |
| `pool_utterances_diarize.json` | Written for this repository in the shape of a response for `utterances=true&diarize=true&smart_format=true`: [pre-recorded audio](https://developers.deepgram.com/docs/pre-recorded-audio), [utterances](https://developers.deepgram.com/docs/utterances) and [diarization](https://developers.deepgram.com/docs/diarization). Words with `speaker`, `speaker_confidence` and `punctuated_word`, 32-bit float times such as `3.59999998` | MIT |
| `train_paragraphs_german.json` | Written for this repository in the shape of a response for `smart_format=true&detect_language=true`: [paragraphs](https://developers.deepgram.com/docs/paragraphs) and [language detection](https://developers.deepgram.com/docs/language-detection). German text, `detected_language` and `language_confidence` on the channel | MIT |
| `museum_words_punctuate.json` | Written for this repository in the shape of a response for `punctuate=true` without paragraphs: [pre-recorded audio](https://developers.deepgram.com/docs/pre-recorded-audio). A sentence of 90 characters | MIT |
| `expected/*.json` | Generated for this repository. Expected parse result of the file with the same name, written by `tests/files/generate-read-goldens.php` | MIT |
| `expected/*.out.json`, `expected/*.to.*` | Written by the Go write golden tests with -update, reviewed by hand. Each file equals the PHP write golden of the same name at the pinned commit | MIT |
