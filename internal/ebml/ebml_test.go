package ebml_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/ebml"
)

func TestReadVint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		in         string
		keepMarker bool
		value      uint64
		length     int
		ok         bool
	}{
		{"1-byte ID with marker", "\xA3", true, 0xA3, 1, true},
		{"4-byte ID with marker", "\x1A\x45\xDF\xA3", true, 0x1A45DFA3, 4, true},
		{"1-byte size", "\x85", false, 5, 1, true},
		{"2-byte size", "\x40\x7F", false, 127, 2, true},
		{"8-byte size", "\x01\x00\x00\x00\x00\x01\x00\x00", false, 65536, 8, true},
		{"first byte 0", "\x00\x81", false, 0, 0, false},
		{"too few bytes", "\x40", false, 0, 0, false},
		{"empty", "", false, 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			value, length, ok := ebml.ReadVint([]byte(tt.in), tt.keepMarker)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.value, value)
			assert.Equal(t, tt.length, length)
		})
	}
}

func newReader(data string) *ebml.Reader {
	return ebml.NewReader(bytes.NewReader([]byte(data)), int64(len(data)))
}

func TestHeadersAndValues(t *testing.T) {
	t.Parallel()

	data := "\x2A\xD7\xB1\x83\x01\x86\xA0" + // TimestampScale 100000
		"\x42\x82\x86webm\x00\x00" + // DocType with NUL padding
		"\x1F\x43\xB6\x75\x01\xFF\xFF\xFF\xFF\xFF\xFF\xFF" // Cluster of unknown size
	r := newReader(data)

	h, ok, err := r.Header(0)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, ebml.Header{ID: 0x2AD7B1, Size: 3, Offset: 4}, h)
	v, err := r.Unsigned(h)
	require.NoError(t, err)
	assert.Equal(t, uint64(100000), v)

	h, _, err = r.Header(7)
	require.NoError(t, err)
	assert.Equal(t, uint32(0x4282), h.ID)
	s, err := r.String(h)
	require.NoError(t, err)
	assert.Equal(t, "webm", s)

	h, _, err = r.Header(h.Offset + h.Size)
	require.NoError(t, err)
	assert.Equal(t, int64(ebml.UnknownSize), h.Size)

	_, ok, err = r.Header(int64(len(data)))
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestBrokenElements(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data string
		do   func(r *ebml.Reader) error
		want string
	}{
		{
			"ID with first byte 0", "\x00\xA3", func(r *ebml.Reader) error { _, _, err := r.Header(0); return err },
			"The EBML element header at byte 0 is not valid.",
		},
		{
			"size with first byte 0", "\xA3\x00\x81", func(r *ebml.Reader) error { _, _, err := r.Header(0); return err },
			"The EBML element header at byte 0 is not valid.",
		},
		{
			"size cut by the file end", "\xA3\x40", func(r *ebml.Reader) error { _, _, err := r.Header(0); return err },
			"The EBML element header at byte 0 is not valid.",
		},
		{"data cut by the file end", "\x86\x8BS_TEXT", func(r *ebml.Reader) error {
			h, _, err := r.Header(0)
			require.NoError(t, err)
			_, err = r.String(h)
			return err
		}, "The element data at byte 2 needs 11 bytes, but the file ends before."},
		{"data of 2^40 bytes", "\xA3\x01\x00\x01\x00\x00\x00\x00\x00", func(r *ebml.Reader) error {
			h, _, err := r.Header(0)
			require.NoError(t, err)
			_, err = r.Bytes(h)
			return err
		}, "The element data at byte 9 needs 1099511627776 bytes, but the file ends before."},
		{"unsigned of 2^63", "\xE7\x88\x80\x00\x00\x00\x00\x00\x00\x00", func(r *ebml.Reader) error {
			h, _, _ := r.Header(0)
			_, err := r.Unsigned(h)
			return err
		}, "The unsigned integer of the element 0xE7 at byte 2 is longer than 8 bytes or not below 2^63."},
		{"unsigned of 9 bytes", "\xE7\x89\x00\x00\x00\x00\x00\x00\x00\x00\x01", func(r *ebml.Reader) error {
			h, _, _ := r.Header(0)
			_, err := r.Unsigned(h)
			return err
		}, "The unsigned integer of the element 0xE7 at byte 2 is longer than 8 bytes or not below 2^63."},
		{"master element of unknown size", "\x1F\x43\xB6\x75\x01\xFF\xFF\xFF\xFF\xFF\xFF\xFF", func(r *ebml.Reader) error {
			h, _, _ := r.Header(0)
			for _, err := range r.Children(h) {
				return err
			}
			return nil
		}, "The element 0x1F43B675 at byte 12 has an unknown size, which only Segment and Cluster may have."},
		{"child larger than its parent", "\x18\x53\x80\x67\x83\x86\x85ab", func(r *ebml.Reader) error {
			h, _, _ := r.Header(0)
			for _, err := range r.Children(h) {
				return err
			}
			return nil
		}, "The element at byte 5 does not fit into its parent element."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var ee *ebml.Error
			require.ErrorAs(t, tt.do(newReader(tt.data)), &ee)
			assert.Equal(t, tt.want, ee.Msg)
		})
	}
}

func TestLargestUnsigned(t *testing.T) {
	t.Parallel()

	r := newReader("\xE7\x88\x7F\xFF\xFF\xFF\xFF\xFF\xFF\xFF")
	h, _, err := r.Header(0)
	require.NoError(t, err)
	v, err := r.Unsigned(h)
	require.NoError(t, err)
	assert.Equal(t, uint64(1<<63-1), v)
}
