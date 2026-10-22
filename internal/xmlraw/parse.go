package xmlraw

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
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
	p.dec = xml.NewDecoder(bytes.NewReader(lfLineEnds(src)))
	p.dec.Strict = true
	p.dec.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }
	if err := p.parse(); err != nil {
		return nil, err
	}
	return p.doc, nil
}

type parser struct {
	opts ParseOptions
	dec  *xml.Decoder
	doc  *Node
	cur  *Node
	root *Node
}

// errStop ends a recover parse early, at a point where libxml stops reading.
var errStop = errors.New("stop")

func (p *parser) parse() error {
	p.cur = p.doc
	for {
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
	line, _ := p.dec.InputPos()
	return &SyntaxError{Line: line, Msg: msg}
}

func (p *parser) syntaxError(err error) error {
	var serr *xml.SyntaxError
	if errors.As(err, &serr) {
		return &SyntaxError{Line: serr.Line, Msg: serr.Msg}
	}
	return p.fail(strings.TrimPrefix(err.Error(), "xml: "))
}

// normalizeAttrs turns TAB and LF in attribute values into spaces, as XML 1.0 section 3.3.3 says.
// encoding/xml decodes references first, so a referenced LF or TAB turns into a space too.
func normalizeAttrs(attrs []xml.Attr) []xml.Attr {
	out := make([]xml.Attr, len(attrs))
	for i, a := range attrs {
		a.Value = strings.Map(func(r rune) rune {
			if r == '\t' || r == '\n' {
				return ' '
			}
			return r
		}, a.Value)
		out[i] = a
	}
	return out
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
	line, _ := p.dec.InputPos()
	n := &Node{Kind: ElementNode, Name: t.Name, Attrs: normalizeAttrs(t.Attr), Line: line}
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

func (p *parser) text(t xml.CharData) error {
	if p.cur != p.doc {
		p.append(&Node{Kind: TextNode, Data: string(t)})
		return nil
	}
	if len(bytes.Trim(t, " \t\n\r")) > 0 {
		return p.outside("text outside the root element")
	}
	return nil
}
