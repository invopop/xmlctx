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
//	decoder := xmlctx.NewDecoder(
//	    bytes.NewReader(xmlData),
//	    xmlctx.WithNamespaces(map[string]string{
//	        "":     "http://example.com/user",
//	        "addr": "http://example.com/address",
//	    }),
//	)
//	err := decoder.Decode(&person)
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

// Unmarshal decodes XML with namespace context awareness
func Unmarshal(data []byte, v any, opts ...Option) error {
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
			return d.decodeElement(d.decoder, rv.Elem(), start)
		}
	}
}

// decodeElement decodes an XML element into a reflect.Value
func (d *Decoder) decodeElement(decoder *xml.Decoder, v reflect.Value, start xml.StartElement) error {
	// xml.Decoder has already resolved start.Name.Space to the full URI
	// start.Name.Local contains the local name without prefix

	// Check if the type implements xml.Unmarshaler
	if v.CanAddr() {
		pv := v.Addr()
		if pv.CanInterface() {
			if u, ok := pv.Interface().(xml.Unmarshaler); ok {
				// Use custom unmarshaler
				return u.UnmarshalXML(decoder, start)
			}
		}
	}

	// Check if the type implements encoding.TextUnmarshaler (for simple values)
	if v.CanAddr() {
		pv := v.Addr()
		if pv.CanInterface() {
			if u, ok := pv.Interface().(interface{ UnmarshalText([]byte) error }); ok {
				// Read the element content as text
				var text strings.Builder
				for {
					tok, err := decoder.Token()
					if err == io.EOF {
						break
					}
					if err != nil {
						return err
					}
					switch t := tok.(type) {
					case xml.CharData:
						text.Write(t)
					case xml.EndElement:
						return u.UnmarshalText([]byte(strings.TrimSpace(text.String())))
					case xml.StartElement:
						// Skip nested elements
						if err := decoder.Skip(); err != nil {
							return err
						}
					}
				}
				return nil
			}
		}
	}

	switch v.Kind() {
	case reflect.Pointer:
		// Initialize pointer if nil
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		// Decode into the element the pointer points to
		return d.decodeElement(decoder, v.Elem(), start)
	case reflect.Struct:
		return d.decodeStruct(decoder, v, start)
	case reflect.String:
		return d.decodeString(decoder, v)
	case reflect.Bool:
		return d.decodeBool(decoder, v)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return d.decodeInt(decoder, v)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return d.decodeUint(decoder, v)
	case reflect.Slice:
		// For slices, create a new element and decode into it
		elemType := v.Type().Elem()
		elem := reflect.New(elemType).Elem()
		if err := d.decodeElement(decoder, elem, start); err != nil {
			return err
		}
		v.Set(reflect.Append(v, elem))
		return nil
	default:
		return fmt.Errorf("unsupported type: %v", v.Kind())
	}
}


// pathFieldInfo holds information about a struct field with path syntax
type pathFieldInfo struct {
	field reflect.Value
	tag   string
}

// findAllPathFieldsWithPrefix finds all struct fields whose path starts with the given element
func (d *Decoder) findAllPathFieldsWithPrefix(v reflect.Value, start xml.StartElement) []pathFieldInfo {
	t := v.Type()
	elemNS := start.Name.Space
	elemLocal := start.Name.Local

	var matches []pathFieldInfo

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("xml")
		if tag == "" || tag == "-" {
			continue
		}

		// Parse the tag
		tagParts := strings.Split(tag, ",")
		tagName := tagParts[0]

		// Skip special fields
		if len(tagParts) > 1 {
			if tagParts[1] == "attr" || tagParts[1] == "chardata" {
				continue
			}
		}
		if strings.Contains(tag, "attr") || strings.HasPrefix(tagName, "xmlns") {
			continue
		}

		// Check if this is a path field
		if !strings.Contains(tagName, ">") {
			continue
		}

		// Get first segment
		pathSegments := strings.Split(tagName, ">")
		firstSegment := pathSegments[0]

		// Check if first segment matches the element
		if d.matchesField(firstSegment, elemLocal, elemNS) {
			matches = append(matches, pathFieldInfo{
				field: v.Field(i),
				tag:   tagName,
			})
		}
	}

	return matches
}

