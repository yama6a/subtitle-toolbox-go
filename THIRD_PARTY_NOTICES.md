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
