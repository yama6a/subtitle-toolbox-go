package dcp_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v2/format/dcp"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

func ExampleRead() {
	content := `<DCSubtitle Version="1.0"><Subtitle TimeIn="00:01:30:020" TimeOut="00:01:33:031">` +
		`<Text VAlign="bottom" VPosition="8">from the north pier.</Text>` +
		`<Text VAlign="bottom" VPosition="15">The ferry leaves at nine,</Text></Subtitle></DCSubtitle>`
	s, _, err := dcp.Read(strings.NewReader(content), model.CommonReadOptions{}, dcp.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Lines, c.Position.Alignment)
	}
	// Output:
	// 1m30.08s 1m33.124s [The ferry leaves at nine, from the north pier.] BottomCenter
}

func ExampleWrite() {
	s := &model.Subtitle{}
	s.SetMetadata(model.MetadataTitle, "Harbour")
	data := model.NewData(model.Field{Key: "header", Value: model.NewData(model.Field{Key: "IssueDate", Value: "2026-10-12T00:00:00"})})
	if err := s.SetFormatData("dcp", data); err != nil {
		fmt.Println(err)
		return
	}
	s.AddCue(model.Cue{Start: 5 * time.Second, End: 7125 * time.Millisecond, Lines: []string{"The ferry leaves at nine,", "from the <i>north</i> pier."}})
	if _, err := dcp.Write(os.Stdout, s, model.CommonWriteOptions{}, dcp.WriteOptions{}); err != nil {
		fmt.Println(err)
	}
	// Output:
	// <?xml version="1.0" encoding="UTF-8"?>
	// <SubtitleReel xmlns="http://www.smpte-ra.org/schemas/428-7/2014/DCST" xmlns:xs="http://www.w3.org/2001/XMLSchema">
	//   <Id>urn:uuid:7f9b6950-3c59-3ac4-899d-fe2670a92d7e</Id>
	//   <ContentTitleText>Harbour</ContentTitleText>
	//   <IssueDate>2026-10-12T00:00:00</IssueDate>
	//   <ReelNumber>1</ReelNumber>
	//   <Language>en</Language>
	//   <EditRate>24 1</EditRate>
	//   <TimeCodeRate>24</TimeCodeRate>
	//   <StartTime>00:00:00:00</StartTime>
	//   <LoadFont ID="font">urn:uuid:089a80de-bed7-3cfc-8bde-d41ea8e3cec0</LoadFont>
	//   <SubtitleList>
	//     <Font AspectAdjust="1.0" Color="FFFFFFFF" Effect="none" EffectColor="FF000000" ID="font" Italic="no" Script="normal" Size="42" Underline="no" Weight="normal">
	//       <Subtitle SpotNumber="1" TimeIn="00:00:05:00" TimeOut="00:00:07:03" FadeUpTime="00:00:00:00" FadeDownTime="00:00:00:00">
	//         <Text Valign="bottom" Vposition="15">The ferry leaves at nine,</Text>
	//         <Text Valign="bottom" Vposition="8">from the <Font Italic="yes">north</Font> pier.</Text>
	//       </Subtitle>
	//     </Font>
	//   </SubtitleList>
	// </SubtitleReel>
}
