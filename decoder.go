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

// flatField is a struct field, possibly promoted from an anonymous embed.
// depth is the embed nesting level (0 for a direct field). value resolves the
// field lazily, so embedded pointers are allocated only when it is actually
// used.
type flatField struct {
	field reflect.StructField
	depth int
	value func() reflect.Value
}

// flattenedFields returns v's fields plus those promoted from untagged
// anonymous embedded structs and struct pointers, breadth-first so shallower
// fields come first. Cycle detection is per-path (not global), so a type
// reachable via several embed paths yields a field for each path; callers
// disambiguate by depth via dominantField.
func flattenedFields(v reflect.Value) []flatField {
	type level struct {
		t     reflect.Type
		index []int
		depth int
		seen  map[reflect.Type]bool // types on this path, for cycle detection
	}
	var fields []flatField
	root := v.Type()
	queue := []level{{t: root, seen: map[reflect.Type]bool{root: true}}}
	for len(queue) > 0 {
		lvl := queue[0]
		queue = queue[1:]
		for i := 0; i < lvl.t.NumField(); i++ {
			sf := lvl.t.Field(i)
			index := make([]int, len(lvl.index)+1)
			copy(index, lvl.index)
			index[len(lvl.index)] = i

			if sf.Anonymous && sf.Tag.Get("xml") == "" {
				ft := sf.Type
				if ft.Kind() == reflect.Pointer {
					ft = ft.Elem()
				}
				if ft.Kind() == reflect.Struct {
					if lvl.seen[ft] {
						continue // cycle on this path
					}
					// Untagged anonymous struct (or pointer to struct):
					// flatten its fields at the next depth level.
					seen := make(map[reflect.Type]bool, len(lvl.seen)+1)
					for k := range lvl.seen {
						seen[k] = true
					}
					seen[ft] = true
					queue = append(queue, level{t: ft, index: index, depth: lvl.depth + 1, seen: seen})
					continue
				}
			}

			if sf.PkgPath != "" {
				// Unexported, non-embedded field: not settable.
				continue
			}

			fields = append(fields, flatField{
				field: sf,
				depth: lvl.depth,
				value: func() reflect.Value { return fieldByIndexAlloc(v, index) },
			})
		}
	}
	return fields
}

// dominantField returns the value of the uniquely shallowest field matched by
// match. If nothing matches, or two or more match at the same shallowest
// depth (ambiguous), it returns an invalid Value — mirroring how encoding/xml
// drops ambiguous promoted fields.
func dominantField(fields []flatField, match func(ff flatField) bool) reflect.Value {
	best := -1
	bestDepth := 0
	ambiguous := false
	for i, ff := range fields {
		if !match(ff) {
			continue
		}
		if best == -1 || ff.depth < bestDepth {
			best = i
			bestDepth = ff.depth
			ambiguous = false
		} else if ff.depth == bestDepth {
			ambiguous = true
		}
	}
	if best == -1 || ambiguous {
		return reflect.Value{}
	}
	return fields[best].value()
}

