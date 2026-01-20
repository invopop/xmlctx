# xmlctx

[![Go Reference](https://pkg.go.dev/badge/github.com/invopop/xmlctx.svg)](https://pkg.go.dev/github.com/invopop/xmlctx)

Namespace-aware XML decoder for Go.

The standard `encoding/xml` package can't properly match struct tags like `xml:"ns1:profile"` because it resolves namespace prefixes to full URIs. This means identical XML documents with different prefixes won't decode correctly.

`xmlctx` fixes this by letting you map struct tag prefixes to namespace URIs, then matching elements based on URI instead of prefix name.

**Note:** This library is for decoding/unmarshaling XML only. For marshaling structs to XML, use the standard `encoding/xml` package.

## Installation

```bash
go get github.com/invopop/xmlctx
```

## Usage

```go
type Person struct {
    XMLName xml.Name `xml:"http://example.com/user person"`
    XmlnsA  string   `xml:"xmlns:addr,attr"`
    Name    string   `xml:"name"`
    Email   string   `xml:"email"`
    City    string   `xml:"addr:city"`
    Country string   `xml:"addr:country"`
}

var person Person
decoder := xmlctx.NewDecoder(
    bytes.NewReader(xmlData),
    xmlctx.WithNamespaces(map[string]string{
        "":     "http://example.com/user",
        "addr": "http://example.com/address",
    }),
)
err := decoder.Decode(&person)
```

The XML can use any prefix (`addr:`, `a:`, `address:`, etc.) as long as it maps to the correct namespace URI.

## Example

These three XML documents all decode the same way:

```xml
<user xmlns:ns1="http://example.com/profile">
  <ns1:bio>Software engineer</ns1:bio>
</user>

<user xmlns:prf="http://example.com/profile">
  <prf:bio>Software engineer</prf:bio>
</user>

<user xmlns="http://example.com/profile">
  <bio>Software engineer</bio>
</user>
```

All work with:

```go
type User struct {
    XMLName xml.Name `xml:"user"`
    XmlnsNS string   `xml:"xmlns:ns1,attr"`
    Bio     string   `xml:"ns1:bio"`
}

decoder := xmlctx.NewDecoder(
    bytes.NewReader(data),
    xmlctx.WithNamespaces(map[string]string{
        "ns1": "http://example.com/profile",
    }),
)
err := decoder.Decode(&user)
```

## What's supported

- Namespace URI matching instead of prefix matching
- Default namespaces
- Nested namespace declarations
- Multiple prefixes for the same namespace
- Namespaced attributes
- String, bool, integer (int, int8-64, uint, uint8-64), pointer, and slice types
- Character data (`,chardata` tag)

## Examples

See the [`examples/`](examples/) directory for complete, runnable programs:

- **basic** - Basic usage with namespace-aware structs
- **different-prefixes** - Same struct decodes XML with different prefixes
- **roundtrip** - Marshal and unmarshal with the same struct

Run them with:
```bash
cd examples/basic && go run main.go
```
