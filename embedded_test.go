package xmlctx_test

import (
	"encoding/xml"
	"testing"

	"github.com/invopop/xmlctx"
)

const cbcNS = "urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2"

// header is an unexported anonymous embed with exported, promotable fields.
type header struct {
	ID        string `xml:"cbc:ID"`
	IssueDate string `xml:"cbc:IssueDate"`
}

// embInvoice embeds header anonymously alongside regular fields.
type embInvoice struct {
	XMLName xml.Name
	header
	Note string `xml:"cbc:Note,omitempty"`
}

// TestEmbeddedStructFields checks element fields promoted from an embed.
func TestEmbeddedStructFields(t *testing.T) {
	xmlData := []byte(`<Invoice xmlns:cbc="` + cbcNS + `">` +
		`<cbc:ID>x</cbc:ID>` +
		`<cbc:IssueDate>2024-01-01</cbc:IssueDate>` +
		`<cbc:Note>n</cbc:Note>` +
		`</Invoice>`)

	var inv embInvoice
	err := xmlctx.Unmarshal(xmlData, &inv,
		xmlctx.WithNamespaces(map[string]string{
			"cbc": cbcNS,
		}),
	)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if inv.XMLName.Local != "Invoice" {
		t.Errorf("XMLName.Local: got %s, want Invoice", inv.XMLName.Local)
	}
	if inv.ID != "x" {
		t.Errorf("ID: got %q, want x", inv.ID)
	}
	if inv.IssueDate != "2024-01-01" {
		t.Errorf("IssueDate: got %q, want 2024-01-01", inv.IssueDate)
	}
	if inv.Note != "n" {
		t.Errorf("Note: got %q, want n", inv.Note)
	}
}

// docMeta contributes an attribute field through embedding.
type docMeta struct {
	Version string `xml:"version,attr"`
	Lang    string `xml:"ns1:lang,attr"`
}

// TestEmbeddedStructAttributes checks attribute fields promoted from an embed.
func TestEmbeddedStructAttributes(t *testing.T) {
	type doc struct {
		XMLName xml.Name `xml:"doc"`
		docMeta
		Name string `xml:"name"`
	}

	xmlData := []byte(`<doc xmlns:ns1="` + NS1URL + `" version="1.2" ns1:lang="en"><name>a</name></doc>`)

	var d doc
	err := xmlctx.Unmarshal(xmlData, &d,
		xmlctx.WithNamespaces(map[string]string{
			"ns1": NS1URL,
		}),
	)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if d.Version != "1.2" {
		t.Errorf("Version: got %q, want 1.2", d.Version)
	}
	if d.Lang != "en" {
		t.Errorf("Lang: got %q, want en", d.Lang)
	}
	if d.Name != "a" {
		t.Errorf("Name: got %q, want a", d.Name)
	}
}

// Extra and Unused are exported so nil embedded pointers can be allocated.
type Extra struct {
	Comment string `xml:"comment"`
}

type Unused struct {
	Never string `xml:"never-present"`
}

