package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	stbox "github.com/yama6a/subtitle-toolbox-go/v5"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/extfallback"
)

// expand turns the input arguments into input files: "-", the subtitle files of a directory, the matches of a
// glob, or the path as given. It removes inputs whose real path came before.
func (r *runner) expand(args []string) ([]string, error) {
	var inputs []string
	for _, arg := range args {
		files, err := r.expandArg(arg)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, files...)
	}
	seen := make(map[string]bool, len(inputs))
	unique := inputs[:0]
	for _, in := range inputs {
		key := in
		if in != stdinArg {
			if realPath, err := filepath.EvalSymlinks(r.c.path(in)); err == nil {
				key, _ = filepath.Abs(realPath)
			}
		}
		if !seen[key] {
			seen[key] = true
			unique = append(unique, in)
		}
	}
	return unique, nil
}

func (r *runner) expandArg(arg string) ([]string, error) {
	if arg == stdinArg {
		return []string{arg}, nil
	}
	st, err := os.Stat(r.c.path(arg))
	switch {
	case err == nil && st.IsDir():
		files := r.directoryFiles(arg)
		if len(files) == 0 {
			return nil, usage("The directory " + arg + " holds no file with a known subtitle extension.")
		}
		return files, nil
	case err == nil || !strings.ContainsAny(arg, "*?["):
		return []string{arg}, nil
	default:
		return r.glob(arg), nil
	}
}

// directoryFiles returns the files of dir, sorted by name, that hold a readable format by their extension. A
// .txt file stays when content detection finds a format in it. A .sub file that the read of an .idx file of the
// folder opens is the second file of a VobSub pair. The .idx file is the input.
func (r *runner) directoryFiles(dir string) []string {
	entries, err := os.ReadDir(r.c.path(dir))
	if err != nil {
		return nil
	}
	base := strings.TrimRight(dir, `/\`)
	var files []string
	for _, e := range withoutVobSubData(entries) {
		path := base + "/" + e.Name()
		if st, err := os.Stat(r.c.path(path)); err != nil || !st.Mode().IsRegular() {
			continue
		}
		f, ok := stbox.FormatFromPath(e.Name())
		if ok && f.CanRead() {
			files = append(files, path)
			continue
		}
		if strings.ToLower(filepath.Ext(e.Name())) == ".txt" {
			if _, found := r.detect(r.c.path(path)); found {
				files = append(files, path)
			}
		}
	}
	return files
}

// glob returns the regular files that match pattern, relative to the directory of the call again. Without a
// match, the pattern stays as the input, and the read fails later as a missing file.
func (r *runner) glob(pattern string) []string {
	matches, _ := filepath.Glob(r.c.path(pattern))
	var files []string
	for _, m := range matches {
		if st, err := os.Stat(m); err != nil || !st.Mode().IsRegular() {
			continue
		}
		if r.c.dir != "" && !filepath.IsAbs(pattern) {
			if rel, err := filepath.Rel(r.c.dir, m); err == nil {
				m = rel
			}
		}
		files = append(files, m)
	}
	if len(files) == 0 {
		return []string{pattern}
	}
	return files
}

// vobSubPartners returns the other file of the pair of each VobSub input that exists. A VobSub read opens both files,
// so no output may overwrite either.
func (r *runner) vobSubPartners(inputs []string) []string {
	var partners []string
	for _, arg := range inputs {
		if arg == stdinArg || r.readOptions.Format != stbox.VobSub && !strings.EqualFold(filepath.Ext(arg), ".idx") {
			continue
		}
		if r.peek(arg) != stbox.VobSub {
			continue
		}
		candidates, _ := extfallback.Partners(arg)
		for _, name := range candidates {
			if st, err := os.Stat(r.c.path(name)); err == nil && st.Mode().IsRegular() {
				partners = append(partners, name)
				break
			}
		}
	}
	return partners
}

// withoutVobSubData returns entries without the .sub files that the read of an .idx file in entries uses. The names come
// from extfallback.Partners, the rule of the read, so a hidden file is never missing for the .idx file.
func withoutVobSubData(entries []os.DirEntry) []os.DirEntry {
	hidden := make(map[string]bool)
	for _, e := range entries {
		if strings.EqualFold(filepath.Ext(e.Name()), ".idx") {
			candidates, _ := extfallback.Partners(e.Name())
			for _, c := range candidates {
				hidden[c] = true
			}
		}
	}
	return slices.DeleteFunc(slices.Clone(entries), func(e os.DirEntry) bool { return hidden[e.Name()] })
}
