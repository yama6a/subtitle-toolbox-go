package stbox_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	stbox "github.com/yama6a/subtitle-toolbox-go"
)

func ExampleParse_shiftToWebVTT() {
	german := "1\n00:00:03,500 --> 00:00:06,000\nGuten Morgen.\n\n2\n00:00:07,000 --> 00:00:09,000\nWie geht es Ihnen?\n"

	s, info, err := stbox.Parse(strings.NewReader(german), stbox.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(info.Format, s.Len()) // srt 2

	if err := s.Shift(-2500 * time.Millisecond); err != nil {
		fmt.Println(err)
		return
	}
	if err := stbox.Write(os.Stdout, s, stbox.WebVTT, stbox.WriteOptions{BOM: stbox.BOMRemove}); err != nil {
		fmt.Println(err)
	}
	// Output:
	// srt 2
	// WEBVTT
	//
	// 1
	// 00:00:01.000 --> 00:00:03.500
	// Guten Morgen.
	//
	// 2
	// 00:00:04.500 --> 00:00:06.500
	// Wie geht es Ihnen?
}

func ExampleParse_encoding() {
	latin1 := []byte("1\n00:00:01,000 --> 00:00:02,000\nCaf\xe9 au lait\n")

	s, info, err := stbox.Parse(bytes.NewReader(latin1), stbox.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(info.SourceEncoding, s.Len()) // Windows-1252 1

	_, info, err = stbox.Parse(bytes.NewReader(latin1), stbox.ReadOptions{Encoding: "cp1250"})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(info.SourceEncoding) // cp1250, as the caller wrote it
	// Output:
	// Windows-1252 1
	// cp1250
}

func ExampleParse_error() {
	content := "1\n00:00:01,000 --> 00:00:04,000\nHello\n\n2\n00:00:05,000 => 00:00:07,000\nWorld\n"

	_, _, err := stbox.Parse(strings.NewReader(content), stbox.ReadOptions{Format: stbox.SubRip})
	var perr *stbox.ParseError
	if errors.As(err, &perr) {
		fmt.Println(perr.Format, perr.Line)
	}
	fmt.Println(errors.Is(err, stbox.ErrParse))
	fmt.Println(err)
	// Output:
	// srt 6
	// true
	// srt: line 6: Block #1 has no timing line on its second line. The line is "00:00:05,000 => 00:00:07,000".
}

func ExampleWrite_lineEnding() {
	srt := "1\n00:00:01,000 --> 00:00:02,000\nHello\n"

	s, _, err := stbox.Parse(strings.NewReader(srt), stbox.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	var out bytes.Buffer
	err = stbox.Write(&out, s, stbox.WebVTT, stbox.WriteOptions{LineEnding: stbox.CRLF})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%q\n", out.String())
	// Output:
	// "\ufeffWEBVTT\r\n\r\n1\r\n00:00:01.000 --> 00:00:02.000\r\nHello\r\n"
}
