# xmlctx Examples

This directory contains runnable examples demonstrating how to use xmlctx.

## Running the Examples

```bash
# Basic usage
cd basic && go run main.go

# Different prefixes in XML
cd different-prefixes && go run main.go

# Round-trip marshal and unmarshal
cd roundtrip && go run main.go
```

## Examples

### basic/
Demonstrates basic usage of xmlctx to decode namespaced XML. The struct uses `addr:` prefixes in tags and includes namespace declarations for proper marshaling with `encoding/xml`.

**Key points:**
- Uses `XMLName` to set the root element and namespace
- Declares `xmlns:addr` attribute for marshaling
- Uses prefix-based tags (`addr:city`) that work with xmlctx

### different-prefixes/
Shows that xmlctx matches elements based on namespace URI, not the prefix name. The XML uses `a:` as the prefix, but the struct tags use `addr:`, and decoding works correctly because both map to the same namespace URI.

**Key points:**
- XML uses `a:city` and `a:country`
- Struct tags use `addr:city` and `addr:country`
- Both work because they map to `http://example.com/address`

### roundtrip/
Demonstrates using the same struct for both marshaling (with `encoding/xml`) and unmarshaling (with `xmlctx`).

**Key points:**
- Marshal with `encoding/xml`
- Unmarshal with `xmlctx`
- Single struct definition works for both
- Includes namespace declarations (`xmlns`, `xmlns:addr`)

This pattern lets you maintain one struct for both encoding and decoding namespaced XML.
