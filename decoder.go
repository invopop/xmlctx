// Package xmlctx provides a namespace-aware XML decoder for Go.
//
// The standard encoding/xml package can decode namespaced XML, but it cannot
// properly match namespace-aware struct tags because it resolves prefixes to
// full URIs in StartElement.Name.Space, while struct tags use prefixes.
//
// This package solves this problem by allowing you to specify a namespace
// context that maps prefixes (used in struct tags) to their full namespace URIs.
// The decoder then matches XML elements based on their namespace URI, regardless
// of what prefix is used in the actual XML document.
//
// Note: This package is for decoding/unmarshaling XML only. For marshaling
// structs to XML, use the standard encoding/xml package.
//
// Example usage:
//
//	type Person struct {
//	    Name  string `xml:"name"`
//	    Email string `xml:"addr:email"`
//	}
//
//	err := xmlctx.Parse(xmlData, &person,
//	    xmlctx.WithNamespaces(map[string]string{
//	        "":     "http://example.com/user",
//	        "addr": "http://example.com/address",
//	    }),
//	)
package xmlctx

import (
	"encoding/xml"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
)

// Decoder wraps xml.Decoder with namespace context awareness
type Decoder struct {
	decoder    *xml.Decoder
	namespaces map[string]string
}

// Option is a functional option for configuring the Decoder
type Option func(*Decoder)

// WithNamespaces sets the namespace mappings for the decoder
// The map keys are prefixes used in Go struct tags (e.g., "ns1", "ns2", "")
// The map values are the full namespace URIs (e.g., "http://example.com/schema/profile")
func WithNamespaces(namespaces map[string]string) Option {
	return func(d *Decoder) {
		d.namespaces = namespaces
	}
}

// NewDecoder creates a new namespace-aware decoder
func NewDecoder(r io.Reader, opts ...Option) *Decoder {
	d := &Decoder{
		decoder: xml.NewDecoder(r),
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Parse decodes XML with namespace context awareness
func Parse(data []byte, v any, opts ...Option) error {
	r := strings.NewReader(string(data))
	dec := NewDecoder(r, opts...)
	return dec.Decode(v)
}

// Decode decodes the XML into the provided value
func (d *Decoder) Decode(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("decode target must be a non-nil pointer")
	}

	// Read tokens until we find the root element
	for {
		tok, err := d.decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		if start, ok := tok.(xml.StartElement); ok {
			return d.decodeElement(rv.Elem(), start)
		}
	}
}

// decodeElement decodes an XML element into a reflect.Value
func (d *Decoder) decodeElement(v reflect.Value, start xml.StartElement) error {
	// xml.Decoder has already resolved start.Name.Space to the full URI
	// start.Name.Local contains the local name without prefix

	switch v.Kind() {
	case reflect.Struct:
		return d.decodeStruct(v, start)
	case reflect.String:
		return d.decodeString(v)
	case reflect.Bool:
		return d.decodeBool(v)
	case reflect.Slice:
		// For slices, create a new element and decode into it
		elemType := v.Type().Elem()
		elem := reflect.New(elemType).Elem()
		if err := d.decodeElement(elem, start); err != nil {
			return err
		}
		v.Set(reflect.Append(v, elem))
		return nil
	default:
		return fmt.Errorf("unsupported type: %v", v.Kind())
	}
}

// decodeStruct decodes an XML element into a struct
func (d *Decoder) decodeStruct(v reflect.Value, start xml.StartElement) error {
	// First, decode attributes
	if err := d.decodeAttributes(v, start.Attr); err != nil {
		return err
	}

	// Find chardata field if it exists
	chardataField := d.findChardataField(v)

	// Accumulate character data
	var chardata strings.Builder

	// Then decode child elements
	for {
		tok, err := d.decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		switch tok := tok.(type) {
		case xml.StartElement:
			// Find matching field in struct
			field, err := d.findField(v, tok)
			if err != nil {
				// Skip unknown elements
				if err := d.decoder.Skip(); err != nil {
					return err
				}
				continue
			}

			// Decode into the field
			if err := d.decodeElement(field, tok); err != nil {
				return err
			}

		case xml.CharData:
			// Accumulate character data for chardata field
			if chardataField.IsValid() {
				chardata.Write(tok)
			}

		case xml.EndElement:
			// Set chardata field if it exists
			if chardataField.IsValid() && chardata.Len() > 0 {
				chardataField.SetString(strings.TrimSpace(chardata.String()))
			}
			// End of this struct
			return nil
		}
	}

	return nil
}

// findChardataField finds the struct field marked with ,chardata tag
func (d *Decoder) findChardataField(v reflect.Value) reflect.Value {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("xml")
		if tag == "" {
			continue
		}
		// Check if this is a chardata field (e.g., ",chardata")
		if strings.Contains(tag, "chardata") {
			return v.Field(i)
		}
	}
	return reflect.Value{}
}

