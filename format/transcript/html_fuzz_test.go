package transcript_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

func FuzzReadHTML(f *testing.F) {
	formattest.AddSeeds(f, "html/real/*.html")
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
			s, warnings, err := transcript.ReadHTML(bytes.NewReader(content), common, transcript.ReadOptions{})
			if !formattest.CheckRead(t, s, warnings, err) {
				continue
			}
			checkWords(t, s)
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
