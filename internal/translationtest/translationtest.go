// Package translationtest plays DeepL, Google Cloud Translation and an OpenAI-compatible service in tests.
package translationtest

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
)

// Request is one request that a Server or a Client got.
type Request struct {
	URL    string // the full URL
	Path   string // for example "/v2/translate"
	Key    string // the key of the service header, "" without one
	Header http.Header
	Body   []byte
}

type route struct {
	header, prefix  string
	okFile, errFile string
}

//nolint:gochecknoglobals // a lookup table
var routes = map[string]route{
	"/v2/translate":          {"Authorization", "DeepL-Auth-Key ", "deepl_de.json", "deepl_error.json"},
	"/language/translate/v2": {"X-Goog-Api-Key", "", "google_fr.json", "google_error.json"},
	"/v1/chat/completions":   {"Authorization", "Bearer ", "openai_de.json", "openai_error.json"},
}

func keyOf(path string, h http.Header) string {
	r, ok := routes[path]
	if !ok {
		return ""
	}
	v := h.Get(r.header)
	if rest, ok := strings.CutPrefix(v, r.prefix); ok {
		return rest
	}
	return v
}

// Server is an httptest.Server with the routes /v2/translate, /language/translate/v2 and /v1/chat/completions.
// It is safe for concurrent use.
type Server struct {
	URL string // for example "http://127.0.0.1:41234". The OpenAI-compatible base URL is URL + "/v1"

	srv      *httptest.Server
	files    map[string][]byte
	mu       sync.Mutex
	requests []Request
}

// NewServer starts a server and closes it in tb.Cleanup.
func NewServer(tb testing.TB) *Server {
	tb.Helper()

	s := &Server{files: map[string][]byte{}}
	for _, r := range routes {
		for _, name := range []string{r.okFile, r.errFile} {
			s.files[name] = testfiles.Read(tb, "translation/"+name)
		}
	}
	s.srv = httptest.NewServer(http.HandlerFunc(s.serve))
	tb.Cleanup(s.srv.Close)
	s.URL = s.srv.URL
	return s
}

func (s *Server) serve(w http.ResponseWriter, req *http.Request) {
	body, _ := io.ReadAll(req.Body)
	key := keyOf(req.URL.Path, req.Header)
	s.mu.Lock()
	s.requests = append(s.requests, Request{
		URL: "http://" + req.Host + req.URL.RequestURI(), Path: req.URL.Path, Key: key, Header: req.Header.Clone(), Body: body,
	})
	s.mu.Unlock()

	var status int
	var answer []byte
	r, ok := routes[req.URL.Path]
	switch {
	case !ok:
		status, answer = http.StatusNotFound, []byte(`{"message": "not found"}`)
	case key == "forbidden":
		status, answer = http.StatusForbidden, []byte(`{"message": "error"}`)
	case key == "busy":
		status, answer = http.StatusTooManyRequests, []byte(`{"message": "error"}`)
	case key == "quota":
		status, answer = 456, []byte(`{"message": "error"}`)
	case key == "bad-request":
		status, answer = http.StatusBadRequest, s.files[r.errFile]
	default:
		status, answer = http.StatusOK, s.files[r.okFile]
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(answer)
}

// Requests returns a copy of the requests in arrival order.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Request(nil), s.requests...)
}

// Answer is one canned answer of a Client.
type Answer struct {
	Status int
	Header http.Header
	Body   []byte
}

// Client is an HTTP client that records each request and plays Answers in order.
// Request k gets Answers[min(k, len(Answers)-1)]. Answers must hold at least one answer. It is safe for concurrent use.
type Client struct {
	Answers []Answer

	mu       sync.Mutex
	requests []Request
}

// Do records req and returns the next answer. It never returns an error.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
		_ = req.Body.Close()
	}
	c.mu.Lock()
	k := len(c.requests)
	c.requests = append(c.requests, Request{
		URL: req.URL.String(), Path: req.URL.Path, Key: keyOf(req.URL.Path, req.Header), Header: req.Header.Clone(), Body: body,
	})
	a := c.Answers[min(k, len(c.Answers)-1)]
	c.mu.Unlock()

	header := a.Header.Clone()
	if header == nil {
		header = http.Header{}
	}
	return &http.Response{
		Status:     strconv.Itoa(a.Status) + " " + http.StatusText(a.Status),
		StatusCode: a.Status,
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader(a.Body)),
		Request:    req,
	}, nil
}

// Requests returns a copy of the requests in arrival order.
func (c *Client) Requests() []Request {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Request(nil), c.requests...)
}

// ClosedURL returns http://127.0.0.1:PORT for a port that no server listens on.
func ClosedURL(tb testing.TB) string {
	tb.Helper()

	l, err := (&net.ListenConfig{}).Listen(tb.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatalf("listen: %v", err)
	}
	addr := l.Addr().String()
	if err := l.Close(); err != nil {
		tb.Fatalf("close the listener: %v", err)
	}
	return "http://" + addr
}
