# Release runbook

The `release` workflow releases on every merge to `main` whose pull request carries `patch`, `minor` or `major`. The README section "Releases" says what each label does. [compatibility.md](../compatibility.md) says which label a change needs.

## Normal release

1. Label the pull request `patch` or `minor`.

   ```bash
   gh pr edit <number> -R yama6a/subtitle-toolbox-go --add-label minor
   ```

2. Merge the pull request after every required check is green.
3. Open the `release` workflow run of the merge commit.

   ```bash
   gh run list -R yama6a/subtitle-toolbox-go --workflow release --branch main --limit 1
   gh run watch <run-id> -R yama6a/subtitle-toolbox-go --exit-status
   ```

   Expected: the jobs `version`, `archives`, `image` and `release` are green.

4. Check the assets of the release.

   ```bash
   gh release view v1.2.4 -R yama6a/subtitle-toolbox-go --json assets --jq '.assets[].name'
   ```

   Expected: 7 names, the 6 archives and `stbox_1.2.4_checksums.txt`.

5. Do the steps of [After any release](#after-any-release).

## Major release

A major release N opens a new line and moves the module path to `/vN`. Only the pull request of the major issue of that line carries `major`. The 1.0.0 pull request carries `major` too, and it keeps the module path without a suffix.

### Checks before the major label

Do every step on the last commit of `main`. For a major N of 2 or more, do step 8 on the branch of the major pull request, after the [steps for a major N of 2 or more](#steps-for-a-major-n-of-2-or-more). Set `major` only when every step gives the expected result.

1. Check that every issue of the scope table of the major issue is closed.

   ```bash
   gh issue view <number> -R yama6a/subtitle-toolbox-go --json state --jq .state
   ```

   Expected: `CLOSED` for each issue.

2. List the tags of the line. For 1.0.0, list every `v*` tag.

   ```bash
   git ls-remote --tags origin 'v*'
   ```

   Expected for 1.0.0: no output. Expected for N of 2 or more: no tag `vN.*`.

3. Check that the 12 required checks are green on the last commit of `main`. The command prints only the required checks. It ignores the other runs, such as `renovate / renovate`.

   ```bash
   gh api 'repos/yama6a/subtitle-toolbox-go/commits/main/check-runs?per_page=100' --jq '
     ["release label","version check","gorelease","go / go","cross-compile","fuzz","bench","smoke","docker / docker","shell","yaml","renovate-config"] as $required
     | .check_runs[] | select(.name as $n | $required | index($n)) | "\(.name) \(.conclusion)"' | sort
   ```

   Expected: 12 lines. `release label`, `version check` and `gorelease` end in `skipped`, because they run on pull requests only. The other 9 end in `success`.

   ```text
   bench success
   cross-compile success
   docker / docker success
   fuzz success
   go / go success
   gorelease skipped
   release label skipped
   renovate-config success
   shell success
   smoke success
   version check skipped
   yaml success
   ```

   If the list differs from the branch protection of `main`, update it. Read the protection with:

   ```bash
   gh api repos/yama6a/subtitle-toolbox-go/branches/main/protection -q '.required_status_checks.checks[].context'
   ```

4. Run the read golden runner.

   ```bash
   out=$(mktemp)
   go test -count=1 -run '^TestReadGoldens$' -v . > "$out"
   grep -o -- '--- PASS: TestReadGoldens/[a-z]*' "$out" | sort | uniq -c
   grep -c -- '--- SKIP' "$out"
   ```

   Expected for 1.0.0: 6 cases in `json`, 19 in `srt` and 30 in `vtt`, 55 in total. The skip count is 0.

5. Rewrite the write goldens.

   ```bash
   make golden && git status --porcelain
   ```

   Expected: no output from `git status`.

6. Run every fuzz target for 5 minutes. At 1.0.0 the module has 16 targets, so this takes about 80 minutes.

   ```bash
   make fuzz FUZZTIME=5m
   ```

   Expected: exit code 0, and no new file under a `testdata/fuzz/` folder.

7. Build the archives and run the smoke test.

   ```bash
   rm -rf dist && make dist VERSION=N.0.0 && make smoke VERSION=N.0.0
   ```

   Expected: the last line is `smoke test passed: N.0.0`.

8. Check the module path and the modules.

   ```bash
   go mod edit -json | jq -r '.Module.Path, (.Require[] | "\(.Path) \(.Indirect // false)")'
   ```

   Expected for 1.0.0: the module path `github.com/yama6a/subtitle-toolbox-go`. Expected for N of 2 or more: `github.com/yama6a/subtitle-toolbox-go/vN`. Each line that ends in `false` names an allowed module of `CLAUDE.md` or testify.

9. Check that `golang.org/x/text` needs no newer Go than the module.

   ```bash
   go list -m -json golang.org/x/text | jq -r .GoVersion
   go mod edit -json | jq -r .Go
   ```

   Expected: the first version is not above the second.

10. Review the API of each package.

    ```bash
    for pkg in $(go list ./... | grep -v -e /internal/ -e /cmd/); do go doc -all "$pkg"; done
    ```

    Expected: only identifiers that are meant as API. Nothing that the PHP code marks `@internal` is exported. The names follow the section "API rules" of `CONTRIBUTING.md`.

11. Check the docs. Every page of `docs/README.md` holds its content, and the docs tests pass.

    ```bash
    go test -run '^TestDocs|^TestReadme' .
    ```

    Expected: `ok`.

12. Check that `THIRD_PARTY_NOTICES.md` has a section for every copied file and every linked module, and run the repo tests.

    ```bash
    go test -count=1 -run '^(TestEvery|TestUncredited|TestUnmatchedRows|TestSourcesNames)' .
    ```

    Expected: `ok`.

13. Check that the owner has answered every compatibility decision of the major issue, and that `compatibility.md` states each answer. For 1.0.0 these are D2 and D3 of [#7](https://github.com/yama6a/subtitle-toolbox-go/issues/7).

    ```bash
    gh issue view 7 -R yama6a/subtitle-toolbox-go --json body --jq .body | grep -o '^\*\*Decision (D[23])\*\*'
    grep -n -e '^- \*\*Option defaults\*\*' -e '^## `==` on structs' docs/compatibility.md
    ```

    Expected: the 2 lines `**Decision (D2)**` and `**Decision (D3)**`, then the 2 lines of `compatibility.md` that state them.

14. Set `major` on the pull request and merge it. For a major N of 2 or more, do the [steps for a major N of 2 or more](#steps-for-a-major-n-of-2-or-more) first.

    ```bash
    gh pr edit <number> -R yama6a/subtitle-toolbox-go --add-label major
    gh pr merge <number> -R yama6a/subtitle-toolbox-go --squash
    ```

    Expected: the `release` workflow starts on the merge commit. Then do the steps of [After any release](#after-any-release).

### Steps for a major N of 2 or more

`version check` fails until `go.mod` and every import agree with the `major` label.

1. Change the module path.

   ```bash
   go mod edit -module github.com/yama6a/subtitle-toolbox-go/vN
   ```

2. Rewrite every import of the module to `/vN`.

   ```bash
   grep -rl --include='*.go' '"github.com/yama6a/subtitle-toolbox-go' . \
     | xargs sed -i -E 's#"github.com/yama6a/subtitle-toolbox-go(/v[0-9]+)?([/"])#"github.com/yama6a/subtitle-toolbox-go/vN\2#g'
   go build ./... && go test ./...
   ```

   Expected: both commands pass.

3. Add `docs/upgrade-N.0.md`, with the old call and the new call for each break. Add its row to `docs/README.md`.
4. Change the `go get` and `go install` lines in `README.md` and `docs/cli.md` to `/vN`.
5. Do step 8 of the [checks before the major label](#checks-before-the-major-label) on the branch of the pull request.

   Expected: the module path `github.com/yama6a/subtitle-toolbox-go/vN`.

6. Do step 14 of the checks before the major label.

## After any release

Replace `1.0.0` with the version of the release.

1. Open the `release` workflow run of the merge commit.

   Expected: `version`, `archives`, `image` and `release` are green, and `version` prints `next tag v1.0.0`.

2. View the release.

   ```bash
   gh release view v1.0.0 -R yama6a/subtitle-toolbox-go
   ```

   Expected: 6 archives and `stbox_1.0.0_checksums.txt`.

3. Download one archive and the checksum file. Check them, extract the archive and run the binary.

   ```bash
   gh release download v1.0.0 -R yama6a/subtitle-toolbox-go -p 'stbox_1.0.0_linux_amd64.tar.gz' -p 'stbox_1.0.0_checksums.txt'
   sha256sum --ignore-missing -c stbox_1.0.0_checksums.txt
   tar -xzf stbox_1.0.0_linux_amd64.tar.gz stbox
   ./stbox --version
   ```

   Expected: `stbox_1.0.0_linux_amd64.tar.gz: OK` and `1.0.0`.

4. Pull the image without a login.

   ```bash
   DOCKER_CONFIG="$(mktemp -d)" docker pull ghcr.io/yama6a/subtitle-toolbox-go:1.0.0
   ```

   Expected: the pull succeeds. A permission error means that GHCR created the package as private. Set it to public in the package settings and link it to the repo.

5. Run the image.

   ```bash
   docker run --rm ghcr.io/yama6a/subtitle-toolbox-go:latest --version
   ```

   Expected: `1.0.0`.

6. Verify the provenance of the image.

   ```bash
   gh attestation verify oci://ghcr.io/yama6a/subtitle-toolbox-go:1.0.0 -R yama6a/subtitle-toolbox-go
   ```

   Expected: a verified provenance.

7. Install the binary through the Go module proxy in an empty folder.

   ```bash
   cd "$(mktemp -d)"
   GOPROXY=https://proxy.golang.org GOBIN="$PWD" go install github.com/yama6a/subtitle-toolbox-go/cmd/stbox@v1.0.0
   ./stbox --version
   ```

   Expected: `1.0.0`. This also makes the proxy fetch the version.

8. Open `https://pkg.go.dev/github.com/yama6a/subtitle-toolbox-go@v1.0.0`.

   Expected: the docs show, and the licence reads MIT. pkg.go.dev hides the docs when it does not detect a redistributable licence.

9. For a major release, edit the release notes. Put a hand-written block above the generated notes. It names the formats and the commands of the line, the 3 install forms, and links `docs/compatibility.md` at the tag.

   Write the block to `block.md`. Then run:

   ```bash
   gh release view v1.0.0 -R yama6a/subtitle-toolbox-go --json body --jq .body > generated.md
   cat block.md generated.md > notes.md
   gh release edit v1.0.0 -R yama6a/subtitle-toolbox-go --notes-file notes.md
   ```

## Gotchas

- **Tags are permanent**: the Go module proxy keeps a version forever, also after a tag delete. Never delete or move a `v*` tag. A broken v1.0.0 gets a fix in v1.0.1. If nobody may use v1.0.0, v1.0.1 adds `retract v1.0.0` to `go.mod`.
- **Only one pull request carries `major` before 1.0.0**: `version check` cannot tell the 1.0.0 pull request from another `major` pull request before the first tag.
- **API check in CI**: the `gorelease` job of [#158](https://github.com/yama6a/subtitle-toolbox-go/issues/158) compares the API of a pull request with the newest `v*` tag. It fails when the release label is weaker than the change. The pin of `golang.org/x/exp` is in [`.build/gorelease.sh`](../../.build/gorelease.sh).
- **Open pull requests after the first tag**: before `v1.0.0`, `version check` fails every pull request labelled `patch` or `minor`. The check reruns only on a push or a label change. After the tag, rerun the failed job of each such pull request:

  ```bash
  gh run list -R yama6a/subtitle-toolbox-go --workflow ci --branch <branch> --limit 1
  gh run rerun <run-id> -R yama6a/subtitle-toolbox-go --failed
  ```

  Expected: `version check` passes.
