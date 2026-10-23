# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/csv/real, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `mantas_done_shape.csv` | Written for this repository in the shape of the `Start,End,Text` tables in https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/tests/formats/CsvTest.php: times in seconds, quotes only where needed | MIT |
| `excel_de_semicolon.csv` | Written for this repository in the shape of an Excel "CSV UTF-8" export in a German locale: UTF-8 BOM, `;` delimiter, CR LF row ends, LF inside a quoted cell | MIT |
| `dubbing_script.csv` | Written for this repository in the shape of a dubbing script: `Start TC` time codes at 25 fps, a `Character` column, no end column, a notes column | MIT |
| `sheets_export.tsv` | Written for this repository in the shape of a spreadsheet TSV export: tab delimiter, no quotes, an identifier column with one empty cell | MIT |
| `headerless_frame_times.csv` | Written for this repository in the shape of https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/tests/files/csv_ansi.csv: no header row, hh:mm:ss:ff times, quoted cells with LF line breaks and 10 trailing blank lines in the first cell | MIT |
| `own_openai_whisper.tsv` | Written for this repository in the shape of `WriteTSV` in https://github.com/openai/whisper/blob/86098128c0b4f24f0e2aa2994de830614b474227/whisper/utils.py: a `start`, `end`, `text` header, times in whole milliseconds, tab delimiter, no quotes | MIT |
| `own_whisper_cpp.csv` | Written for this repository in the shape of `output_csv` in https://github.com/ggml-org/whisper.cpp/blob/60c0be6ac8fa71b1a2ae2dd938a31a34a508e774/examples/cli/cli.cpp (`-ocsv`): a `start,end,text` header, times in whole milliseconds that are multiples of 10, every text quoted with a leading space | MIT |
| `expected/*.json` | Generated for this repository. Expected parse result of the file with the same name | MIT |
| `expected/*.to.*` | Written by the Go write golden tests with -update, reviewed by hand. They equal the PHP goldens `expected/*.to.*` of this repository byte for byte | MIT |
