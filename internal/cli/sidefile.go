package cli

import (
	"errors"
	"os"

	stbox "github.com/yama6a/subtitle-toolbox-go/v2"
)

// sideFileUnknownFormat replaces the --from advice of unknownFormatMessage, because --from does not apply to a side file.
const sideFileUnknownFormat = "Format detection found no subtitle format. Write it to a subtitle file with convert --from FORMAT first."

// readSideFile reads a subtitle that an option names, such as the reference of sync. Content detection picks the
// format, so --from does not apply, and the other read options of the call do. It prints the warnings with the path
// as the label. A failure is a *fileError, which stops the run.
func (r *runner) readSideFile(path string) (*stbox.Subtitle, error) {
	opts := r.readOptions
	opts.Format = ""
	if st, err := os.Stat(r.c.path(path)); err != nil || !st.Mode().IsRegular() {
		return nil, &fileError{msg: path + ": " + missingFileMessage}
	}
	sub, info, err := stbox.Load(r.c.path(path), opts)
	if err != nil {
		msg := path + ": " + message(err)
		if errors.Is(err, stbox.ErrUnknownFormat) {
			msg = path + ": " + sideFileUnknownFormat
		}
		if _, ok := errors.AsType[*stbox.ParseError](err); ok && !opts.Lenient {
			msg += "\n" + lenientHint
		}
		return nil, &fileError{msg: msg}
	}
	r.printWarnings(&input{arg: path, sub: sub, info: info})
	return sub, nil
}

// readSideText returns the content of a file that an option names. A file that is missing or is no regular file, or
// that cannot be read, is a *fileError that stops the run.
func readSideText(c *console, name string) ([]byte, error) {
	path := c.path(name)
	if st, err := os.Stat(path); err != nil || !st.Mode().IsRegular() {
		return nil, &fileError{msg: name + ": " + missingFileMessage}
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, &fileError{msg: name + ": Cannot read the file."}
	}
	return content, nil
}
