# Sources

| File | Source | License |
|:--- |:--- |:--- |
| `own_station.srt` | Written for this repository in the shape of a SubRip file: CR LF, sentences over two cues, italics over two lines and over two cues, a music cue and a number cue. | MIT |
| `own_entities.srt` | Written for this repository in the shape of a SubRip file: text that holds the entities `&lt;`, `&gt;` and `&amp;`. | MIT |
| `deepl_de.json` | Written for this repository in the shape of a DeepL `/v2/translate` response to the sentences of `own_station.srt`. | MIT |
| `deepl_error.json` | Written for this repository in the shape of a DeepL HTTP 400 error body. | MIT |
| `google_fr.json` | Written for this repository in the shape of a Cloud Translation v2 `translate` response to the sentences of `own_station.srt`. | MIT |
| `google_error.json` | Written for this repository in the shape of a Cloud Translation v2 HTTP 400 error body. | MIT |
| `openai_de.json` | Written for this repository in the shape of an OpenAI `/v1/chat/completions` response to the sentences of `own_station.srt`. | MIT |
| `openai_reasoning_de.json` | The answer `openai_de.json` in the shape of a thinking model on Ollama: an empty `content` and the array in `reasoning`. | MIT |
| `openai_error.json` | Written for this repository in the shape of an OpenAI chat completions error body. | MIT |
| `deepl_entities_de.json` | Written for this repository in the shape of a DeepL `/v2/translate` response to the sentences of `own_entities.srt`, with the entities as DeepL keeps them. | MIT |
| `own_station_de_deepl.vtt` | Golden output: `own_station.srt` translated with `DeepLEngine` and the answer `deepl_de.json`, written as WebVTT. | MIT |
| `own_station_fr_google.srt` | Golden output: `own_station.srt` translated with `GoogleTranslateEngine` and the answer `google_fr.json`. | MIT |
| `own_station_de_openai.srt` | Golden output: `own_station.srt` translated with `OpenAiCompatibleEngine` and the answer `openai_de.json`. | MIT |
| `own_entities_de_deepl.srt` | Golden output: `own_entities.srt` translated with `DeepLEngine` and the answer `deepl_entities_de.json`. | MIT |