// decodeMultiplePathFields decodes multiple fields that share the same parent path element
func (d *Decoder) decodeMultiplePathFields(decoder *xml.Decoder, pathFields []pathFieldInfo) error {
	// Track which fields have been decoded
	foundFields := make([]bool, len(pathFields))

	// Navigate through the parent element
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			elemNS := t.Name.Space
			elemLocal := t.Name.Local

			// Find all fields whose next segment matches this element
			var matchingFields []pathFieldInfo
			var matchingIndices []int
			matchedAny := false

			for i, pf := range pathFields {
				if foundFields[i] {
					continue
				}

				// Split path and get segments
				pathSegments := strings.Split(pf.tag, ">")
				if len(pathSegments) < 2 {
					continue
				}

				nextSegment := pathSegments[1]

				if d.matchesField(nextSegment, elemLocal, elemNS) {
					matchedAny = true
					if len(pathSegments) == 2 {
						// This is the final segment - decode into the field
						if err := d.decodeElement(decoder, pf.field, t); err != nil {
							return err
						}
						foundFields[i] = true
					} else {
						// More segments remaining - collect for recursive processing
						remainingPath := strings.Join(pathSegments[1:], ">")
						matchingFields = append(matchingFields, pathFieldInfo{
							field: pf.field,
							tag:   remainingPath,
						})
						matchingIndices = append(matchingIndices, i)
					}
				}
			}

			// If we have fields with deeper paths, recursively process them
			if len(matchingFields) > 0 {
				if err := d.decodeMultiplePathFields(decoder, matchingFields); err != nil {
					return err
				}
				// Mark all matching fields as found
				for _, idx := range matchingIndices {
					foundFields[idx] = true
				}
			} else if !matchedAny {
				// No fields matched this element - skip it
				if err := decoder.Skip(); err != nil {
					return err
				}
			}

		case xml.EndElement:
			// Reached end of parent element
			return nil
		}
	}

	return nil
}

// decodeStruct decodes an XML element into a struct
func (d *Decoder) decodeStruct(decoder *xml.Decoder, v reflect.Value, start xml.StartElement) error {
	// First, set XMLName field if present
	if err := d.setXMLName(v, start); err != nil {
		return err
	}

	// Then, decode attributes
	if err := d.decodeAttributes(v, start.Attr); err != nil {
		return err
	}

	// Find special fields
	chardataField := d.findChardataField(v)
	cdataField := d.findCDataField(v)
	innerXMLField := d.findInnerXMLField(v)
	anyField := d.findAnyField(v)
	commentField := d.findCommentField(v)

	// If innerxml is present, capture all inner content as raw XML
	if innerXMLField.IsValid() {
		var buf strings.Builder
		enc := xml.NewEncoder(&buf)
		depth := 0

		for {
			tok, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}

			switch t := tok.(type) {
			case xml.StartElement:
				depth++
				if err := enc.EncodeToken(t); err != nil {
					return err
				}
			case xml.EndElement:
				if depth == 0 {
					// End of parent element
					enc.Flush()
					content := buf.String()
					if innerXMLField.Kind() == reflect.String {
						innerXMLField.SetString(content)
					} else if innerXMLField.Kind() == reflect.Slice && innerXMLField.Type().Elem().Kind() == reflect.Uint8 {
						innerXMLField.SetBytes([]byte(content))
					}
					return nil
				}
				depth--
				if err := enc.EncodeToken(t); err != nil {
					return err
				}
			case xml.CharData, xml.Comment, xml.ProcInst, xml.Directive:
				if err := enc.EncodeToken(xml.CopyToken(t)); err != nil {
					return err
				}
			}
		}
		return nil
	}

	// Accumulate character data and comments
	var chardata strings.Builder
	var comments strings.Builder

	// Then decode child elements
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		switch tok := tok.(type) {
		case xml.StartElement:
			// Check if this element is the start of any path fields
			pathFields := d.findAllPathFieldsWithPrefix(v, tok)

			if len(pathFields) > 0 {
				// Decode all path fields from within this element
				if err := d.decodeMultiplePathFields(decoder, pathFields); err != nil {
					return err
				}
				continue
			}

			// Find matching field in struct (non-path fields only at this point)
			field, _, err := d.findFieldWithTag(v, tok)
			if err != nil {
				// Element doesn't match any field
				// Try to decode into ,any field if present
				if anyField.IsValid() {
					if err := d.decodeAnyElement(decoder, anyField, tok); err != nil {
						return err
					}
					continue
				}
				// Skip unknown elements
				if err := decoder.Skip(); err != nil {
					return err
				}
				continue
			}

			// Decode into the field normally
			// Note: path fields are already handled above by findAllPathFieldsWithPrefix
			if err := d.decodeElement(decoder, field, tok); err != nil {
				return err
			}

		case xml.CharData:
			// Accumulate character data for chardata or cdata field
			if chardataField.IsValid() {
				chardata.Write(tok)
			} else if cdataField.IsValid() {
				chardata.Write(tok)
			}

		case xml.Comment:
			// Accumulate comments for comment field
			if commentField.IsValid() {
				if comments.Len() > 0 {
					comments.WriteString("\n")
				}
				comments.Write(tok)
			}

		case xml.EndElement:
			// Set chardata field if it exists
			if chardataField.IsValid() && chardata.Len() > 0 {
				chardataField.SetString(strings.TrimSpace(chardata.String()))
			} else if cdataField.IsValid() && chardata.Len() > 0 {
				// Set cdata field (cdata and chardata are mutually exclusive)
				cdataField.SetString(strings.TrimSpace(chardata.String()))
			}
			// Set comment field if it exists
			if commentField.IsValid() && comments.Len() > 0 {
				commentField.SetString(strings.TrimSpace(comments.String()))
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

// findCDataField finds the struct field marked with ,cdata tag
func (d *Decoder) findCDataField(v reflect.Value) reflect.Value {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("xml")
		if tag == "" {
			continue
		}
		// Check if this is a cdata field (e.g., ",cdata")
		if strings.Contains(tag, "cdata") && !strings.Contains(tag, "chardata") {
			return v.Field(i)
		}
	}
	return reflect.Value{}
}

// findInnerXMLField finds the struct field marked with ,innerxml tag
func (d *Decoder) findInnerXMLField(v reflect.Value) reflect.Value {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("xml")
		if tag == "" {
			continue
		}
		if strings.Contains(tag, "innerxml") {
			return v.Field(i)
		}
	}
	return reflect.Value{}
}

