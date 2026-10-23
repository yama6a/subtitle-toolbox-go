# Third-party notices

This repository holds material from other projects, and the `stbox` binary links Go modules. Each section names the project, its licence, the material that comes from it, and the upstream notice copied unchanged from the pinned commit or version. Rules for adding material are in [CONTRIBUTING.md](CONTRIBUTING.md).

## Section format

Each section is `## <project>` with these bullets, then the upstream licence or NOTICE text in a fenced `text` block. A bullet appears only when it has content. The heading of a linked module is its module path, for example `## golang.org/x/text`, so `TestEveryLinkedModuleHasANotice` can find it.

| Bullet | Content |
|:--- |:--- |
| Project | the URL |
| Licence | the SPDX name |
| Checked at | the full commit SHA, or the module version |
| Files copied from it | testdata paths |
| Files written for this repository in the shape of its fixtures | testdata paths |
| Code adapted from it | Go file, what it holds, upstream file and commit |
| Data used | for tables taken from data files |
| Changed | Apache-2.0 only: what changed |

## Go

- Project: <https://go.dev>
- Licence: BSD-3-Clause
- Checked at: `go1.27.2`
- Code adapted from it:
  - `internal/markupcore/entity.go`, the HTML5 entity table, from `src/html/entity.go` at `go1.27.2`
  - `format/stboxjson/write.go`, the float format of `appendFloat`, from `src/encoding/json/encode.go` at `go1.27.2`

The Go standard library and runtime are part of every `stbox` binary.

```text
Copyright 2009 The Go Authors.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

   * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
   * Neither the name of Google LLC nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```

## mantas-done/subtitles

- Project: <https://github.com/mantas-done/subtitles>
- Licence: MIT
- Checked at: `959d1705b1ed12cbb9ddba517f8bcb814e1919a8`, and `testdata/vtt/real/mantas_styles.vtt` at `e41f318224dc9c38339c2bfcb0199e5bb788abce` with the same licence text
- Files copied from it:
  - `testdata/lenient/mantas_multiple_new_lines.vtt`, from `tests/files/vtt_with_multiple_new_lines.vtt`
  - `testdata/lenient/mantas_utf16.srt`, from `tests/files/utf16.srt`
  - `testdata/vtt/real/mantas_styles.vtt`, from `tests/files/vtt_with_styles.vtt`
  - `testdata/ttml/real/mantas_dfxp_br.dfxp`, from `tests/files/dfxp_with_different_br.dfxp`
  - `testdata/ttml/real/mantas_duplicated_ids.ttml`, from `tests/files/ttml_with_duplicated_element_ids.ttml`
  - `testdata/ttml/real/mantas_fps_multiplier.ttml`, from `tests/files/ttml_with_fps_and_multiplier_given.ttml`
  - `testdata/ttml/real/mantas_ttml2.ttml`, from `tests/files/ttml2.ttml`
- Files written for this repository in the shape of its fixtures:
  - `testdata/ttml/real/mantas_netflix_ticks.dfxp`
  - `testdata/lenient/mantas_multiple_divs.ttml`
  - `testdata/sami/real/mantas_smi.smi`, in the shape of `tests/files/smi.smi`
  - `testdata/sami/real/mantas_smi_formatted.smi`, in the shape of `tests/files/smi_formatted.smi`
  - `testdata/microdvd/real/sub_microdvd.sub`, in the shape of `tests/files/sub_microdvd.sub`
  - `testdata/microdvd/real/sub_microdvd_with_styles.sub`, in the shape of `tests/files/sub_microdvd_with_styles.sub`
  - `testdata/lrc/real/mantas-done-lrc.lrc`, in the shape of `tests/files/lrc.lrc`
  - `testdata/lrc/hours_with_text_on_next_line.lrc`, in the shape of the `testNotLrc` input in `tests/formats/LrcTest.php`
  - `testdata/csv/real/mantas_done_shape.csv`, in the shape of the `Start,End,Text` tables in `tests/formats/CsvTest.php`
  - `testdata/csv/real/headerless_frame_times.csv`, in the shape of `tests/files/csv_ansi.csv`
  - `testdata/subviewer/real/mantas_headers_shape.sub`, in the shape of `tests/files/sub_viewer_with_headers.sub`
  - `testdata/subviewer/real/mantas_plain_shape.sub`, in the shape of `tests/files/sub_viewer.sub`

