package xmlctx_test

import (
	stdxml "encoding/xml"
	"reflect"
	"testing"

	"github.com/invopop/xmlctx"
)

// These tests decode the same no-namespace XML twice — once with the standard
// library and once with xmlctx — and assert the results are identical. Using
// plain (unprefixed) tags with XML in no namespace makes encoding/xml and
// xmlctx directly comparable.

// --- (1) single embed ---
type diffSingleHeader struct {
	ID   string `xml:"id"`
	Date string `xml:"date"`
}
type diffSingle struct {
	XMLName stdxml.Name `xml:"doc"`
	diffSingleHeader
	Note string `xml:"note"`
}

// --- (2) nested embed (2 levels) ---
type diffNestA struct {
	A string `xml:"a"`
}
type diffNestB struct {
	diffNestA
	B string `xml:"b"`
}
type diffNested struct {
	XMLName stdxml.Name `xml:"doc"`
	diffNestB
	C string `xml:"c"`
}

// --- (3) embedded pointer ---
type DiffPtrExtra struct {
	Extra string `xml:"extra"`
}
type diffPtr struct {
	XMLName stdxml.Name `xml:"doc"`
	*DiffPtrExtra
	Name string `xml:"name"`
}

// (3b) embedded pointer whose field is named "comment" — the previously
// diverging case: the finder must not mistake it for a ,comment sink and
// must leave the pointer nil when the element is absent.
type DiffPtrComment struct {
	Comment string `xml:"comment"`
}
type DiffPtrCommentDoc struct {
	XMLName stdxml.Name `xml:"doc"`
	*DiffPtrComment
	Name string `xml:"name"`
}

// --- (4) multiple embeds ---
type diffMultiA struct {
	A string `xml:"a"`
}
type diffMultiB struct {
	B string `xml:"b"`
}
type diffMulti struct {
	XMLName stdxml.Name `xml:"doc"`
	diffMultiA
	diffMultiB
	D string `xml:"d"`
}

// --- (5) diamond ambiguity ---
type diffDiamondA struct {
	Val string `xml:"val"`
}
type diffDiamondB struct{ diffDiamondA }
type diffDiamondC struct{ diffDiamondA }
type diffDiamond struct {
	XMLName stdxml.Name `xml:"doc"`
	diffDiamondB
	diffDiamondC
}

// --- (6) shadow ---
type diffShadowBase struct {
	Name  string `xml:"name"`
	Other string `xml:"other"`
}
type diffShadow struct {
	XMLName stdxml.Name `xml:"doc"`
	diffShadowBase
	Name string `xml:"name"`
}

// --- (7) embed contributing an attribute ---
type diffAttrMeta struct {
	Version string `xml:"version,attr"`
}
type diffAttr struct {
	XMLName stdxml.Name `xml:"doc"`
	diffAttrMeta
	Name string `xml:"name"`
}

// --- (8) mix of embedded + direct element fields ---
type diffMixEmbed struct {
	E1 string `xml:"e1"`
	E2 string `xml:"e2"`
}
type diffMix struct {
	XMLName stdxml.Name `xml:"doc"`
	diffMixEmbed
	D1 string `xml:"d1"`
	D2 string `xml:"d2"`
}

// --- (9) non-embedded control ---
type diffControl struct {
	XMLName stdxml.Name `xml:"doc"`
	ID      string      `xml:"id,attr"`
	Name    string      `xml:"name"`
	Items   []string    `xml:"item"`
}

// assertSameAsStdlib decodes data into a fresh value from newTarget with both
// encoding/xml and xmlctx and asserts the results are reflect.DeepEqual.
func assertSameAsStdlib(t *testing.T, data string, newTarget func() any) {
	t.Helper()

	std := newTarget()
	if err := stdxml.Unmarshal([]byte(data), std); err != nil {
		t.Fatalf("encoding/xml failed: %v", err)
	}

	ctx := newTarget()
	if err := xmlctx.Unmarshal([]byte(data), ctx, xmlctx.WithNamespaces(map[string]string{})); err != nil {
		t.Fatalf("xmlctx failed: %v", err)
	}

	if !reflect.DeepEqual(std, ctx) {
		t.Errorf("results differ:\n  encoding/xml: %+v\n  xmlctx:       %+v", std, ctx)
	}
}