// findAnyField finds the struct field marked with ,any tag
func (d *Decoder) findAnyField(v reflect.Value) reflect.Value {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("xml")
		if tag == "" {
			continue
		}
		// Look for ,any but not ,any,attr
		if strings.Contains(tag, ",any") && !strings.Contains(tag, ",any,attr") {
			return v.Field(i)
		}
	}
	return reflect.Value{}
}

// findCommentField finds the struct field marked with ,comment tag
func (d *Decoder) findCommentField(v reflect.Value) reflect.Value {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("xml")
		if tag == "" {
			continue
		}
		if strings.Contains(tag, "comment") {
			return v.Field(i)
		}
	}
	return reflect.Value{}
}

// setXMLName sets the XMLName field if present in the struct
func (d *Decoder) setXMLName(v reflect.Value, start xml.StartElement) error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		// Look for a field named XMLName of type xml.Name
		if field.Name == "XMLName" && field.Type == reflect.TypeOf(xml.Name{}) {
			v.Field(i).Set(reflect.ValueOf(start.Name))
			return nil
		}
	}
	return nil
}

// decodeAnyElement decodes an unmatched element into the ,any field
func (d *Decoder) decodeAnyElement(decoder *xml.Decoder, v reflect.Value, start xml.StartElement) error {
	// For ,any fields, we typically store them as interface{} or in a slice
	// We'll decode it generically as a map or skip it for now
	// The standard library uses xml.Token slices, but for simplicity we'll decode to a generic struct

	// If the field is a slice, we can append elements to it
	if v.Kind() == reflect.Slice {
		// Create a new element of the slice's element type
		elemType := v.Type().Elem()
		elem := reflect.New(elemType).Elem()

		// Try to decode into the element
		if err := d.decodeElement(decoder, elem, start); err != nil {
			// If decoding fails, just skip this element
			return decoder.Skip()
		}

		v.Set(reflect.Append(v, elem))
		return nil
	}

	// For non-slice fields, try to decode directly
	if v.CanSet() {
		// Initialize if it's a pointer and nil
		if v.Kind() == reflect.Pointer && v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}

		targetVal := v
		if v.Kind() == reflect.Pointer {
			targetVal = v.Elem()
		}

		return d.decodeElement(decoder, targetVal, start)
	}

	// If we can't set it, just skip the element
	return decoder.Skip()
}