// fieldByIndexAlloc is like reflect.Value.FieldByIndex but allocates nil
// embedded pointers so the result is settable, returning an invalid Value if
// one cannot be allocated (e.g. unexported).
func fieldByIndexAlloc(v reflect.Value, index []int) reflect.Value {
	for i, x := range index {
		if i > 0 && v.Kind() == reflect.Pointer {
			if v.IsNil() {
				if !v.CanSet() {
					return reflect.Value{}
				}
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		}
		v = v.Field(x)
	}
	return v
}

// findAllPathFieldsWithPrefix finds all struct fields whose path starts with the given element
func (d *Decoder) findAllPathFieldsWithPrefix(v reflect.Value, start xml.StartElement) []pathFieldInfo {
	elemNS := start.Name.Space
	elemLocal := start.Name.Local

	// Distinct path tags can legitimately share a starting element, so group
	// candidates by full tag and resolve each group by embed depth: shallowest
	// wins, ties (same shallowest depth) are dropped as ambiguous.
	type cand struct {
		depth     int
		value     func() reflect.Value
		ambiguous bool
	}
	byTag := map[string]*cand{}
	var order []string

	for _, ff := range flattenedFields(v) {
		tag := ff.field.Tag.Get("xml")
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
		if !d.matchesField(firstSegment, elemLocal, elemNS) {
			continue
		}

		c, ok := byTag[tagName]
		if !ok {
			byTag[tagName] = &cand{depth: ff.depth, value: ff.value}
			order = append(order, tagName)
		} else if ff.depth < c.depth {
			c.depth = ff.depth
			c.value = ff.value
			c.ambiguous = false
		} else if ff.depth == c.depth {
			c.ambiguous = true
		}
	}

	var matches []pathFieldInfo
	for _, tagName := range order {
		c := byTag[tagName]
		if c.ambiguous {
			continue
		}
		fv := c.value()
		if !fv.IsValid() {
			continue
		}
		matches = append(matches, pathFieldInfo{field: fv, tag: tagName})
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
					if err := enc.Flush(); err != nil {
						return err
					}
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
	return dominantField(flattenedFields(v), func(ff flatField) bool {
		return strings.Contains(ff.field.Tag.Get("xml"), "chardata")
	})
}

// findCDataField finds the struct field marked with ,cdata tag
func (d *Decoder) findCDataField(v reflect.Value) reflect.Value {
	return dominantField(flattenedFields(v), func(ff flatField) bool {
		tag := ff.field.Tag.Get("xml")
		return strings.Contains(tag, "cdata") && !strings.Contains(tag, "chardata")
	})
}

// findInnerXMLField finds the struct field marked with ,innerxml tag
func (d *Decoder) findInnerXMLField(v reflect.Value) reflect.Value {
	return dominantField(flattenedFields(v), func(ff flatField) bool {
		return strings.Contains(ff.field.Tag.Get("xml"), "innerxml")
	})
}

// findAnyField finds the struct field marked with ,any tag
func (d *Decoder) findAnyField(v reflect.Value) reflect.Value {
	return dominantField(flattenedFields(v), func(ff flatField) bool {
		tag := ff.field.Tag.Get("xml")
		// Look for ,any but not ,any,attr
		return strings.Contains(tag, ",any") && !strings.Contains(tag, ",any,attr")
	})
}

// findCommentField finds the struct field marked with ,comment tag
func (d *Decoder) findCommentField(v reflect.Value) reflect.Value {
	return dominantField(flattenedFields(v), func(ff flatField) bool {
		return strings.Contains(ff.field.Tag.Get("xml"), "comment")
	})
}

// setXMLName sets the XMLName field if present in the struct
func (d *Decoder) setXMLName(v reflect.Value, start xml.StartElement) error {
	xmlNameType := reflect.TypeOf(xml.Name{})
	fv := dominantField(flattenedFields(v), func(ff flatField) bool {
		return ff.field.Name == "XMLName" && ff.field.Type == xmlNameType
	})
	if fv.IsValid() && fv.CanSet() {
		fv.Set(reflect.ValueOf(start.Name))
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
	// start.Name.Space contains the full namespace URI (already resolved by xml.Decoder)
	// start.Name.Local contains the local element name
	elemNS := start.Name.Space
	elemLocal := start.Name.Local

	// Search through struct fields, including promoted fields from anonymous
	// embeds. On conflict the shallowest match wins; a tie at the shallowest
	// depth is ambiguous and dropped, matching encoding/xml.
	best := -1
	bestDepth := 0
	bestTag := ""
	ambiguous := false

	fields := flattenedFields(v)
	for i, ff := range fields {
		tag := ff.field.Tag.Get("xml")
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
		if !d.matchesField(firstSegment, elemLocal, elemNS) {
			continue
		}

		if best == -1 || ff.depth < bestDepth {
			best = i
			bestDepth = ff.depth
			bestTag = tagName
			ambiguous = false
		} else if ff.depth == bestDepth {
			ambiguous = true
		}
	}

	if best != -1 && !ambiguous {
		if fv := fields[best].value(); fv.IsValid() {
			return fv, bestTag, nil
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
	fields := flattenedFields(v)
	matchedAttrs := make(map[int]bool) // Track which attrs were matched
	var anyAttrField reflect.Value
	var anyAttrFieldIdx = -1

	// First pass: find the ,any,attr field if present
	for i, ff := range fields {
		tag := ff.field.Tag.Get("xml")
		if tag == "" {
			continue
		}
		// Check for ,any,attr
		if strings.Contains(tag, ",any,attr") {
			anyAttrField = ff.value()
			anyAttrFieldIdx = i
			break
		}
	}

	// Second pass: gather attribute fields grouped by attr tag, resolving
	// promoted-field conflicts by embed depth (shallowest wins; a tie at the
	// shallowest depth is ambiguous and dropped, matching encoding/xml).
	type attrCand struct {
		depth     int
		value     func() reflect.Value
		ambiguous bool
	}
	byName := map[string]*attrCand{}
	var order []string
	for i, ff := range fields {
		if i == anyAttrFieldIdx {
			continue // Skip the ,any,attr field in this pass
		}

		tag := ff.field.Tag.Get("xml")
		if tag == "" || !strings.Contains(tag, "attr") {
			continue
		}

		// Skip ,any,attr which was handled above
		if strings.Contains(tag, ",any,attr") {
			continue
		}

		// Parse attribute tag (e.g., "id,attr" or "xmlns:ns1,attr")
		attrName := strings.Split(tag, ",")[0]

		c, ok := byName[attrName]
		if !ok {
			byName[attrName] = &attrCand{depth: ff.depth, value: ff.value}
			order = append(order, attrName)
		} else if ff.depth < c.depth {
			c.depth = ff.depth
			c.value = ff.value
			c.ambiguous = false
		} else if ff.depth == c.depth {
			c.ambiguous = true
		}
	}

	// Apply the resolved attribute fields.
	for _, attrName := range order {
		c := byName[attrName]
		if c.ambiguous {
			continue
		}
		for attrIdx, attr := range attrs {
			if d.matchesAttribute(attrName, attr) {
				fv := c.value()
				if !fv.IsValid() {
					break
				}
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

	// Handle xmlns namespace declarations specially
	// xmlns:prefix="uri" becomes attr.Name.Space="xmlns" attr.Name.Local="prefix"
	// xmlns="uri" becomes attr.Name.Space="" attr.Name.Local="xmlns"
	if strings.HasPrefix(tag, "xmlns") {
		if tag == "xmlns" {
			// Match plain xmlns attribute
			return attr.Name.Local == "xmlns" && attr.Name.Space == ""
		} else if prefix, ok := strings.CutPrefix(tag, "xmlns:"); ok {
			// Match xmlns:prefix attribute
			return attr.Name.Local == prefix && attr.Name.Space == "xmlns"
		}
	}

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