```text
MIT License

Copyright (c) 2021 mantas-done

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## github.com/rivo/uniseg

- Project: <https://github.com/rivo/uniseg>
- Licence: MIT
- Checked at: `v0.4.7`

The `stbox` binary links this module.

```text
MIT License

Copyright (c) 2019 Oliver Kuederle

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## golang.org/x/text

- Project: <https://cs.opensource.google/go/x/text>
- Licence: BSD-3-Clause
- Checked at: `v0.43.0`

The `stbox` binary links this module.

```text
Copyright 2009 The Go Authors.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

   * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
   * Neither the name of Google LLC nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```

## github.com/dlclark/regexp2/v2

- Project: <https://github.com/dlclark/regexp2>
- Licence: MIT
- Checked at: `v2.8.4`

The `stbox` binary links this module. Its `ATTRIB` file credits the .NET and Go code it ports, so it follows the licence.

```text
The MIT License (MIT)

Copyright (c) Doug Clark

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

```text
============
These pieces of code were ported from dotnet/corefx:

syntax/charclass.go (from RegexCharClass.cs): ported to use the built-in Go unicode classes.  Canonicalize is 
    a direct port, but most of the other code required large changes because the C# implementation 
    used a string to represent the CharSet data structure and I cleaned that up in my implementation.

syntax/code.go (from RegexCode.cs): ported literally with various cleanups and layout to make it more Go-ish.

syntax/escape.go (from RegexParser.cs): ported Escape method and added some optimizations.  Unescape is inspired by 
    the C# implementation but couldn't be directly ported because of the lack of do-while syntax in Go.

syntax/parser.go (from RegexpParser.cs and RegexOptions.cs): ported parser struct and associated methods as 
    literally as possible. Several language differences required changes.  E.g. lack pre/post-fix increments as 
    expressions, lack of do-while loops, lack of overloads, etc.

syntax/prefix.go (from RegexFCD.cs and RegexBoyerMoore.cs): ported as literally as possible and added support
    for unicode chars that are longer than the 16-bit char in C# for the 32-bit rune in Go.

syntax/replacerdata.go (from RegexReplacement.cs): conceptually ported and re-organized to handle differences 
    in charclass implementation, and fix odd code layout between RegexParser.cs, Regex.cs, and RegexReplacement.cs.

syntax/tree.go (from RegexTree.cs and RegexNode.cs): ported literally as possible.

syntax/writer.go (from RegexWriter.cs): ported literally with minor changes to make it more Go-ish.

match.go (from RegexMatch.cs): ported, simplified, and changed to handle Go's lack of inheritence.

regexp.go (from Regex.cs and RegexOptions.cs): conceptually serves the same "starting point", but is simplified 
    and changed to handle differences in C# strings and Go strings/runes.  

replace.go (from RegexReplacement.cs): ported closely and then cleaned up to combine the MatchEvaluator and 
    simple string replace implementations.

runner.go (from RegexRunner.cs): ported literally as possible.

regexp_test.go (from CaptureTests.cs and GroupNamesAndNumbers.cs): conceptually ported, but the code was 
    manually structured like Go tests.

replace_test.go (from RegexReplaceStringTest0.cs): conceptually ported

rtl_test.go (from RightToLeft.cs): conceptually ported
---
dotnet/corefx was released under this license:

The MIT License (MIT)

Copyright (c) Microsoft Corporation

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

============
These pieces of code are copied from the Go framework:

- The overall directory structure of regexp2 was inspired by the Go runtime regexp package.
- The optimization in the escape method of syntax/escape.go is from the Go runtime QuoteMeta() func in regexp/regexp.go
- The method signatures in regexp.go are designed to match the Go framework regexp methods closely
- func regexp2.MustCompile and func quote are almost identifical to the regexp package versions
- BenchmarkMatch* and TestProgramTooLong* funcs in regexp_performance_test.go were copied from the framework 
    regexp/exec_test.go
---
The Go framework was released under this license:

Copyright (c) 2012 The Go Authors. All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

   * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
   * Neither the name of Google Inc. nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

============
Some test data were gathered from the Mono project.

regexp_mono_test.go: ported from https://github.com/mono/mono/blob/master/mcs/class/System/Test/System.Text.RegularExpressions/PerlTrials.cs
---
Mono tests released under this license:

Permission is hereby granted, free of charge, to any person obtaining
a copy of this software and associated documentation files (the
"Software"), to deal in the Software without restriction, including
without limitation the rights to use, copy, modify, merge, publish,
distribute, sublicense, and/or sell copies of the Software, and to
permit persons to whom the Software is furnished to do so, subject to
the following conditions:

The above copyright notice and this permission notice shall be
included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
```

## BYVoid/OpenCC

- Project: <https://github.com/BYVoid/OpenCC>
- Licence: Apache-2.0
- Checked at: `8cf737a4e193db27d675b2ce9867d84aace33745`
- Data used: `internal/textenc/han_traditional.txt`, the Han list of the code page detection, from `data/dictionary/STCharacters.txt` and `data/dictionary/TWVariants.txt`, the tables of Simplified characters to Traditional characters and of Traditional characters to their Taiwan forms
- Changed: only the Traditional forms of the 661 characters in `internal/textenc/han_simplified.txt`, a list written for subtitle-toolbox-php, are kept. Rare forms were removed by hand, for example `箇` for `个` and `喫` for `吃`. The result is one line of 688 characters, not a table.

The project has no `NOTICE` file at this commit.

```text
Apache License
Version 2.0, January 2004
http://www.apache.org/licenses/

TERMS AND CONDITIONS FOR USE, REPRODUCTION, AND DISTRIBUTION

1. Definitions.

"License" shall mean the terms and conditions for use, reproduction, and distribution as defined by Sections 1 through 9 of this document.

"Licensor" shall mean the copyright owner or entity authorized by the copyright owner that is granting the License.

"Legal Entity" shall mean the union of the acting entity and all other entities that control, are controlled by, or are under common control with that entity. For the purposes of this definition, "control" means (i) the power, direct or indirect, to cause the direction or management of such entity, whether by contract or otherwise, or (ii) ownership of fifty percent (50%) or more of the outstanding shares, or (iii) beneficial ownership of such entity.

"You" (or "Your") shall mean an individual or Legal Entity exercising permissions granted by this License.

"Source" form shall mean the preferred form for making modifications, including but not limited to software source code, documentation source, and configuration files.

"Object" form shall mean any form resulting from mechanical transformation or translation of a Source form, including but not limited to compiled object code, generated documentation, and conversions to other media types.

"Work" shall mean the work of authorship, whether in Source or Object form, made available under the License, as indicated by a copyright notice that is included in or attached to the work (an example is provided in the Appendix below).

"Derivative Works" shall mean any work, whether in Source or Object form, that is based on (or derived from) the Work and for which the editorial revisions, annotations, elaborations, or other modifications represent, as a whole, an original work of authorship. For the purposes of this License, Derivative Works shall not include works that remain separable from, or merely link (or bind by name) to the interfaces of, the Work and Derivative Works thereof.

"Contribution" shall mean any work of authorship, including the original version of the Work and any modifications or additions to that Work or Derivative Works thereof, that is intentionally submitted to Licensor for inclusion in the Work by the copyright owner or by an individual or Legal Entity authorized to submit on behalf of the copyright owner. For the purposes of this definition, "submitted" means any form of electronic, verbal, or written communication sent to the Licensor or its representatives, including but not limited to communication on electronic mailing lists, source code control systems, and issue tracking systems that are managed by, or on behalf of, the Licensor for the purpose of discussing and improving the Work, but excluding communication that is conspicuously marked or otherwise designated in writing by the copyright owner as "Not a Contribution."

"Contributor" shall mean Licensor and any individual or Legal Entity on behalf of whom a Contribution has been received by Licensor and subsequently incorporated within the Work.

2. Grant of Copyright License. Subject to the terms and conditions of this License, each Contributor hereby grants to You a perpetual, worldwide, non-exclusive, no-charge, royalty-free, irrevocable copyright license to reproduce, prepare Derivative Works of, publicly display, publicly perform, sublicense, and distribute the Work and such Derivative Works in Source or Object form.

3. Grant of Patent License. Subject to the terms and conditions of this License, each Contributor hereby grants to You a perpetual, worldwide, non-exclusive, no-charge, royalty-free, irrevocable (except as stated in this section) patent license to make, have made, use, offer to sell, sell, import, and otherwise transfer the Work, where such license applies only to those patent claims licensable by such Contributor that are necessarily infringed by their Contribution(s) alone or by combination of their Contribution(s) with the Work to which such Contribution(s) was submitted. If You institute patent litigation against any entity (including a cross-claim or counterclaim in a lawsuit) alleging that the Work or a Contribution incorporated within the Work constitutes direct or contributory patent infringement, then any patent licenses granted to You under this License for that Work shall terminate as of the date such litigation is filed.

