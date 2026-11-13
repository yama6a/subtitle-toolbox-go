package transcript_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func FuzzReadHTML(f *testing.F) {
	entries, err := os.ReadDir(testfiles.Path(f, "html/real"))
	if err != nil {
		f.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".html" {
			f.Add(testfiles.Read(f, "html/real/"+e.Name()))
		}
	}
	for _, seed := range []string{
		"<time>0:00</time><p>Hi.</p>\n<time>0:61</time><p>Oops.</p>",
		"<cite>A :</cite><p>x<br/>y</p><time>1:02:03.5</time>",
		"<time>99999999999999999999:00:04</time><p>x</p>",
		"<p><p><p ><cite <time", "<TIME>0:01</TIME ><P>&amp;</P\n>", ``,
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			common := model.CommonReadOptions{Lenient: lenient}
			s, _, err := transcript.ReadHTML(bytes.NewReader(content), common, transcript.ReadOptions{})
			if err != nil {
				var perr *model.ParseError
				if !errors.As(err, &perr) {
					t.Fatalf("ReadHTML returned %v, which is no *ParseError", err)
				}
				continue
			}
			checkCues(t, s)
			var out bytes.Buffer
			if err := transcript.WriteHTML(&out, s, model.CommonWriteOptions{}, transcript.WriteOptions{}); err != nil {
				if !errors.Is(err, model.ErrUnwritable) {
					t.Fatalf("WriteHTML returned %v, which does not match ErrUnwritable", err)
				}
				continue
			}
			if _, _, err := transcript.ReadHTML(bytes.NewReader(out.Bytes()), common, transcript.ReadOptions{}); err != nil {
				t.Fatalf("the output %s does not read: %v", out.Bytes(), err)
			}
		}
	})
}
