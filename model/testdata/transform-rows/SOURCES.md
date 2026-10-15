# Sources

| File | Source | License |
|:--- |:--- |:--- |
| `case.json` | The rows "German sharp s in upper case", "Turkish dotted capital I in upper case", "Azerbaijani dotless i in lower case", "Greek final sigma", "sentence case title-cases sharp s", "sentence continues in the next cue", "speaker change and dialogue dash start a sentence", the 4 "English pronoun" rows and the 2 rows of the dot of I come from tests/TextTransformsTest.php and tests/SentenceCaserTest.php of yama6a/subtitle-toolbox-php at `c0adecd5375820d5313f1583fc7e67942d514600`. The other rows are written for this repository. Every `want` value is the result of `changeCase` of that commit. | MIT |
| `replace.json` | The first row comes from `testReplaceTextCaseInsensitiveWithoutRegex` in tests/TextTransformsTest.php of yama6a/subtitle-toolbox-php at `c0adecd5375820d5313f1583fc7e67942d514600`. The second row is written for this repository. Every `want` value is the result of `replaceText` of that commit. | MIT |
| `bidi.json`, `rtl.json` | Rows of `bidiControlProvider` and `rightToLeftProvider` in tests/TextTransformsTest.php of yama6a/subtitle-toolbox-php at `c0adecd5375820d5313f1583fc7e67942d514600`. | MIT |
| `multilingual.json` | The expected texts of `testRealMultilingualFileInLowerCase` and `testTurkishRulesApplyToEveryCue` in tests/TextTransformsTest.php of yama6a/subtitle-toolbox-php at `c0adecd5375820d5313f1583fc7e67942d514600`. | MIT |