4. Redistribution. You may reproduce and distribute copies of the Work or Derivative Works thereof in any medium, with or without modifications, and in Source or Object form, provided that You meet the following conditions:

   1. You must give any other recipients of the Work or Derivative Works a copy of this License; and

   2. You must cause any modified files to carry prominent notices stating that You changed the files; and

   3. You must retain, in the Source form of any Derivative Works that You distribute, all copyright, patent, trademark, and attribution notices from the Source form of the Work, excluding those notices that do not pertain to any part of the Derivative Works; and

   4. If the Work includes a "NOTICE" text file as part of its distribution, then any Derivative Works that You distribute must include a readable copy of the attribution notices contained within such NOTICE file, excluding those notices that do not pertain to any part of the Derivative Works, in at least one of the following places: within a NOTICE text file distributed as part of the Derivative Works; within the Source form or documentation, if provided along with the Derivative Works; or, within a display generated by the Derivative Works, if and wherever such third-party notices normally appear. The contents of the NOTICE file are for informational purposes only and do not modify the License. You may add Your own attribution notices within Derivative Works that You distribute, alongside or as an addendum to the NOTICE text from the Work, provided that such additional attribution notices cannot be construed as modifying the License.

You may add Your own copyright statement to Your modifications and may provide additional or different license terms and conditions for use, reproduction, or distribution of Your modifications, or for any such Derivative Works as a whole, provided Your use, reproduction, and distribution of the Work otherwise complies with the conditions stated in this License.

5. Submission of Contributions. Unless You explicitly state otherwise, any Contribution intentionally submitted for inclusion in the Work by You to the Licensor shall be under the terms and conditions of this License, without any additional terms or conditions. Notwithstanding the above, nothing herein shall supersede or modify the terms of any separate license agreement you may have executed with Licensor regarding such Contributions.

6. Trademarks. This License does not grant permission to use the trade names, trademarks, service marks, or product names of the Licensor, except as required for reasonable and customary use in describing the origin of the Work and reproducing the content of the NOTICE file.

7. Disclaimer of Warranty. Unless required by applicable law or agreed to in writing, Licensor provides the Work (and each Contributor provides its Contributions) on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied, including, without limitation, any warranties or conditions of TITLE, NON-INFRINGEMENT, MERCHANTABILITY, or FITNESS FOR A PARTICULAR PURPOSE. You are solely responsible for determining the appropriateness of using or redistributing the Work and assume any risks associated with Your exercise of permissions under this License.

8. Limitation of Liability. In no event and under no legal theory, whether in tort (including negligence), contract, or otherwise, unless required by applicable law (such as deliberate and grossly negligent acts) or agreed to in writing, shall any Contributor be liable to You for damages, including any direct, indirect, special, incidental, or consequential damages of any character arising as a result of this License or out of the use or inability to use the Work (including but not limited to damages for loss of goodwill, work stoppage, computer failure or malfunction, or any and all other commercial damages or losses), even if such Contributor has been advised of the possibility of such damages.

9. Accepting Warranty or Additional Liability. While redistributing the Work or Derivative Works thereof, You may choose to offer, and charge a fee for, acceptance of support, warranty, indemnity, or other liability obligations and/or rights consistent with this License. However, in accepting such obligations, You may act only on Your own behalf and on Your sole responsibility, not on behalf of any other Contributor, and only if You agree to indemnify, defend, and hold each Contributor harmless for any liability incurred by, or claims asserted against, such Contributor by reason of your accepting any such warranty or additional liability.

