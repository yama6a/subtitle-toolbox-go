package xmlraw

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// ParseOptions holds the settings of Parse.
type ParseOptions struct {
	// Recover repairs unclosed and mismatched elements as libxml recover mode does.
	Recover bool
}

// Parse reads a document. It loads no DTD and no external entity, and it reads the content as UTF-8 whatever the
// XML declaration says. CR LF and a lone CR become LF first, as libxml reads them. A failed parse returns a
// *SyntaxError. Recover mode fails only when no root element is left.
//
// Recover mode closes the current element at a mismatched end tag, whatever its name, and stops at the first
// other error or at content after the root element. Then every open element closes.
func Parse(r io.Reader, opts ParseOptions) (*Node, error) {
	src, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("xmlraw: %w", err)
	}
	p := &parser{opts: opts, doc: &Node{Kind: DocumentNode}}
	src = lfLineEnds(src)
	p.breaks = spaceAttributeValues(src)
	p.src = src
	p.dec = xml.NewDecoder(bytes.NewReader(src))
	p.dec.Strict = true
	p.dec.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }
	if err := p.parse(); err != nil {
		return nil, err
	}
	return p.doc, nil
}

type parser struct {
	opts ParseOptions
	// src is the input that the decoder reads, so the parser can tell a CDATA section from text.
	src []byte
	// tokenStart is the offset of the token that the decoder read last.
	tokenStart int64
	dec        *xml.Decoder
	// breaks holds the offsets of the LFs in attribute values that spaceAttributeValues turned into spaces.
	breaks []int
	doc    *Node
	cur    *Node
	root   *Node
}

// errStop ends a recover parse early, at a point where libxml stops reading.
var errStop = errors.New("stop")

func (p *parser) parse() error {
	p.cur = p.doc
	for {
		p.tokenStart = p.dec.InputOffset()
		tok, err := p.dec.RawToken()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if p.opts.Recover {
				break
			}
			return p.syntaxError(err)
		}
		if err := p.add(tok); err != nil {
			if errors.Is(err, errStop) {
				break
			}
			return err
		}
	}
	if p.cur != p.doc && !p.opts.Recover {
		return p.fail("unexpected EOF: element <" + QName(p.cur.Name) + "> is not closed")
	}
	if p.root == nil {
		return p.fail("the document has no root element")
	}
	return nil
}

func (p *parser) add(tok xml.Token) error {
	switch t := tok.(type) {
	case xml.StartElement:
		return p.start(t)
	case xml.EndElement:
		return p.end(t)
	case xml.CharData:
		return p.text(t)
	case xml.Comment:
		p.append(&Node{Kind: CommentNode, Data: string(t)})
	case xml.ProcInst:
		if t.Target != "xml" || p.cur != p.doc {
			p.append(&Node{Kind: ProcInstNode, Name: xml.Name{Local: t.Target}, Data: string(t.Inst)})
		}
	case xml.Directive:
		p.append(&Node{Kind: DirectiveNode, Data: string(t)})
	}
	return nil
}

func (p *parser) append(n *Node) {
	n.Parent = p.cur
	p.cur.Children = append(p.cur.Children, n)
}

// outside fails on content outside the root element, where libxml recover mode stops reading.
func (p *parser) outside(what string) error {
	if p.opts.Recover {
		return errStop
	}
	return p.fail("the document has " + what)
}

func (p *parser) fail(msg string) error {
	return &SyntaxError{Line: p.line(), Msg: msg}
}

func (p *parser) syntaxError(err error) error {
	var serr *xml.SyntaxError
	if errors.As(err, &serr) {
		return &SyntaxError{Line: p.line(), Msg: serr.Msg}
	}
	return p.fail(strings.TrimPrefix(err.Error(), "xml: "))
}

// line returns the line of the decoder in the input, with the LFs that spaceAttributeValues replaced.
func (p *parser) line() int {
	line, _ := p.dec.InputPos()
	return line + sort.SearchInts(p.breaks, int(p.dec.InputOffset()))
}

