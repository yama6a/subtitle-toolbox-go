# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/speakers, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `whisper_cpp_diarize.json` | Written for this repository in the shape of whisper.cpp `-oj -di` on stereo audio: `output_json()` and `estimate_diarization_speaker()` in [`cli.cpp`](https://github.com/ggml-org/whisper.cpp/blob/60c0be6ac8fa71b1a2ae2dd938a31a34a508e774/examples/cli/cli.cpp). Tab indentation, `speaker` after `text` with the values `0`, `1` and `?` | MIT |
| `whisper_cpp_diarize.vtt` | Written for this repository. Expected the WebVTT writer output without BOM for `whisper_cpp_diarize.json` with `transcript.ReadOptions.SpeakerVoices` | MIT |