// findField finds the struct field that matches the XML element
func (d *Decoder) findField(v reflect.Value, start xml.StartElement) (reflect.Value, error) {
	t := v.Type()

	// start.Name.Space contains the full namespace URI (already resolved by xml.Decoder)
	// start.Name.Local contains the local element name
	elemNS := start.Name.Space
	elemLocal := start.Name.Local

	// Search through struct fields
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("xml")
		if tag == "" || tag == "-" {
			continue
		}

		// Parse the tag
		tagParts := strings.Split(tag, ",")
		tagName := tagParts[0]

		// Skip special fields (attributes, chardata, etc.)
		if len(tagParts) > 1 {
			if tagParts[1] == "attr" || tagParts[1] == "chardata" || strings.HasPrefix(tagParts[0], "xmlns") {
				continue
			}
		}
		if strings.Contains(tag, "attr") || strings.HasPrefix(tagName, "xmlns") {
			continue
		}

		// Check if this field matches the element
		if d.matchesField(tagName, elemLocal, elemNS) {
			return v.Field(i), nil
		}
	}

	return reflect.Value{}, fmt.Errorf("no field found for element %s (ns: %s)", elemLocal, elemNS)
}

// matchesField checks if a struct tag matches an element
func (d *Decoder) matchesField(tag, elemLocal, elemNS string) bool {
	// Handle tags like "ns1:profile"
	if strings.Contains(tag, ":") {
		parts := strings.SplitN(tag, ":", 2)
		tagPrefix := parts[0]
		tagLocal := parts[1]

		// Look up the expected namespace URL for this prefix
		expectedNS, ok := d.namespaces[tagPrefix]
		if !ok {
			// Unknown prefix in tag
			return false
		}

		// Match: local name must match AND namespace URL must match
		return tagLocal == elemLocal && expectedNS == elemNS
	}

	// For tags without prefix (e.g., "name", "email")
	// Match if local names match and element is in default namespace
	if tag != elemLocal {
		return false
	}

	// Check if element is in default namespace
	defaultNS, hasDefault := d.namespaces[""]
	if hasDefault {
		return elemNS == defaultNS
	}

	// If no default namespace in context, match if element has no namespace
	return elemNS == ""
}

// decodeAttributes decodes XML attributes into struct fields
func (d *Decoder) decodeAttributes(v reflect.Value, attrs []xml.Attr) error {
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("xml")
		if tag == "" || !strings.Contains(tag, "attr") {
			continue
		}

		// Parse attribute tag (e.g., "id,attr" or "xmlns:ns1,attr")
		tagParts := strings.Split(tag, ",")
		attrName := tagParts[0]

		// Skip xmlns declarations (they're handled by xml.Decoder)
		if strings.HasPrefix(attrName, "xmlns") {
			continue
		}

		// Find matching attribute
		for _, attr := range attrs {
			if d.matchesAttribute(attrName, attr) {
				// Set the field value
				fv := v.Field(i)
				if err := d.setFieldValue(fv, attr.Value); err != nil {
					return err
				}
				break
			}
		}
	}

	return nil
}

// matchesAttribute checks if a struct tag matches an attribute
func (d *Decoder) matchesAttribute(tag string, attr xml.Attr) bool {
	// attr.Name.Space contains the namespace URI (if any)
	// attr.Name.Local contains the attribute name

	// Handle namespaced attributes like "ns1:visibility"
	if strings.Contains(tag, ":") {
		parts := strings.SplitN(tag, ":", 2)
		tagPrefix := parts[0]
		tagLocal := parts[1]

		// Look up expected namespace for prefix
		expectedNS, ok := d.namespaces[tagPrefix]
		if !ok {
			return false
		}

		return tagLocal == attr.Name.Local && expectedNS == attr.Name.Space
	}

	// For non-namespaced attributes, just match the local name
	return tag == attr.Name.Local
}

// setFieldValue sets a field value from a string
func (d *Decoder) setFieldValue(v reflect.Value, s string) error {
	switch v.Kind() {
	case reflect.String:
		v.SetString(s)
	case reflect.Bool:
		v.SetBool(s == "true")
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse integer: %w", err)
		}
		v.SetInt(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		i, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse unsigned integer: %w", err)
		}
		v.SetUint(i)
	default:
		return fmt.Errorf("unsupported field type: %v", v.Kind())
	}
	return nil
}

// decodeString decodes character data into a string field
func (d *Decoder) decodeString(v reflect.Value) error {
	var s strings.Builder
	for {
		tok, err := d.decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		switch t := tok.(type) {
		case xml.CharData:
			s.Write(t)
		case xml.EndElement:
			v.SetString(strings.TrimSpace(s.String()))
			return nil
		}
	}
	return nil
}

// decodeBool decodes character data into a bool field
func (d *Decoder) decodeBool(v reflect.Value) error {
	var s strings.Builder
	for {
		tok, err := d.decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		switch t := tok.(type) {
		case xml.CharData:
			s.Write(t)
		case xml.EndElement:
			str := strings.TrimSpace(s.String())
			v.SetBool(str == "true")
			return nil
		}
	}
	return nil
}
