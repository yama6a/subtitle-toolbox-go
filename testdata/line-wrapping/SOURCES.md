# Sources

| File | Source | License |
|:--- |:--- |:--- |
| `wrap.json` | The Chinese, Japanese and Korean cases of tests/LineWrapperTest.php of yama6a/subtitle-toolbox-php at `c0adecd5375820d5313f1583fc7e67942d514600`. Every `want` value is the result of `LineWrapper` of that commit. The text `\xff` stands for the byte 0xFF, because JSON holds only valid UTF-8. | MIT |
| `wrap_lines.json` | The Arabic and Japanese cases of `testWrapLinesBreaksArabicTextAtSpaces` and `testWrapLinesBreaksTextWithoutSpacesBetweenCharacters` in tests/FixesTest.php, and the first 2 cues of `testEachUnwrappedLineGetsItsOwnRightToLeftEmbedding` in tests/TextTransformsTest.php of yama6a/subtitle-toolbox-php at `c0adecd5375820d5313f1583fc7e67942d514600`. Every `want` value is the result of `wrapLines` or `unwrapLines` of that commit. | MIT |
