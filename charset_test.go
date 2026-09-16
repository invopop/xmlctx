package xmlctx_test

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/invopop/xmlctx"
)

// latin1Reader converts ISO-8859-1 to UTF-8. It stands in for a real charset
// package so that these tests, like the package, need no dependencies.
func latin1Reader(label string, input io.Reader) (io.Reader, error) {
	if !strings.EqualFold(label, "ISO-8859-1") {
		return nil, fmt.Errorf("unsupported charset %q", label)
	}
	b, err := io.ReadAll(input)
	if err != nil {
		return nil, err
	}
	// The first 256 code points are Latin-1, so each byte is its own rune.
	runes := make([]rune, len(b))
	for i, c := range b {
		runes[i] = rune(c)
	}
	return strings.NewReader(string(runes)), nil
}

type charsetDoc struct {
	XMLName xml.Name `xml:"doc"`
	Name    string   `xml:"name"`
}

// latin1Doc declares ISO-8859-1 and holds 0xE9, which is é in that encoding.
var latin1Doc = []byte("<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?>\n<doc><name>caf\xe9</name></doc>")

func TestWithCharsetReader(t *testing.T) {
	t.Run("decodes a non-UTF-8 document", func(t *testing.T) {
		var doc charsetDoc
		err := xmlctx.Unmarshal(latin1Doc, &doc, xmlctx.WithCharsetReader(latin1Reader))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if doc.Name != "café" {
			t.Errorf("got %q, want %q", doc.Name, "café")
		}
	})

	t.Run("rejects a non-UTF-8 document without one", func(t *testing.T) {
		var doc charsetDoc
		err := xmlctx.Unmarshal(latin1Doc, &doc)
		if err == nil {
			t.Fatal("expected an error, got none")
		}
		if !strings.Contains(err.Error(), "CharsetReader is nil") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("propagates the reader's error", func(t *testing.T) {
		doc := []byte(`<?xml version="1.0" encoding="Shift_JIS"?><doc><name>x</name></doc>`)
		var out charsetDoc
		err := xmlctx.Unmarshal(doc, &out, xmlctx.WithCharsetReader(latin1Reader))
		if err == nil {
			t.Fatal("expected an error, got none")
		}
		if !strings.Contains(err.Error(), "unsupported charset") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("leaves UTF-8 documents alone", func(t *testing.T) {
		var doc charsetDoc
		in := []byte(`<?xml version="1.0" encoding="UTF-8"?><doc><name>café</name></doc>`)
		if err := xmlctx.Unmarshal(in, &doc, xmlctx.WithCharsetReader(latin1Reader)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if doc.Name != "café" {
			t.Errorf("got %q, want %q", doc.Name, "café")
		}
	})
}
