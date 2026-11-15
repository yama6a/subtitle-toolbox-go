package csv_test

import (
	"bytes"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/csv"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

func FuzzRead(f *testing.F) {
	formattest.AddSeeds(f, "csv/*.csv", "csv/real/*.csv", "csv/real/*.tsv")
	for _, seed := range []string{
		"start,end,text\n1,2,\"a\n",
		"a,1,2\nb,3,4\n",
		"Start Time,End Time,Subtitle\n1,2.5,a\n",
		"start\tend\ttext\n0\t2340\ta\n2340\t5100\tb\n",
		"start,duration,text\n99999:59:59,2,a\n",
		"start,end,text\n99999999999999999999:00:00:00,1,a\n00:00:1,0:5,b\n",
		"\"\"\"x\",1,2\r\r\n;;\t\x00\xff",
	} {
		f.Add([]byte(seed))
	}
	layouts := []csv.ReadOptions{
		{},
		{Columns: csv.Columns{Start: csv.ColumnAt(0), End: csv.ColumnAt(1), Text: csv.ColumnAt(2), NoHeader: true}, FrameRate: model.FrameRate{Num: 25, Den: 1}},
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, opts := range layouts {
			for _, lenient := range []bool{false, true} {
				s, warnings, err := csv.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, opts)
				if !formattest.CheckRead(t, s, warnings, err) {
					continue
				}
				var out bytes.Buffer
				if _, err := csv.Write(&out, s, model.CommonWriteOptions{}, csv.WriteOptions{}); err != nil {
					t.Fatalf("Write returned %v", err)
				}
			}
		}
	})
}
