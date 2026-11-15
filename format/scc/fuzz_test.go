package scc_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/scc"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func FuzzRead(f *testing.F) {
	formattest.AddSeeds(f, "scc/real/*.scc")
	for _, seed := range []string{
		popOnHello,
		"Scenarist_SCC V1.0\n",
		"Scenarist_SCC V1.0\n\n00:00:01:00\t942c 94c\n",
		"Scenarist_SCC V1.0\r\n\r\n00:00:01:00\t942c\n\n00:00:02:00\t94zz\n",
		"Scenarist_SCC V1.0\n\n99:99:99;99\t9420 9420 9470 9470 c8e9 942f 942f\n00:00:00;00\t942c",
		"Scenarist_SCC V1.0\n\n942c 942c\n",
		"Scenarist_SCC V2.0\n",
		"Scenarist_SCC V1.0\n\n100000:05:59;29\t9420 9420 94ae 94ae 9452 9452 c8e9 942f 942f\n",
		"Scenarist_SCC V1.0\n\n100001:00:00:00\t9420 9470 c8e9 942f\n\n99999:59:59;29\t942c\n",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := scc.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, scc.ReadOptions{})
			if !formattest.CheckRead(t, s, warnings, err) {
				continue
			}
			for _, fit := range []bool{false, true} {
				if _, err := scc.Write(io.Discard, s, model.CommonWriteOptions{}, scc.WriteOptions{Fit: fit}); err != nil && !errors.Is(err, model.ErrUnwritable) {
					t.Fatalf("Write returned %v, which does not match ErrUnwritable", err)
				}
			}
		}
	})
}