END OF TERMS AND CONDITIONS
```

## asticode/go-astisub

- Project: <https://github.com/asticode/go-astisub>
- Licence: MIT
- Checked at: `f285923a0c8d5b2ecce67d8eaf9ed26cce16e5b0`, and `58297a613f70cac3d29e5ce52a7215a3b4038b40` and `b6b18718ddb6ee0da08772d8ab310c9c3d2d0459` for the SubRip files, and `7671e72e9d47287d6f8ded80a004560b63a2ead5` for the TTML files, with the same licence text
- Files copied from it:
  - `testdata/ttml/real/astisub_breaklines.ttml`, from `testdata/example-in-breaklines.ttml`
  - `testdata/ttml/real/astisub_merging_style.ttml`, from `testdata/example-in-merging-style.ttml`
- Files written for this repository in the shape of its fixtures:
  - `testdata/ttml/real/astisub_smpte.ttml`
  - `testdata/vtt/real/astisub_carriage_return.vtt`
  - `testdata/vtt/real/astisub_html_entities.vtt`
  - `testdata/srt/real/own_styled.srt`
  - `testdata/srt/real/own_timestamp_without_millis.srt`

```text
MIT License

Copyright (c) 2016 Quentin Renard

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## tkarabela/pysubs2

- Project: <https://github.com/tkarabela/pysubs2>
- Licence: MIT
- Checked at: `ae3c520a8aa13130f36d84df02ce15a843bc9bdb`
- Files written for this repository in the shape of its fixtures:
  - `testdata/json/real/pysubs2_regions_3x.json`
  - `testdata/ttml/real/pysubs2_regions.ttml`
  - `testdata/sami/real/pysubs2_source_id.smi`, in the shape of `SAMI_INPUT1` in `tests/formats/test_sami.py`
  - `testdata/mpl2/real/pysubs2_shape_bom.txt`, in the shape of `tests/formats/test_mpl2.py`

```text
Copyright (c) 2014-2026 Tomas Karabela

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
```

## W3C specification examples

- Project: <https://www.w3.org/TR/webvtt1/>, <https://www.w3.org/TR/ttml1/>, <https://www.w3.org/TR/ttml-imsc1.1/> and <https://www.w3.org/TR/2006/CR-ttaf1-dfxp-20061116/>
- Licence: W3C Software and Document License, 2023 version, <https://www.w3.org/copyright/software-license-2023/>
- Files copied from it:
  - `testdata/json/real/w3c_cue_settings_3x.json`
  - `testdata/vtt/real/w3c_chapters.vtt`
  - `testdata/vtt/real/w3c_comments.vtt`
  - `testdata/vtt/real/w3c_cue_settings.vtt`
  - `testdata/vtt/real/w3c_identifiers.vtt`
  - `testdata/vtt/real/w3c_regions.vtt`
  - `testdata/vtt/real/w3c_styles.vtt`
  - `testdata/vtt/real/w3c_timestamps.vtt`
  - `testdata/forced/w3c_imsc11_forced.ttml`
  - `testdata/ttml/real/w3c_dfxp_timing.dfxp`
  - `testdata/ttml/real/w3c_imsc11_frames.ttml`
  - `testdata/ttml/real/w3c_imsc11_line_gaps.ttml`
  - `testdata/ttml/real/w3c_imsc11_paragraphs.ttml`
  - `testdata/ttml/real/w3c_imsc11_regions.ttml`
  - `testdata/ttml/real/w3c_ttml1_cells.ttml`
  - `testdata/ttml/real/w3c_ttml1_timed_spans.ttml`
  - `testdata/ttml/real/w3c_ttml1_timing.ttml`

This repository includes material copied from or derived from the specifications above. Copyright (c) World Wide Web Consortium. https://www.w3.org/copyright/software-license-2023/

```text
Permission to copy, modify, and distribute this work, with or without modification, for any purpose and without fee or royalty is hereby granted, provided that you include the following on ALL copies of the work or portions thereof, including modifications:

- The full text of this NOTICE in a location viewable to users of the redistributed or derivative work.
- Any pre-existing intellectual property disclaimers, notices, or terms and conditions. If none exist, the W3C software and document short notice should be included.
- Notice of any changes or modifications, through a copyright statement on the new code or document such as "This software or document includes material copied from or derived from [title and URI of the W3C document]. Copyright (c) [$year-of-document] World Wide Web Consortium. https://www.w3.org/copyright/software-license-2023/"

Disclaimers

THIS WORK IS PROVIDED "AS IS," AND COPYRIGHT HOLDERS MAKE NO REPRESENTATIONS OR WARRANTIES, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO, WARRANTIES OF MERCHANTABILITY OR FITNESS FOR ANY PARTICULAR PURPOSE OR THAT THE USE OF THE SOFTWARE OR DOCUMENT WILL NOT INFRINGE ANY THIRD PARTY PATENTS, COPYRIGHTS, TRADEMARKS OR OTHER RIGHTS.

COPYRIGHT HOLDERS WILL NOT BE LIABLE FOR ANY DIRECT, INDIRECT, SPECIAL OR CONSEQUENTIAL DAMAGES ARISING OUT OF ANY USE OF THE SOFTWARE OR DOCUMENT.

The name and trademarks of copyright holders may NOT be used in advertising or publicity pertaining to the work without specific, written prior permission. Title to copyright in this work will at all times remain with copyright holders.
```

