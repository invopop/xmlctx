package xmlctx

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Byte order marks. UTF-32's open with UTF-16's, so the longer is tested first.
var (
	bomUTF8    = []byte{0xEF, 0xBB, 0xBF}
	bomUTF16BE = []byte{0xFE, 0xFF}
	bomUTF16LE = []byte{0xFF, 0xFE}
	bomUTF32LE = []byte{0xFF, 0xFE, 0x00, 0x00}
)

// autoReader returns a reader yielding UTF-8, whatever the document opened
// with, falling back to a stated encoding when no mark settles it.
//
// UTF-16 has to be transcoded here rather than by a CharsetReader: the
// declaration naming it is itself UTF-16, so the decoder cannot read far enough
// to find it.
func autoReader(r io.Reader, stated string) io.Reader {
	br := bufio.NewReader(r)
	prefix, _ := br.Peek(4) // short documents peek short; that is not an error
	switch {
	case bytes.HasPrefix(prefix, bomUTF8):
		_, _ = br.Discard(len(bomUTF8))
		return br
	case bytes.HasPrefix(prefix, bomUTF32LE):
		// Ruled out rather than supported, so it is not read as UTF-16LE.
		return br
	case bytes.HasPrefix(prefix, bomUTF16BE):
		_, _ = br.Discard(len(bomUTF16BE))
		return &utf16Reader{src: br, bigEndian: true}
	case bytes.HasPrefix(prefix, bomUTF16LE):
		_, _ = br.Discard(len(bomUTF16LE))
		return &utf16Reader{src: br}
	}
	if stated != "" {
		// The document may declare nothing, and cannot know better anyway.
		if decoded, err := defaultCharsetReader(stated, br); err == nil {
			return decoded
		}
	}
	return br
}

// passthroughCharset accepts any declaration untouched, for bytes decoded
// before the XML decoder saw them.
func passthroughCharset(_ string, input io.Reader) (io.Reader, error) {
	return input, nil
}

// NewXMLDecoder returns an encoding/xml decoder with this package's charset
// handling, for callers walking tokens themselves. Sniffing a root element with
// a plain xml.NewDecoder refuses every encoding but UTF-8, before the document
// reaches anything that could read it.
func NewXMLDecoder(r io.Reader) *xml.Decoder {
	dec := xml.NewDecoder(autoReader(r, ""))
	dec.CharsetReader = defaultCharsetReader
	return dec
}

// utf16Reader transcodes UTF-16 to UTF-8, reading its source whole so a
// surrogate pair cannot straddle a read boundary.
type utf16Reader struct {
	src       io.Reader
	bigEndian bool
	out       *bytes.Reader
}

func (u *utf16Reader) Read(p []byte) (int, error) {
	if u.out == nil {
		raw, err := io.ReadAll(u.src)
		if err != nil {
			return 0, err
		}
		u.out = bytes.NewReader(utf16ToUTF8(raw, u.bigEndian))
	}
	return u.out.Read(p)
}

func utf16ToUTF8(raw []byte, bigEndian bool) []byte {
	units := make([]uint16, 0, len(raw)/2)
	for i := 0; i+1 < len(raw); i += 2 {
		if bigEndian {
			units = append(units, uint16(raw[i])<<8|uint16(raw[i+1]))
		} else {
			units = append(units, uint16(raw[i+1])<<8|uint16(raw[i]))
		}
	}
	var buf bytes.Buffer
	buf.Grow(len(units))
	for _, r := range utf16.Decode(units) {
		buf.WriteRune(r)
	}
	return buf.Bytes()
}

// windows1252 holds only the bytes where it departs from ISO-8859-1.
var windows1252 = map[byte]rune{
	0x80: '€', 0x82: '‚', 0x83: 'ƒ', 0x84: '„', 0x85: '…', 0x86: '†', 0x87: '‡',
	0x88: 'ˆ', 0x89: '‰', 0x8A: 'Š', 0x8B: '‹', 0x8C: 'Œ', 0x8E: 'Ž', 0x91: '‘',
	0x92: '’', 0x93: '“', 0x94: '”', 0x95: '•', 0x96: '–', 0x97: '—', 0x98: '˜',
	0x99: '™', 0x9A: 'š', 0x9B: '›', 0x9C: 'œ', 0x9E: 'ž', 0x9F: 'Ÿ',
}

// defaultCharsetReader decodes the encodings still met in European
// e-invoicing. Peppol permits them: its envelope specification asks only that
// envelope and payload agree, and its own examples are in ISO-8859-1.
func defaultCharsetReader(label string, input io.Reader) (io.Reader, error) {
	switch normaliseCharset(label) {
	case "utf-8", "us-ascii", "ascii":
		return input, nil
	case "utf-16", "utf-16le", "utf-16be":
		// Already transcoded by autoReader; the declaration describes the
		// bytes as they arrived.
		return input, nil
	case "iso-8859-1", "iso8859-1", "latin1", "iso-latin-1", "iso_8859-1":
		return &singleByteReader{src: input}, nil
	case "windows-1252", "cp1252":
		return &singleByteReader{src: input, table: windows1252}, nil
	}
	return nil, fmt.Errorf("unsupported charset %q: pass WithCharsetReader to decode it", label)
}

func normaliseCharset(label string) string {
	return strings.ToLower(strings.TrimSpace(label))
}

// singleByteReader decodes a single-byte encoding to UTF-8. With no table each
// byte is its own code point, which is ISO-8859-1.
type singleByteReader struct {
	src   io.Reader
	table map[byte]rune
	out   *bytes.Reader
}

func (s *singleByteReader) Read(p []byte) (int, error) {
	if s.out == nil {
		raw, err := io.ReadAll(s.src)
		if err != nil {
			return 0, err
		}
		buf := make([]byte, 0, len(raw))
		for _, b := range raw {
			r := rune(b)
			if s.table != nil {
				if mapped, ok := s.table[b]; ok {
					r = mapped
				}
			}
			buf = utf8.AppendRune(buf, r)
		}
		s.out = bytes.NewReader(buf)
	}
	return s.out.Read(p)
}