func TestDiffEmbeddedShapes(t *testing.T) {
	cases := []struct {
		name string
		xml  string
		mk   func() any
	}{
		{
			name: "single embed",
			xml:  `<doc><id>1</id><date>2024-01-01</date><note>n</note></doc>`,
			mk:   func() any { return &diffSingle{} },
		},
		{
			name: "nested embed two levels",
			xml:  `<doc><a>1</a><b>2</b><c>3</c></doc>`,
			mk:   func() any { return &diffNested{} },
		},
		{
			name: "embedded pointer populated",
			xml:  `<doc><name>x</name><extra>hi</extra></doc>`,
			mk:   func() any { return &diffPtr{} },
		},
		{
			name: "embedded pointer absent stays nil",
			xml:  `<doc><name>x</name></doc>`,
			mk:   func() any { return &diffPtr{} },
		},
		{
			// Previously diverged: xmlctx spuriously allocated the pointer
			// because the ,comment finder matched a field named "comment".
			name: "embedded pointer with comment field, absent, stays nil",
			xml:  `<doc><name>x</name></doc>`,
			mk:   func() any { return &DiffPtrCommentDoc{} },
		},
		{
			name: "embedded pointer with comment field, present",
			xml:  `<doc><name>x</name><comment>hi</comment></doc>`,
			mk:   func() any { return &DiffPtrCommentDoc{} },
		},
		{
			name: "multiple embeds",
			xml:  `<doc><a>1</a><b>2</b><d>4</d></doc>`,
			mk:   func() any { return &diffMulti{} },
		},
		{
			name: "shadow: outer field wins",
			xml:  `<doc><name>outer</name><other>o</other></doc>`,
			mk:   func() any { return &diffShadow{} },
		},
		{
			name: "embed contributes attribute",
			xml:  `<doc version="1.2"><name>x</name></doc>`,
			mk:   func() any { return &diffAttr{} },
		},
		{
			name: "mix embedded and direct fields",
			xml:  `<doc><e1>a</e1><e2>b</e2><d1>c</d1><d2>e</d2></doc>`,
			mk:   func() any { return &diffMix{} },
		},
		{
			name: "non-embedded control",
			xml:  `<doc id="i"><name>x</name><item>1</item><item>2</item></doc>`,
			mk:   func() any { return &diffControl{} },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertSameAsStdlib(t, tc.xml, tc.mk)
		})
	}
}

// TestDiffDiamondAmbiguity covers diamond ambiguity separately because the two
// decoders legitimately differ on error handling: encoding/xml returns an
// error and leaves the field zero, while xmlctx silently drops the ambiguous
// field (a deliberate leniency difference, not a bug). We therefore do not
// require error parity or DeepEqual — we assert the field is empty in BOTH.
func TestDiffDiamondAmbiguity(t *testing.T) {
	xmlData := []byte(`<doc><val>x</val></doc>`)

	// encoding/xml returns an error for the conflicting promoted field.
	var std diffDiamond
	if err := stdxml.Unmarshal(xmlData, &std); err == nil {
		t.Error("expected encoding/xml to report a conflict error, got nil")
	}
	if std.diffDiamondB.Val != "" || std.diffDiamondC.Val != "" {
		t.Errorf("encoding/xml: ambiguous field should be zero, got B=%q C=%q",
			std.diffDiamondB.Val, std.diffDiamondC.Val)
	}

	// xmlctx drops the ambiguous field without erroring.
	var ctx diffDiamond
	if err := xmlctx.Unmarshal(xmlData, &ctx, xmlctx.WithNamespaces(map[string]string{})); err != nil {
		t.Fatalf("xmlctx failed: %v", err)
	}
	if ctx.diffDiamondB.Val != "" || ctx.diffDiamondC.Val != "" {
		t.Errorf("xmlctx: ambiguous field should be dropped (empty), got B=%q C=%q",
			ctx.diffDiamondB.Val, ctx.diffDiamondC.Val)
	}
}

// TestDiffExactValues pins specific decoded values (table-driven) to lock in
// behavior beyond the stdlib comparison.
func TestDiffExactValues(t *testing.T) {
	t.Run("single embed promotes fields", func(t *testing.T) {
		var d diffSingle
		if err := xmlctx.Unmarshal(
			[]byte(`<doc><id>42</id><date>2024-01-01</date><note>n</note></doc>`),
			&d, xmlctx.WithNamespaces(map[string]string{}),
		); err != nil {
			t.Fatal(err)
		}
		if d.ID != "42" || d.Date != "2024-01-01" || d.Note != "n" {
			t.Errorf("got %+v", d)
		}
	})

	t.Run("nested embed promotes across two levels", func(t *testing.T) {
		var d diffNested
		if err := xmlctx.Unmarshal(
			[]byte(`<doc><a>1</a><b>2</b><c>3</c></doc>`),
			&d, xmlctx.WithNamespaces(map[string]string{}),
		); err != nil {
			t.Fatal(err)
		}
		if d.A != "1" || d.B != "2" || d.C != "3" {
			t.Errorf("got %+v", d)
		}
	})

	t.Run("comment-named field in embedded pointer stays nil when absent", func(t *testing.T) {
		var d DiffPtrCommentDoc
		if err := xmlctx.Unmarshal(
			[]byte(`<doc><name>x</name></doc>`),
			&d, xmlctx.WithNamespaces(map[string]string{}),
		); err != nil {
			t.Fatal(err)
		}
		if d.DiffPtrComment != nil {
			t.Errorf("embedded pointer should be nil, got %+v", d.DiffPtrComment)
		}
	})

	t.Run("embedded attribute promoted", func(t *testing.T) {
		var d diffAttr
		if err := xmlctx.Unmarshal(
			[]byte(`<doc version="1.2"><name>x</name></doc>`),
			&d, xmlctx.WithNamespaces(map[string]string{}),
		); err != nil {
			t.Fatal(err)
		}
		if d.Version != "1.2" || d.Name != "x" {
			t.Errorf("got %+v", d)
		}
	})

	t.Run("shadow: outer field wins, promoted stays empty", func(t *testing.T) {
		var d diffShadow
		if err := xmlctx.Unmarshal(
			[]byte(`<doc><name>outer</name><other>o</other></doc>`),
			&d, xmlctx.WithNamespaces(map[string]string{}),
		); err != nil {
			t.Fatal(err)
		}
		if d.Name != "outer" || d.diffShadowBase.Name != "" || d.Other != "o" {
			t.Errorf("got %+v", d)
		}
	})
}