## Alhadis/language-subtitles

- Project: <https://github.com/Alhadis/language-subtitles>
- Licence: ISC
- Checked at: `9acc0aa2c232a7b06aa0e8c1058ee5be81a98837`
- Files copied from it:
  - `testdata/srt/real/language_subtitles_dots_tester.srt`, from `samples/srt/stress/Tester for dots instead of commas in timestamps.srt`
  - `testdata/srt/real/language_subtitles_ssa_extensions.srt`, from `samples/srt/stress/SSA Extensions.srt`

```text
Copyright (c) 2022-2026, John Gardner

Permission to use, copy, modify, and/or distribute this software for any
purpose with or without fee is hereby granted, provided that the above
copyright notice and this permission notice appear in all copies.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
```

## dioKaratzas/swift-subtitle-kit

- Project: <https://github.com/dioKaratzas/swift-subtitle-kit>
- Licence: MIT
- Checked at: `728d62dac9fbda7fabd299eed3673ce36c85c267`
- Files copied from it:
  - `testdata/srt/real/swift_subtitle_kit_embedded_bom.srt`, from `Tests/SubtitleKitTests/Fixtures/embedded-bom-srt-fixture.srt`

```text
MIT License

Copyright (c) 2026 Dionysis Karatzas

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## sandflow/ttconv

- Project: <https://github.com/sandflow/ttconv>
- Licence: BSD-2-Clause
- Checked at: `1fed6abf555ca1d5ae2c839ad3c1625f15c9a96c`
- Files copied from it, with `begin` and `end` added to each `<p>`:
  - `testdata/ttml/real/ttconv_referential_styling.ttml`, from `src/test/resources/ttml/referential_styling.ttml`
  - `testdata/ttml/real/ttconv_lwsp_default.ttml`, from `src/test/resources/ttml/lwsp_default.ttml`
  - `testdata/ttml/real/ttconv_lwsp_preserve.ttml`, from `src/test/resources/ttml/lwsp_preserve.ttml`

```text
Copyright (c) 2020, Sandflow Consulting LLC

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice, this
   list of conditions and the following disclaimer.
2. Redistributions in binary form must reproduce the above copyright notice,
   this list of conditions and the following disclaimer in the documentation
   and/or other materials provided with the distribution.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE LIABLE FOR
ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
(INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```

## papnkukn/subsrt

- Project: <https://github.com/papnkukn/subsrt>
- Licence: MIT
- Checked at: `478e54390b74859b3643b80ff988113557ff80d8`
- Files copied from it:
  - `testdata/microdvd/real/subsrt_sample.sub`, from `test/fixtures/sample.sub`
- Files written for this repository in the shape of its fixtures:
  - `testdata/sami/real/subsrt_sample.smi`, in the shape of `test/fixtures/sample.smi`
  - `testdata/sbv/real/subsrt_shape.sbv`, in the shape of `test/fixtures/sample.sbv` at `46deebd95703be9b6a76eb3c1aaf915713f30394`
  - `testdata/lrc/real/subsrt-sample.lrc`, in the shape of `test/fixtures/sample.lrc` at `46deebd95703be9b6a76eb3c1aaf915713f30394`

```text
MIT License

Copyright (c) 2017

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## SubtitleEdit/subtitleedit

- Project: <https://github.com/SubtitleEdit/subtitleedit>
- Licence: MIT
- Checked at: `5b9ee8baf08c472c7a74fbc337446b656dedabb4`
- Code adapted from it:
  - `hearingimpaired`, the hearing-impaired rules, from `src/libse/Forms/RemoveTextForHI.cs` at `5b9ee8baf08c472c7a74fbc337446b656dedabb4`

```text
MIT License

Copyright (c) 2026 Nikolaj Olsson

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```