// spaceAttributeValues turns each literal TAB and LF in the attribute values of the tags into a space, as XML 1.0
// section 3.3.3 says, and returns the offsets of the LFs. It runs before encoding/xml decodes the references, so a
// TAB or LF of a reference such as &#10; stays, as in libxml2. Comments, CDATA sections, processing instructions
// and the DOCTYPE keep their content. It changes src in place.
func spaceAttributeValues(src []byte) []int {
	var breaks []int
	for i := 0; i < len(src); i++ {
		if src[i] != '<' {
			continue
		}
		rest := src[i:]
		switch {
		case bytes.HasPrefix(rest, []byte("<!--")):
			i += skipPast(rest, "<!--", "-->")
		case bytes.HasPrefix(rest, []byte("<![CDATA[")):
			i += skipPast(rest, "<![CDATA[", "]]>")
		case bytes.HasPrefix(rest, []byte("<?")):
			i += skipPast(rest, "<?", "?>")
		case bytes.HasPrefix(rest, []byte("<!")):
			i += skipDeclaration(rest)
		default:
			i += spaceTag(rest, i, &breaks)
		}
	}
	return breaks
}

func skipPast(s []byte, start, end string) int {
	if j := bytes.Index(s[len(start):], []byte(end)); j >= 0 {
		return len(start) + j + len(end) - 1
	}
	return len(s)
}

// skipDeclaration returns the length of a declaration such as <!DOCTYPE ...>, whose internal subset in [ and ] and
// whose quoted strings can hold >. A comment of the subset can hold quotes and brackets, as encoding/xml reads it.
func skipDeclaration(s []byte) int {
	depth := 0
	var quote byte
	for j := 0; j < len(s); j++ {
		c := s[j]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case bytes.HasPrefix(s[j:], []byte("<!--")):
			j += skipPast(s[j:], "<!--", "-->")
		case c == '"', c == '\'':
			quote = c
		case c == '[':
			depth++
		case c == ']':
			depth--
		case c == '>' && depth <= 0:
			return j
		}
	}
	return len(s)
}

func spaceTag(s []byte, at int, breaks *[]int) int {
	var quote byte
	for j, c := range s {
		if quote == 0 {
			switch c {
			case '"', '\'':
				quote = c
			case '>':
				return j
			}
			continue
		}
		switch c {
		case quote:
			quote = 0
		case '\n':
			*breaks = append(*breaks, at+j)
			s[j] = ' '
		case '\t':
			s[j] = ' '
		}
	}
	return len(s)
}

func lfLineEnds(src []byte) []byte {
	if bytes.IndexByte(src, '\r') < 0 {
		return src
	}
	src = bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))
	return bytes.ReplaceAll(src, []byte("\r"), []byte("\n"))
}

func (p *parser) start(t xml.StartElement) error {
	if p.cur == p.doc && p.root != nil {
		return p.outside("a second root element <" + QName(t.Name) + ">")
	}
	n := &Node{Kind: ElementNode, Name: t.Name, Attrs: t.Attr, Line: p.line()}
	p.append(n)
	if p.cur == p.doc {
		p.root = n
	}
	p.cur = n
	return nil
}

func (p *parser) end(t xml.EndElement) error {
	if p.cur == p.doc {
		return p.outside("the end tag </" + QName(t.Name) + "> outside the root element")
	}
	if t.Name != p.cur.Name && !p.opts.Recover {
		return p.fail("element <" + QName(p.cur.Name) + "> closed by </" + QName(t.Name) + ">")
	}
	p.cur = p.cur.Parent
	return nil
}

// text adds text or a CDATA section, which encoding/xml both returns as xml.CharData.
func (p *parser) text(t xml.CharData) error {
	if p.cur != p.doc {
		kind := TextNode
		if bytes.HasPrefix(p.src[p.tokenStart:], []byte("<![CDATA[")) {
			kind = CDATANode
		}
		p.append(&Node{Kind: kind, Data: string(t)})
		return nil
	}
	if len(bytes.Trim(t, " \t\n\r")) > 0 {
		return p.outside("text outside the root element")
	}
	return nil
}
