# Invalid Namespace Test Files

This directory contains XML files demonstrating various invalid namespace usage patterns. These test files verify that the decoder correctly rejects or ignores elements with incorrect namespace configurations.

## Test Files and Scenarios

### 01_xmlns_literal_ns1.xml
**Issue:** Uses `xmlns="ns1"` which creates a literal namespace `"ns1"`

The pattern `xmlns="ns1"` does NOT reference the URI that the `ns1` prefix maps to. Instead, it creates a literal namespace with the string `"ns1"` as the URI.

**Correct:**
```xml
<user xmlns:ns1="http://example.com/schema/profile">
  <ns1:profile>...</ns1:profile>
</user>
```

**Invalid:**
```xml
<user xmlns:ns1="http://example.com/schema/profile">
  <profile xmlns="ns1">...</profile>  <!-- literal namespace "ns1" -->
</user>
```

### 02_wrong_namespace_uri.xml
**Issue:** Profile namespace uses wrong URI

The `ns1` prefix is declared with `http://example.com/schema/WRONG` instead of `http://example.com/schema/profile`, so profile elements won't match.

### 03_undeclared_prefix.xml
**Issue:** Uses prefixes that are never declared

Elements use `prf:` and `addr:` prefixes, but these are never declared with `xmlns:prf` or `xmlns:addr`. Go's XML parser treats undeclared prefixes as literal namespace URIs, so `prf:profile` creates namespace `"prf"` (the literal string).

### 04_swapped_namespaces.xml
**Issue:** Namespace URIs are swapped

`ns1` is declared with the address URI and `ns2` is declared with the profile URI, causing all namespace matching to fail.

### 05_empty_namespace.xml
**Issue:** Uses `xmlns=""` for profile elements

Profile elements explicitly set the default namespace to empty (`xmlns=""`), putting them in no namespace when they should be in the profile namespace.

### 06_namespace_typo.xml
**Issue:** Typo in namespace URI

The profile namespace is declared as `http://example.com/schema/proflie` (typo: "proflie" instead of "profile").

### 07_wrong_default_namespace.xml
**Issue:** Wrong default namespace on root

The root element uses the profile namespace as default instead of the user namespace, so unprefixed elements like `<name>` and `<email>` won't match.

### 08_mixed_xmlns_literal.xml
**Issue:** Mixes correct and incorrect patterns

- Profile elements use correct `ns1:` prefix (these work)
- Settings/metadata use `xmlns="ns1"` literal namespace (these fail)

This demonstrates partial parsing where some elements work and others don't.

### 09_no_namespace_declarations.xml
**Issue:** No namespace declarations at all

All elements are unprefixed with no namespace declarations, so they have no namespace. Only attributes (which don't require namespaces) will decode.

### 10_http_vs_https.xml
**Issue:** Uses `https://` instead of `http://` in URIs

Namespace URIs are exact string matches. `https://example.com/schema/user` is completely different from `http://example.com/schema/user`.

## Expected Behavior

When decoded with xmlctx using the correct namespace mappings:

**All files**: Parse without error, but elements with wrong namespaces are skipped (fields remain empty/zero values)

The `TestInvalidNamespaces` test verifies each scenario with specific assertions about which fields should be empty.