// TestEmbeddedPointerStruct checks a nil embedded pointer is allocated when a
// promoted field matches, and stays nil otherwise.
func TestEmbeddedPointerStruct(t *testing.T) {
	type doc struct {
		XMLName xml.Name `xml:"doc"`
		*Extra
		*Unused
		Name string `xml:"name"`
	}

	xmlData := []byte(`<doc><name>a</name><comment>hello</comment></doc>`)

	var d doc
	err := xmlctx.Unmarshal(xmlData, &d, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if d.Extra == nil {
		t.Fatal("Extra should have been allocated")
	}
	if d.Comment != "hello" {
		t.Errorf("Comment: got %q, want hello", d.Comment)
	}
	if d.Unused != nil {
		t.Errorf("Unused should remain nil, got %+v", d.Unused)
	}
	if d.Name != "a" {
		t.Errorf("Name: got %q, want a", d.Name)
	}
}

// baseA is embedded in baseB, which is embedded in the outer struct.
type baseA struct {
	A string `xml:"a"`
}

type baseB struct {
	baseA
	B string `xml:"b"`
}

type extraC struct {
	C string `xml:"c"`
}

// TestNestedAndMultipleEmbeds checks promotion through nested and sibling embeds.
func TestNestedAndMultipleEmbeds(t *testing.T) {
	type doc struct {
		XMLName xml.Name `xml:"doc"`
		baseB
		extraC
		D string `xml:"d"`
	}

	xmlData := []byte(`<doc><a>1</a><b>2</b><c>3</c><d>4</d></doc>`)

	var d doc
	err := xmlctx.Unmarshal(xmlData, &d, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if d.A != "1" {
		t.Errorf("A: got %q, want 1", d.A)
	}
	if d.B != "2" {
		t.Errorf("B: got %q, want 2", d.B)
	}
	if d.C != "3" {
		t.Errorf("C: got %q, want 3", d.C)
	}
	if d.D != "4" {
		t.Errorf("D: got %q, want 4", d.D)
	}
}

// shadowBase has a tag that conflicts with a field on the outer struct.
type shadowBase struct {
	Name  string `xml:"name"`
	Other string `xml:"other"`
}

// TestEmbeddedShadowedField checks the shallower field wins on a tag conflict.
func TestEmbeddedShadowedField(t *testing.T) {
	type doc struct {
		XMLName xml.Name `xml:"doc"`
		shadowBase
		Name string `xml:"name"`
	}

	xmlData := []byte(`<doc><name>outer</name><other>inner</other></doc>`)

	var d doc
	err := xmlctx.Unmarshal(xmlData, &d, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if d.Name != "outer" {
		t.Errorf("Name: got %q, want outer", d.Name)
	}
	if d.shadowBase.Name != "" {
		t.Errorf("shadowBase.Name should be empty (shadowed), got %q", d.shadowBase.Name)
	}
	if d.Other != "inner" {
		t.Errorf("Other: got %q, want inner", d.Other)
	}
}

// TestTaggedAnonymousFieldNotFlattened checks a tagged anonymous field stays a
// regular named element rather than being flattened.
func TestTaggedAnonymousFieldNotFlattened(t *testing.T) {
	type doc struct {
		XMLName xml.Name `xml:"doc"`
		Extra   `xml:"extra"`
		Name    string `xml:"name"`
	}

	// The comment element lives inside <extra>, not at the top level.
	xmlData := []byte(`<doc><extra><comment>hi</comment></extra><name>a</name></doc>`)

	var d doc
	err := xmlctx.Unmarshal(xmlData, &d, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if d.Comment != "hi" {
		t.Errorf("Comment: got %q, want hi", d.Comment)
	}
	if d.Name != "a" {
		t.Errorf("Name: got %q, want a", d.Name)
	}

	// A top-level <comment> must NOT populate the tagged anonymous field.
	xmlData = []byte(`<doc><comment>hi</comment><name>a</name></doc>`)
	var d2 doc
	err = xmlctx.Unmarshal(xmlData, &d2, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	if d2.Comment != "" {
		t.Errorf("Comment should be empty, got %q", d2.Comment)
	}
}

// TestNonEmbeddedStructUnchanged checks a plain struct with no embeds still
// decodes attributes, elements, chardata, and a named nested struct.
func TestNonEmbeddedStructUnchanged(t *testing.T) {
	type inner struct {
		Value string `xml:",chardata"`
		Kind  string `xml:"kind,attr"`
	}
	type doc struct {
		XMLName xml.Name `xml:"doc"`
		ID      string   `xml:"id,attr"`
		Name    string   `xml:"name"`
		Inner   inner    `xml:"inner"`
		Items   []string `xml:"item"`
	}

	xmlData := []byte(`<doc id="d1"><name>a</name><inner kind="k">v</inner><item>1</item><item>2</item></doc>`)

	var d doc
	err := xmlctx.Unmarshal(xmlData, &d, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if d.ID != "d1" {
		t.Errorf("ID: got %q, want d1", d.ID)
	}
	if d.Name != "a" {
		t.Errorf("Name: got %q, want a", d.Name)
	}
	if d.Inner.Value != "v" || d.Inner.Kind != "k" {
		t.Errorf("Inner: got %+v, want {Value:v Kind:k}", d.Inner)
	}
	if len(d.Items) != 2 || d.Items[0] != "1" || d.Items[1] != "2" {
		t.Errorf("Items: got %v, want [1 2]", d.Items)
	}
}

// diamondA is embedded via two paths in TestDiamondAmbiguityDropped.
type diamondA struct {
	Val string `xml:"val"`
}

type diamondB struct{ diamondA }
type diamondC struct{ diamondA }

// TestDiamondAmbiguityDropped checks that a field reachable via two equally
// shallow embed paths is ambiguous and dropped, matching encoding/xml.
func TestDiamondAmbiguityDropped(t *testing.T) {
	type doc struct {
		XMLName xml.Name `xml:"d"`
		diamondB
		diamondC
	}

	xmlData := []byte(`<d><val>x</val></d>`)

	var d doc
	err := xmlctx.Unmarshal(xmlData, &d, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if d.diamondB.Val != "" {
		t.Errorf("diamondB.Val should be dropped (empty), got %q", d.diamondB.Val)
	}
	if d.diamondC.Val != "" {
		t.Errorf("diamondC.Val should be dropped (empty), got %q", d.diamondC.Val)
	}
}

// deepName is embedded two levels deep in TestShallowShadowsDeep.
type deepName struct {
	Name string `xml:"name"`
}

type midName struct{ deepName }

// TestShallowShadowsDeep checks a depth-0 field shadows a same-tag field
// promoted from a deeper embed.
func TestShallowShadowsDeep(t *testing.T) {
	type doc struct {
		XMLName xml.Name `xml:"d"`
		Name    string   `xml:"name"` // depth 0
		midName          // depth 2 -> deepName.Name
	}

	xmlData := []byte(`<d><name>z</name></d>`)

	var d doc
	err := xmlctx.Unmarshal(xmlData, &d, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if d.Name != "z" {
		t.Errorf("Name: got %q, want z", d.Name)
	}
	if d.midName.Name != "" {
		t.Errorf("deep Name should stay empty (shadowed), got %q", d.midName.Name)
	}
}

// sameInner is embedded both directly and via a wrapper at different depths.
type sameInner struct {
	Val string `xml:"val"`
}

type sameWrap struct{ sameInner }

// TestSameTypeDifferentDepth checks that when the same type is reachable at
// different depths, the shallower instance wins rather than being dropped.
func TestSameTypeDifferentDepth(t *testing.T) {
	type doc struct {
		XMLName   xml.Name `xml:"d"`
		sameInner          // depth 1
		sameWrap           // depth 2 -> sameInner
	}

	xmlData := []byte(`<d><val>y</val></d>`)

	var d doc
	err := xmlctx.Unmarshal(xmlData, &d, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if d.Val != "y" {
		t.Errorf("shallow Val: got %q, want y", d.Val)
	}
	if d.sameWrap.Val != "" {
		t.Errorf("deep sameWrap.Val should stay empty, got %q", d.sameWrap.Val)
	}
}
