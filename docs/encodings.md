# Text encodings

The reader turns every text input into UTF-8 before a parser sees it. `ReadInfo.SourceEncoding` names the encoding it read.

<!-- example: ExampleParse_encoding -->
```go
latin1 := []byte("1\n00:00:01,000 --> 00:00:02,000\nCaf\xe9 au lait\n")

s, info, err := stbox.Parse(bytes.NewReader(latin1), stbox.ReadOptions{})
if err != nil {
	fmt.Println(err)
	return
}
fmt.Println(info.SourceEncoding, s.Len()) // Windows-1252 1

_, info, err = stbox.Parse(bytes.NewReader(latin1), stbox.ReadOptions{Encoding: "cp1250"})
if err != nil {
	fmt.Println(err)
	return
}
fmt.Println(info.SourceEncoding) // cp1250, as the caller wrote it
// Output:
// Windows-1252 1
// cp1250
```

## Supported names

`ReadOptions.Encoding` takes a name of this table, compared without case. `stbox.Encodings()` returns the canonical names. Another name fails `ReadOptions.Validate` with `The encoding "CP1125" is unknown.`

| Canonical name | Also accepted |
|:--- |:--- |
| UTF-8 | UTF8 |
| UTF-16LE, UTF-16BE, UTF-16 | |
| UTF-32LE, UTF-32BE, UTF-32 | |
| Windows-1250 to Windows-1258 | CP1250 to CP1258 |
| Windows-874 | CP874 |
| ISO-8859-1 to ISO-8859-11, ISO-8859-13 to ISO-8859-16 | ISO8859-N for each ISO-8859-N, LATIN1 for ISO-8859-1 |
| KOI8-R, KOI8-U | |
| Shift_JIS | SHIFT-JIS, SJIS, MS_KANJI, CSSHIFTJIS, CP932, Windows-31J |
| EUC-JP | |
| GBK | CP936 |
| GB18030 | |
| Big5 | CP950 |
| EUC-KR | |
| CP949 | UHC |
| CP437, CP850, CP866 | IBM437, IBM850, IBM866 |

