package xmlctx_test

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/invopop/xmlctx"
)

type doc struct {
	XMLName struct{} `xml:"note"`
	Text    string   `xml:"text"`
}

const sample = `<?xml version="1.0" encoding="%s"?><note><text>Offre n° 42</text></note>`

// latin1 encodes to ISO-8859-1, where every byte is its own code point.
func latin1(s string) []byte {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		out = append(out, byte(r))
	}
	return out
}

func utf16le(s string) []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xFE})
	for _, u := range utf16.Encode([]rune(s)) {
		buf.WriteByte(byte(u))
		buf.WriteByte(byte(u >> 8))
	}
	return buf.Bytes()
}

func utf16be(s string) []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0xFE, 0xFF})
	for _, u := range utf16.Encode([]rune(s)) {
		buf.WriteByte(byte(u >> 8))
		buf.WriteByte(byte(u))
	}
	return buf.Bytes()
}

// Peppol asks only that envelope and payload share an encoding, and gives its
// own examples in ISO-8859-1. encoding/xml alone refuses every one of these.
func TestCharsetDetection(t *testing.T) {
	const want = "Offre n° 42"

	t.Run("plain UTF-8", func(t *testing.T) {
		var d doc
		mustDecode(t, xmlctx.Unmarshal([]byte(strings.ReplaceAll(sample, "%s", "UTF-8")), &d))
		wantText(t, d.Text, want)
	})

	t.Run("UTF-8 with a byte order mark", func(t *testing.T) {
		raw := append([]byte{0xEF, 0xBB, 0xBF}, strings.ReplaceAll(sample, "%s", "UTF-8")...)
		var d doc
		mustDecode(t, xmlctx.Unmarshal(raw, &d))
		wantText(t, d.Text, want)
	})

	t.Run("ISO-8859-1", func(t *testing.T) {
		var d doc
		mustDecode(t, xmlctx.Unmarshal(latin1(strings.ReplaceAll(sample, "%s", "ISO-8859-1")), &d))
		wantText(t, d.Text, want)
	})

	t.Run("windows-1252, where it departs from latin-1", func(t *testing.T) {
		// 0x80 is the euro sign here, and undefined in ISO-8859-1.
		src := strings.ReplaceAll(sample, "%s", "windows-1252")
		src = strings.ReplaceAll(src, "Offre n° 42", "Prix € 42")
		raw := make([]byte, 0, len(src))
		for _, r := range src {
			if r == '€' {
				raw = append(raw, 0x80)
				continue
			}
			raw = append(raw, byte(r))
		}
		var d doc
		mustDecode(t, xmlctx.Unmarshal(raw, &d))
		wantText(t, d.Text, "Prix € 42")
	})

	t.Run("UTF-16 little endian", func(t *testing.T) {
		var d doc
		mustDecode(t, xmlctx.Unmarshal(utf16le(strings.ReplaceAll(sample, "%s", "UTF-16")), &d))
		wantText(t, d.Text, want)
	})

	t.Run("UTF-16 big endian", func(t *testing.T) {
		var d doc
		mustDecode(t, xmlctx.Unmarshal(utf16be(strings.ReplaceAll(sample, "%s", "UTF-16")), &d))
		wantText(t, d.Text, want)
	})

	t.Run("an encoding we do not know is named in the error", func(t *testing.T) {
		err := xmlctx.Unmarshal([]byte(strings.ReplaceAll(sample, "%s", "EUC-JP")), &doc{})
		if err == nil {
			t.Fatal("expected an error for an unknown charset")
		}
		for _, want := range []string{"EUC-JP", "WithCharsetReader"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error should name %q, got: %v", want, err)
			}
		}
	})

	t.Run("a caller's reader takes precedence", func(t *testing.T) {
		called := false
		var d doc
		err := xmlctx.Unmarshal(latin1(strings.ReplaceAll(sample, "%s", "ISO-8859-1")), &d,
			xmlctx.WithCharsetReader(func(_ string, input io.Reader) (io.Reader, error) {
				called = true
				return &singleByte{src: input}, nil
			}))
		mustDecode(t, err)
		if !called {
			t.Error("the supplied reader should be used, not the built-in one")
		}
		wantText(t, d.Text, want)
	})
}

// singleByte is a caller's own ten-line ISO-8859-1 reader.
type singleByte struct {
	src io.Reader
	out *bytes.Reader
}

func (s *singleByte) Read(p []byte) (int, error) {
	if s.out == nil {
		raw, err := io.ReadAll(s.src)
		if err != nil {
			return 0, err
		}
		var buf bytes.Buffer
		for _, b := range raw {
			buf.WriteRune(rune(b))
		}
		s.out = bytes.NewReader(buf.Bytes())
	}
	return s.out.Read(p)
}

func mustDecode(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func wantText(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A payload lifted out of an envelope may declare nothing while its wrapper
// names the encoding, as a Peppol envelope or an HTTP charset parameter does.
func TestStatedEncoding(t *testing.T) {
	const want = "Offre n° 42"
	const undeclared = `<note><text>Offre n° 42</text></note>`

	t.Run("a document that declares nothing", func(t *testing.T) {
		var d doc
		mustDecode(t, xmlctx.Unmarshal(latin1(undeclared), &d, xmlctx.WithEncoding("ISO-8859-1")))
		wantText(t, d.Text, want)
	})

	t.Run("without being told, the same bytes are not valid UTF-8", func(t *testing.T) {
		err := xmlctx.Unmarshal(latin1(undeclared), &doc{})
		if err == nil {
			t.Fatal("expected latin-1 bytes to be rejected as UTF-8")
		}
	})

	t.Run("a declaration describes the bytes as they arrived", func(t *testing.T) {
		// Decoded before the XML decoder sees it, so the declaration no longer
		// describes what is being read.
		var d doc
		mustDecode(t, xmlctx.Unmarshal(
			latin1(strings.ReplaceAll(sample, "%s", "ISO-8859-1")), &d,
			xmlctx.WithEncoding("ISO-8859-1")))
		wantText(t, d.Text, want)
	})

	t.Run("a byte order mark still wins, being evidence", func(t *testing.T) {
		// Told latin-1, but the document opens with a UTF-16 mark.
		var d doc
		mustDecode(t, xmlctx.Unmarshal(
			utf16le(strings.ReplaceAll(sample, "%s", "UTF-16")), &d,
			xmlctx.WithEncoding("ISO-8859-1")))
		wantText(t, d.Text, want)
	})

	t.Run("UTF-8 stated, which changes nothing", func(t *testing.T) {
		var d doc
		mustDecode(t, xmlctx.Unmarshal([]byte(undeclared), &d, xmlctx.WithEncoding("UTF-8")))
		wantText(t, d.Text, want)
	})
}
