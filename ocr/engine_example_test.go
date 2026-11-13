package ocr_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
	"github.com/yama6a/subtitle-toolbox-go/v4/ocr"
)

// webEngine sends the PNG of a cue to an OCR web service and reads {"lines": [...]} from the answer.
type webEngine struct {
	url string
}

func (e webEngine) Recognize(ctx context.Context, im model.Image, language ocr.Language) (ocr.Text, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.url+"?lang="+url.QueryEscape(string(language)), bytes.NewReader(im.PNG))
	if err != nil {
		return ocr.Text{}, err
	}
	req.Header.Set("Content-Type", "image/png")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ocr.Text{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ocr.Text{}, fmt.Errorf("the service answered %s", resp.Status)
	}
	var answer struct {
		Lines []string `json:"lines"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&answer); err != nil {
		return ocr.Text{}, err
	}
	return ocr.Text{Lines: answer.Lines}, nil
}

func ExampleEngine() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string][]string{"lines": {"language " + r.URL.Query().Get("lang")}})
	}))
	defer server.Close()

	text, err := webEngine{url: server.URL}.Recognize(context.Background(), model.Image{PNG: []byte("PNG")}, ocr.German)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(text.Lines)
	// Output:
	// [language deu]
}