- **UTF-16 and UTF-32**: little endian unless a BOM says otherwise. glibc iconv reads them in host byte order, which is little endian on x86-64 and arm64, so the results match PHP there.
- **Shift_JIS**: reads as CP932, so the bytes 0x5C and 0x7E stay `\` and `~`. The ASS line break `\N` and tags such as `{\i1}` survive.
- **EUC-KR and CP949**: both read the CP949 table.
- **Decision**: a name is a `string`, and `stbox.Encodings()` lists the names. A constant per name would crowd the root package next to the `Format` constants, and `Validate` catches typos.

## Read order

The first rule that matches wins.

1. A UTF-8 BOM: the bytes stay. The parse step removes the BOM.
2. A UTF-32 or UTF-16 BOM: the rest decodes in that encoding. A BOM wins over `Encoding`.
3. No BOM, and `Encoding` names no UTF-16, UTF-32, UCS-2 or UCS-4 encoding: content of an even length can read as UTF-16 without a BOM. The check counts the zero bytes in the byte pairs of the first 1024 bytes. A zero in at least 40% of the pairs on one side, and in at most 5% on the other side, gives UTF-16. A zero on the second side gives LE, on the first side BE.
4. No `Encoding`: content that is valid UTF-8, holds a NUL byte, or holds more valid multi-byte UTF-8 sequences than bad bytes stays UTF-8.
5. No `Encoding`: code page detection picks the code page. Without a result, the bytes stay and the encoding is UTF-8.
6. `Encoding` is UTF-8: the bytes stay.
7. Valid UTF-8 without a NUL byte stays UTF-8 for every `Encoding`. A pure ASCII file read with `Windows-1252` reports UTF-8.
8. Else the content decodes with `Encoding`.

A byte that the encoding does not define fails the read in both modes with a `*ParseError`: `Cannot convert the content from Windows-1252 to UTF-8.` The decoders of `golang.org/x/text` write U+FFFD for such a byte. The reader turns that U+FFFD into the failure, and also a C1 control from a Windows code page, because Microsoft leaves those bytes undefined.

## Code page detection

Detection decodes the lines with a byte from 0x80 with 23 candidates: Windows-1250 to Windows-1258, Windows-874, KOI8-R, KOI8-U, ISO-8859-1, -2, -5, -6, -7 and -9, CP949, CP932, EUC-JP, GBK and CP950. It scores the letters of the languages of each candidate and costs other letters, symbols, mixed scripts and odd letter case. [detect.go](../internal/textenc/detect.go) holds the rules.

- **CJK with a broken character**: a CJK file with a byte that its code page cannot decode keeps its bytes, so it never reads as Thai or Arabic text.
- **Ambiguous bytes**: a Thai TIS-620 file also decodes as CP932, and an Estonian Windows-1257 file as GBK. The scores pick Windows-874 and Windows-1257.

## Warnings

A lenient read puts a guess first in `ReadInfo.Warnings`, with action `Repaired` and no line. A strict read drops it.

| Rule | Message |
|---:|:--- |
| 3 | `The content is UTF-16LE without a BOM.` (or `UTF-16BE`) |
| 5 | `The content is not UTF-8. Detection picked Windows-1252. Set the source encoding if that is wrong.` |

The CLI prints `Pass --encoding to set the source encoding.` after such a warning when the user gave no `--encoding`.

## Invalid UTF-8

The parse step checks UTF-8 after decoding. A bad byte is a byte outside a valid UTF-8 sequence of RFC 3629. The offset counts from the start of the decoded content, before the BOM.

| Formats | Strict | Lenient |
|:--- |:--- |:--- |
| TTML, SAMI, iTT, YouTube XML | `*ParseError`: `The content is not valid UTF-8. The first bad byte is at offset N. Set the source encoding of the file.` | each bad byte becomes U+FFFD. `Repaired` warning with the same text plus ` The parser read the bad bytes as U+FFFD.` |
| every other text format | the bytes stay, no warning | the bytes stay. `Repaired` warning with the first message |
| JSON formats | each bad byte becomes U+FFFD while decoding | the same, plus the `Repaired` warning |

## Output

Every writer writes UTF-8. A text format writes the bytes of invalid UTF-8 as they are, so a Latin-1 MicroDVD file read without an encoding round-trips byte for byte. JSON, TTML, iTT and SAMI return an `*UnwritableError` for them.

## Differences from glibc iconv

PHP decodes with glibc iconv, and Go with the WHATWG tables of `golang.org/x/text`. Code page detection gives the same result on every test file. With a chosen encoding, these bytes differ.

| Encoding | Bytes | PHP | Go |
|:--- |:--- |:--- |:--- |
| EUC-KR | a CP949 pair such as `8C 63` | U+008C and `c` | U+B620, as CP949 |
| EUC-KR | a CP949 pair with a first byte from `81` to `A0` and a second byte from `A1`, such as `81 A1` | fails | U+AC7E for `81 A1` |
| EUC-KR | a CP949 pair with a first byte from `A1` to `C6` and a second byte from `41` to `A0`, such as `A1 41` | fails | U+C8A5 for `A1 41` |
| EUC-KR | a lone byte from `80` to `9F` | a C1 control | fails |
| Big5, CP950 | an HKSCS pair such as `87 40` | fails | U+43F0 |
| Big5, CP950 | 365 pairs from `C6 A1` to `C8 FE` | private use, such as U+F6B1 for `C6 A1` | other characters, such as U+2460 for `C6 A1` |
| Big5, CP950 | the 43 pairs from `C8 A5` to `C8 F4` | private use, such as U+F7EF for `C8 A5` | fails |
| Big5, CP950 | the 33 pairs from `A3 C0` to `A3 E0` | fails | U+2400 to U+2421 |
| Big5, CP950 | `F9 FE` | U+2593 | U+FFED |
| Big5, CP950 | `80` | U+0080 | fails |
| Shift_JIS, CP932 | the user-defined pairs `F0 40` to `F9 FC` | private use from U+E000 | fails |
| Shift_JIS, CP932 | `80` | fails | U+0080 |
| EUC-JP | a lone byte from `80` to `8D` or `90` to `9F` | a C1 control | fails |
| EUC-JP | the NEC pairs with the first byte `AD` or `F9` to `FC`, such as `AD A1` | fails | U+2460 for `AD A1` |
| EUC-JP | `A1 C1`, `A1 C2`, `A1 DD`, `A1 F1`, `A1 F2`, `A2 CC` | U+301C, U+2016, U+2212, U+00A2, U+00A3, U+00AC | U+FF5E, U+2225, U+FF0D, U+FFE0, U+FFE1, U+FFE2 |
| GBK, CP936 | 82 pairs such as `A2 E3` and `A8 BF` | fails | U+20AC and U+01F9 |
| GB18030 | `80` | fails | U+20AC |
| GB18030 | the user-defined pairs, such as `A1 40` | private use, such as U+E4C6 | fails |
| GB18030 | `A3 A0` | U+E5E5 | U+3000 |
| GB18030 | 19 four-byte sequences: `81 35 F4 37`, `82 35 90 37` to `82 35 91 34`, `84 31 82 36` to `84 31 83 35` | private use, such as U+E7C7 for `81 35 F4 37` | U+1E3F, U+9FB4 to U+9FBB, U+FE10 to U+FE19 |
| Windows-1255 | `CA` | fails | U+05BA |
| KOI8-U | `AE`, `BE` | U+255D, U+256C | U+045E, U+040E |

- **Names**: PHP accepts every iconv name, for example `CP1125`. Go accepts only the table above.
- **Test files**: `chinese-gbk.srt` read with `Encoding: "Big5"` holds `C8 A5`. PHP reads the file, and Go fails with `Cannot convert the content from Big5 to UTF-8.` The x/text table has no entry for these pairs.
