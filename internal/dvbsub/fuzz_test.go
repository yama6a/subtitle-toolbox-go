package dvbsub_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/dvbsub"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

func FuzzDecoder(f *testing.F) {
	f.Add([]byte{0x20, 0x00, 0x0F, 0x10, 0, 1, 0, 2, 5, 0x08, 0x0F, 0x80, 0, 1, 0, 0}, true)
	f.Add([]byte{0x0F, 0x11, 0, 1, 0, 10, 1, 0, 0, 2, 0, 2, 0x08, 1, 0, 0, 0x0F, 0x13, 0, 1, 0, 9, 0, 1, 0, 0, 3, 0, 0, 0x11, 0x12, 0xF0}, false)

	f.Fuzz(func(t *testing.T, data []byte, all bool) {
		d := dvbsub.NewDecoder(1, 1, all, model.CommonReadOptions{Lenient: all})
		if err := d.Add(data, time.Second, 2*time.Second, true); err != nil {
			var perr *model.ParseError
			require.ErrorAs(t, err, &perr)
			return
		}
		for _, c := range d.Finish(time.Second) {
			require.NoError(t, c.Image.Validate())
		}
	})
}