// findFieldWithTag finds the struct field that matches the XML element and returns the field and its tag
func (d *Decoder) findFieldWithTag(v reflect.Value, start xml.StartElement) (reflect.Value, string, error) {
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

		// Handle path syntax (e.g., "ram:OriginTradeCountry>ram:ID")
		// For matching, we only check the first segment
		firstSegment := tagName
		if strings.Contains(tagName, ">") {
			pathSegments := strings.Split(tagName, ">")
			firstSegment = pathSegments[0]
		}

		// Check if this field matches the element
		if d.matchesField(firstSegment, elemLocal, elemNS) {
			return v.Field(i), tagName, nil
		}
	}

	return reflect.Value{}, "", fmt.Errorf("no field found for element %s (ns: %s)", elemLocal, elemNS)
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
	matchedAttrs := make(map[int]bool) // Track which attrs were matched
	var anyAttrField reflect.Value
	var anyAttrFieldIdx int = -1

	// First pass: find the ,any,attr field if present
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("xml")
		if tag == "" {
			continue
		}
		// Check for ,any,attr
		if strings.Contains(tag, ",any,attr") {
			anyAttrField = v.Field(i)
			anyAttrFieldIdx = i
			break
		}
	}

	// Second pass: match specific attributes
	for i := 0; i < t.NumField(); i++ {
		if i == anyAttrFieldIdx {
			continue // Skip the ,any,attr field in this pass
		}

		field := t.Field(i)
		tag := field.Tag.Get("xml")
		if tag == "" || !strings.Contains(tag, "attr") {
			continue
		}

		// Skip ,any,attr which was handled above
		if strings.Contains(tag, ",any,attr") {
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
		for attrIdx, attr := range attrs {
			if d.matchesAttribute(attrName, attr) {
				// Set the field value
				fv := v.Field(i)
				if err := d.setFieldValue(fv, attr.Value); err != nil {
					return err
				}
				matchedAttrs[attrIdx] = true
				break
			}
		}
	}

	// Third pass: collect unmatched attributes into ,any,attr field
	if anyAttrField.IsValid() && anyAttrField.CanSet() {
		var unmatchedAttrs []xml.Attr
		for i, attr := range attrs {
			if !matchedAttrs[i] {
				unmatchedAttrs = append(unmatchedAttrs, attr)
			}
		}

		if len(unmatchedAttrs) > 0 {
			// The field should be []xml.Attr
			if anyAttrField.Type() == reflect.TypeOf([]xml.Attr{}) {
				anyAttrField.Set(reflect.ValueOf(unmatchedAttrs))
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
	// Check if the type implements xml.UnmarshalerAttr
	if v.CanAddr() {
		pv := v.Addr()
		if pv.CanInterface() {
			if u, ok := pv.Interface().(xml.UnmarshalerAttr); ok {
				// Use custom attribute unmarshaler
				return u.UnmarshalXMLAttr(xml.Attr{Value: s})
			}
		}
	}

	// Check if the type implements encoding.TextUnmarshaler
	if v.CanAddr() {
		pv := v.Addr()
		if pv.CanInterface() {
			if u, ok := pv.Interface().(interface{ UnmarshalText([]byte) error }); ok {
				return u.UnmarshalText([]byte(s))
			}
		}
	}

	// Handle pointer types
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		return d.setFieldValue(v.Elem(), s)
	}

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
func (d *Decoder) decodeString(decoder *xml.Decoder, v reflect.Value) error {
	var s strings.Builder
	for {
		tok, err := decoder.Token()
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
func (d *Decoder) decodeBool(decoder *xml.Decoder, v reflect.Value) error {
	var s strings.Builder
	for {
		tok, err := decoder.Token()
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

// decodeInt decodes character data into an int field
func (d *Decoder) decodeInt(decoder *xml.Decoder, v reflect.Value) error {
	var s strings.Builder
	for {
		tok, err := decoder.Token()
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
			i, err := strconv.ParseInt(str, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse integer: %w", err)
			}
			v.SetInt(i)
			return nil
		}
	}
	return nil
}

// decodeUint decodes character data into a uint field
func (d *Decoder) decodeUint(decoder *xml.Decoder, v reflect.Value) error {
	var s strings.Builder
	for {
		tok, err := decoder.Token()
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
			i, err := strconv.ParseUint(str, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse unsigned integer: %w", err)
			}
			v.SetUint(i)
			return nil
		}
	}
	return nil
}
