package xmlctx_test

import (
	"encoding/xml"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/invopop/xmlctx"
)

const (
	// Namespace URLs
	DefaultNS = "http://example.com/schema/user"
	NS1URL    = "http://example.com/schema/profile"
	NS2URL    = "http://example.com/schema/address"
)

// Helper functions to create pointers
func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }

// User represents the root structure with multiple namespaces
type User struct {
	XMLName xml.Name `xml:"user"`

	// Namespace declarations
	XMLNS string `xml:"xmlns,attr"`
	NS1   string `xml:"xmlns:ns1,attr"`
	NS2   string `xml:"xmlns:ns2,attr"`

	// Attribute on the root element
	ID      string `xml:"id,attr"`
	Version string `xml:"version,attr"`

	// Default namespace fields
	Name  string `xml:"name"`
	Email string `xml:"email"`

	// ns1:namespace fields
	Profile  Profile  `xml:"ns1:profile"`
	Settings Settings `xml:"ns1:settings"`

	// ns2:namespace fields
	Address  *Address `xml:"ns2:address"`
	Metadata Metadata `xml:"ns2:metadata"`
}

// Profile represents user profile information in ns1:namespace
type Profile struct {
	// Attributes
	Visibility string `xml:"visibility,attr"`
	Verified   bool   `xml:"verified,attr"`

	// Fields
	Bio       string   `xml:"ns1:bio"`
	AvatarURL string   `xml:"ns1:avatar-url"`
	Tags      []string `xml:"ns1:tag"`
}

// Settings represents user settings in ns1:namespace
type Settings struct {
	Theme        string               `xml:"ns1:theme"`
	Language     string               `xml:"ns1:language"`
	Notification NotificationSettings `xml:"ns1:notification"`
}

// NotificationSettings is a nested structure within Settings
type NotificationSettings struct {
	Enabled   bool   `xml:"enabled,attr"`
	Email     bool   `xml:"ns1:email"`
	Push      bool   `xml:"ns1:push"`
	Frequency string `xml:"ns1:frequency"`
}

// Address represents address information in ns2:namespace
type Address struct {
	// Attributes
	Type    *string `xml:"type,attr"`
	Primary *bool   `xml:"primary,attr"`

	// Fields
	Street  string `xml:"ns2:street"`
	City    string `xml:"ns2:city"`
	State   string `xml:"ns2:state"`
	ZipCode string `xml:"ns2:zip-code"`
	Country string `xml:"ns2:country"`
}

// Metadata represents additional metadata in ns2:namespace
type Metadata struct {
	CreatedAt    string        `xml:"ns2:created-at"`
	UpdatedAt    string        `xml:"ns2:updated-at"`
	Source       string        `xml:"ns2:source"`
	CustomFields []CustomField `xml:"ns2:custom-field"`
}

// CustomField represents a custom key-value field
type CustomField struct {
	Key   string `xml:"key,attr"`
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

func TestMarshalUser(t *testing.T) {
	user := User{
		// Namespace declarations
		XMLNS: DefaultNS,
		NS1:   NS1URL,
		NS2:   NS2URL,

		// Attributes
		ID:      "user-123",
		Version: "1.0",

		// Default namespace fields
		Name:  "John Doe",
		Email: "john.doe@example.com",

		// ns1: Profile
		Profile: Profile{
			Visibility: "public",
			Verified:   true,
			Bio:        "Software engineer and open source enthusiast",
			AvatarURL:  "https://example.com/avatars/john.jpg",
			Tags:       []string{"developer", "golang", "xml"},
		},

		// ns1: Settings
		Settings: Settings{
			Theme:    "dark",
			Language: "en-US",
			Notification: NotificationSettings{
				Enabled:   true,
				Email:     true,
				Push:      false,
				Frequency: "daily",
			},
		},

		// ns2: Address
		Address: &Address{
			Type:    strPtr("home"),
			Primary: boolPtr(true),
			Street:  "123 Main Street",
			City:    "San Francisco",
			State:   "CA",
			ZipCode: "94102",
			Country: "USA",
		},

		// ns2: Metadata
		Metadata: Metadata{
			CreatedAt: "2024-01-15T10:30:00Z",
			UpdatedAt: "2024-01-19T14:20:00Z",
			Source:    "web-app",
			CustomFields: []CustomField{
				{Key: "department", Type: "string", Value: "Engineering"},
				{Key: "employee-id", Type: "number", Value: "12345"},
			},
		},
	}

	// Marshal to XML
	output, err := xml.MarshalIndent(user, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Print the XML
	xmlOutput := xml.Header + string(output)
	fmt.Println(xmlOutput)

	// Write to file
	err = os.WriteFile("testdata/00_generated.xml", []byte(xmlOutput), 0644)
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Unmarshal back using xmlctx to verify round-trip
	// (standard xml.Unmarshal doesn't work with namespace-aware tags)
	var unmarshaledUser User
	err = xmlctx.Unmarshal(output, &unmarshaledUser,
		xmlctx.WithNamespaces(map[string]string{
			"":    DefaultNS,
			"ns1": NS1URL,
			"ns2": NS2URL,
		}),
	)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify key fields match
	if unmarshaledUser.ID != user.ID {
		t.Errorf("ID mismatch: got %s, want %s", unmarshaledUser.ID, user.ID)
	}
	if unmarshaledUser.Name != user.Name {
		t.Errorf("Name mismatch: got %s, want %s", unmarshaledUser.Name, user.Name)
	}
	if unmarshaledUser.Email != user.Email {
		t.Errorf("Email mismatch: got %s, want %s", unmarshaledUser.Email, user.Email)
	}
	if unmarshaledUser.Profile.Bio != user.Profile.Bio {
		t.Errorf("Profile.Bio mismatch: got %s, want %s", unmarshaledUser.Profile.Bio, user.Profile.Bio)
	}
	if unmarshaledUser.Profile.Verified != user.Profile.Verified {
		t.Errorf("Profile.Verified mismatch: got %v, want %v", unmarshaledUser.Profile.Verified, user.Profile.Verified)
	}
	if len(unmarshaledUser.Profile.Tags) != len(user.Profile.Tags) {
		t.Errorf("Profile.Tags length mismatch: got %d, want %d", len(unmarshaledUser.Profile.Tags), len(user.Profile.Tags))
	}
	if unmarshaledUser.Settings.Theme != user.Settings.Theme {
		t.Errorf("Settings.Theme mismatch: got %s, want %s", unmarshaledUser.Settings.Theme, user.Settings.Theme)
	}
	if unmarshaledUser.Settings.Notification.Enabled != user.Settings.Notification.Enabled {
		t.Errorf("Notification.Enabled mismatch: got %v, want %v", unmarshaledUser.Settings.Notification.Enabled, user.Settings.Notification.Enabled)
	}
	if unmarshaledUser.Address.City != user.Address.City {
		t.Errorf("Address.City mismatch: got %s, want %s", unmarshaledUser.Address.City, user.Address.City)
	}
	if (unmarshaledUser.Address.Primary == nil) != (user.Address.Primary == nil) {
		t.Errorf("Address.Primary nil mismatch: got %v, want %v", unmarshaledUser.Address.Primary, user.Address.Primary)
	} else if unmarshaledUser.Address.Primary != nil && *unmarshaledUser.Address.Primary != *user.Address.Primary {
		t.Errorf("Address.Primary mismatch: got %v, want %v", *unmarshaledUser.Address.Primary, *user.Address.Primary)
	}
	if unmarshaledUser.Metadata.Source != user.Metadata.Source {
		t.Errorf("Metadata.Source mismatch: got %s, want %s", unmarshaledUser.Metadata.Source, user.Metadata.Source)
	}
	if len(unmarshaledUser.Metadata.CustomFields) != len(user.Metadata.CustomFields) {
		t.Errorf("CustomFields length mismatch: got %d, want %d", len(unmarshaledUser.Metadata.CustomFields), len(user.Metadata.CustomFields))
	}

	fmt.Println("\nRound-trip test passed!")
}

// verifyUser checks that a User struct has the expected values
func verifyUser(t *testing.T, u User) {
	t.Helper()

	if u.ID != "user-123" {
		t.Errorf("ID: got %s, want user-123", u.ID)
	}
	if u.Version != "1.0" {
		t.Errorf("Version: got %s, want 1.0", u.Version)
	}
	if u.Name != "John Doe" {
		t.Errorf("Name: got %s, want John Doe", u.Name)
	}
	if u.Email != "john.doe@example.com" {
		t.Errorf("Email: got %s, want john.doe@example.com", u.Email)
	}
	if u.Profile.Visibility != "public" {
		t.Errorf("Profile.Visibility: got %s, want public", u.Profile.Visibility)
	}
	if !u.Profile.Verified {
		t.Error("Profile.Verified: got false, want true")
	}
	if u.Profile.Bio != "Software engineer and open source enthusiast" {
		t.Errorf("Profile.Bio: got %s, want Software engineer and open source enthusiast", u.Profile.Bio)
	}
	if u.Profile.AvatarURL != "https://example.com/avatars/john.jpg" {
		t.Errorf("Profile.AvatarURL: got %s, want https://example.com/avatars/john.jpg", u.Profile.AvatarURL)
	}
	if len(u.Profile.Tags) != 3 {
		t.Errorf("Profile.Tags length: got %d, want 3", len(u.Profile.Tags))
	}
	if u.Settings.Theme != "dark" {
		t.Errorf("Settings.Theme: got %s, want dark", u.Settings.Theme)
	}
	if u.Settings.Language != "en-US" {
		t.Errorf("Settings.Language: got %s, want en-US", u.Settings.Language)
	}
	if !u.Settings.Notification.Enabled {
		t.Error("Notification.Enabled: got false, want true")
	}
	if !u.Settings.Notification.Email {
		t.Error("Notification.Email: got false, want true")
	}
	if u.Settings.Notification.Push {
		t.Error("Notification.Push: got true, want false")
	}
	if u.Settings.Notification.Frequency != "daily" {
		t.Errorf("Notification.Frequency: got %s, want daily", u.Settings.Notification.Frequency)
	}
	if u.Address.Type == nil || *u.Address.Type != "home" {
		t.Errorf("Address.Type: got %v, want home", u.Address.Type)
	}
	if u.Address.Primary == nil || !*u.Address.Primary {
		t.Error("Address.Primary: got false or nil, want true")
	}
	if u.Address.Street != "123 Main Street" {
		t.Errorf("Address.Street: got %s, want 123 Main Street", u.Address.Street)
	}
	if u.Address.City != "San Francisco" {
		t.Errorf("Address.City: got %s, want San Francisco", u.Address.City)
	}
	if u.Address.State != "CA" {
		t.Errorf("Address.State: got %s, want CA", u.Address.State)
	}
	if u.Address.ZipCode != "94102" {
		t.Errorf("Address.ZipCode: got %s, want 94102", u.Address.ZipCode)
	}
	if u.Address.Country != "USA" {
		t.Errorf("Address.Country: got %s, want USA", u.Address.Country)
	}
	if u.Metadata.CreatedAt != "2024-01-15T10:30:00Z" {
		t.Errorf("Metadata.CreatedAt: got %s, want 2024-01-15T10:30:00Z", u.Metadata.CreatedAt)
	}
	if u.Metadata.UpdatedAt != "2024-01-19T14:20:00Z" {
		t.Errorf("Metadata.UpdatedAt: got %s, want 2024-01-19T14:20:00Z", u.Metadata.UpdatedAt)
	}
	if u.Metadata.Source != "web-app" {
		t.Errorf("Metadata.Source: got %s, want web-app", u.Metadata.Source)
	}
	if len(u.Metadata.CustomFields) != 2 {
		t.Errorf("CustomFields length: got %d, want 2", len(u.Metadata.CustomFields))
	} else {
		if u.Metadata.CustomFields[0].Key != "department" {
			t.Errorf("CustomField[0].Key: got %s, want department", u.Metadata.CustomFields[0].Key)
		}
		if u.Metadata.CustomFields[0].Value != "Engineering" {
			t.Errorf("CustomField[0].Value: got %s, want Engineering", u.Metadata.CustomFields[0].Value)
		}
		if u.Metadata.CustomFields[1].Key != "employee-id" {
			t.Errorf("CustomField[1].Key: got %s, want employee-id", u.Metadata.CustomFields[1].Key)
		}
		if u.Metadata.CustomFields[1].Value != "12345" {
			t.Errorf("CustomField[1].Value: got %s, want 12345", u.Metadata.CustomFields[1].Value)
		}
	}
}

// TestUnmarshalAllVariations tests all valid XML variations from the testdata folder
func TestUnmarshalAllVariations(t *testing.T) {
	testFiles := []string{
		"testdata/01_explicit_prefixes.xml",
		"testdata/02_different_prefix_names.xml",
		"testdata/03_profile_as_default.xml",
		"testdata/04_address_as_default.xml",
		"testdata/05_single_char_prefixes.xml",
		"testdata/06_nested_declarations.xml",
		"testdata/07_ns1_root_ns2_nested.xml",
		"testdata/08_ns2_root_ns1_nested.xml",
		"testdata/09_default_namespace_switch.xml",
		"testdata/10_prefix_redeclaration.xml",
		"testdata/11_complex_combination.xml",
		"testdata/12_multiple_prefixes_same_namespace.xml",
		"testdata/13_complete_default_switching.xml",
		"testdata/14_no_root_declarations.xml",
		"testdata/15_namespaced_attributes.xml",
		"testdata/16_overlapping_scopes.xml",
		"testdata/17_unknown_elements_skipped.xml",
	}

	for _, file := range testFiles {
		t.Run(file, func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("Failed to read file %s: %v", file, err)
			}

			var user User
			err = xmlctx.Unmarshal(data, &user,
				xmlctx.WithNamespaces(map[string]string{
					"":    DefaultNS, // default namespace
					"ns1": NS1URL,    // profile namespace
					"ns2": NS2URL,    // address namespace
				}),
			)
			if err != nil {
				t.Fatalf("Failed to unmarshal %s: %v", file, err)
			}

			verifyUser(t, user)
			fmt.Printf("%s: PASSED\n", file)
		})
	}
}

// TestInvalidNamespaces tests XML files with various invalid namespace patterns
func TestInvalidNamespaces(t *testing.T) {
	tests := []struct {
		file        string
		description string
		checkFunc   func(t *testing.T, user User)
	}{
		{
			file:        "testdata/invalid/01_xmlns_literal_ns1.xml",
			description: "xmlns='ns1' creates literal namespace",
			checkFunc: func(t *testing.T, user User) {
				if user.Profile.Bio != "" {
					t.Errorf("Expected Profile.Bio empty, got: %s", user.Profile.Bio)
				}
				if user.Settings.Theme != "" {
					t.Errorf("Expected Settings.Theme empty, got: %s", user.Settings.Theme)
				}
			},
		},
		{
			file:        "testdata/invalid/02_wrong_namespace_uri.xml",
			description: "Wrong namespace URI for profile",
			checkFunc: func(t *testing.T, user User) {
				if user.Profile.Bio != "" {
					t.Errorf("Expected Profile.Bio empty, got: %s", user.Profile.Bio)
				}
			},
		},
		{
			file:        "testdata/invalid/03_undeclared_prefix.xml",
			description: "Using undeclared prefixes",
			checkFunc: func(t *testing.T, user User) {
				// Undeclared prefixes become literal namespaces (e.g., "prf" and "addr")
				// So these elements won't match our expected namespaces
				if user.Profile.Bio != "" {
					t.Errorf("Expected Profile.Bio empty, got: %s", user.Profile.Bio)
				}
				if user.Address != nil && user.Address.City != "" {
					t.Errorf("Expected Address.City empty, got: %s", user.Address.City)
				}
			},
		},
		{
			file:        "testdata/invalid/04_swapped_namespaces.xml",
			description: "Swapped namespace URIs",
			checkFunc: func(t *testing.T, user User) {
				if user.Profile.Bio != "" {
					t.Errorf("Expected Profile.Bio empty, got: %s", user.Profile.Bio)
				}
				if user.Address != nil && user.Address.City != "" {
					t.Errorf("Expected Address.City empty, got: %s", user.Address.City)
				}
			},
		},
		{
			file:        "testdata/invalid/05_empty_namespace.xml",
			description: "Empty namespace for profile elements",
			checkFunc: func(t *testing.T, user User) {
				if user.Profile.Bio != "" {
					t.Errorf("Expected Profile.Bio empty, got: %s", user.Profile.Bio)
				}
			},
		},
		{
			file:        "testdata/invalid/06_namespace_typo.xml",
			description: "Typo in namespace URI",
			checkFunc: func(t *testing.T, user User) {
				if user.Profile.Bio != "" {
					t.Errorf("Expected Profile.Bio empty, got: %s", user.Profile.Bio)
				}
			},
		},
		{
			file:        "testdata/invalid/07_wrong_default_namespace.xml",
			description: "Wrong default namespace",
			checkFunc: func(t *testing.T, user User) {
				if user.Name != "" {
					t.Errorf("Expected Name empty, got: %s", user.Name)
				}
				if user.Email != "" {
					t.Errorf("Expected Email empty, got: %s", user.Email)
				}
			},
		},
		{
			file:        "testdata/invalid/08_mixed_xmlns_literal.xml",
			description: "Mix of correct prefixes and xmlns literals",
			checkFunc: func(t *testing.T, user User) {
				if user.Settings.Theme != "" {
					t.Errorf("Expected Settings.Theme empty, got: %s", user.Settings.Theme)
				}
				// Profile should work (uses ns1: prefix)
				if user.Profile.Bio == "" {
					t.Errorf("Expected Profile.Bio to have value")
				}
			},
		},
		{
			file:        "testdata/invalid/09_no_namespace_declarations.xml",
			description: "No namespace declarations",
			checkFunc: func(t *testing.T, user User) {
				// Only ID and Version (attributes) should work
				if user.Name != "" {
					t.Errorf("Expected Name empty, got: %s", user.Name)
				}
				if user.Profile.Bio != "" {
					t.Errorf("Expected Profile.Bio empty, got: %s", user.Profile.Bio)
				}
			},
		},
		{
			file:        "testdata/invalid/10_http_vs_https.xml",
			description: "http vs https in namespace URIs",
			checkFunc: func(t *testing.T, user User) {
				if user.Name != "" {
					t.Errorf("Expected Name empty, got: %s", user.Name)
				}
				if user.Profile.Bio != "" {
					t.Errorf("Expected Profile.Bio empty, got: %s", user.Profile.Bio)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			data, err := os.ReadFile(tt.file)
			if err != nil {
				t.Fatalf("Failed to read file %s: %v", tt.file, err)
			}

			var user User
			err = xmlctx.Unmarshal(data, &user,
				xmlctx.WithNamespaces(map[string]string{
					"":    DefaultNS,
					"ns1": NS1URL,
					"ns2": NS2URL,
				}),
			)

			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			// Run file-specific checks
			tt.checkFunc(t, user)
			fmt.Printf("%s: Correctly handled - %s\n", tt.file, tt.description)
		})
	}
}

// TestDecodeErrors tests error handling in Decode
func TestDecodeErrors(t *testing.T) {
	t.Run("non-pointer", func(t *testing.T) {
		xmlData := []byte(`<user><name>John</name></user>`)
		var user User
		err := xmlctx.Unmarshal(xmlData, user, xmlctx.WithNamespaces(map[string]string{}))
		if err == nil {
			t.Error("Expected error for non-pointer, got nil")
		}
	})

	t.Run("nil-pointer", func(t *testing.T) {
		xmlData := []byte(`<user><name>John</name></user>`)
		var user *User
		err := xmlctx.Unmarshal(xmlData, user, xmlctx.WithNamespaces(map[string]string{}))
		if err == nil {
			t.Error("Expected error for nil pointer, got nil")
		}
	})

	t.Run("empty-xml", func(t *testing.T) {
		xmlData := []byte(``)
		var user User
		err := xmlctx.Unmarshal(xmlData, &user, xmlctx.WithNamespaces(map[string]string{}))
		// Should handle EOF gracefully
		if err != nil && err.Error() != "EOF" {
			t.Errorf("Unexpected error: %v", err)
		}
	})
}

// TestIntegerTypes tests all integer type conversions
func TestIntegerTypes(t *testing.T) {
	type AllInts struct {
		XMLName xml.Name `xml:"ints"`
		I       int      `xml:"i,attr"`
		I8      int8     `xml:"i8,attr"`
		I16     int16    `xml:"i16,attr"`
		I32     int32    `xml:"i32,attr"`
		I64     int64    `xml:"i64,attr"`
		U       uint     `xml:"u,attr"`
		U8      uint8    `xml:"u8,attr"`
		U16     uint16   `xml:"u16,attr"`
		U32     uint32   `xml:"u32,attr"`
		U64     uint64   `xml:"u64,attr"`
	}

	xmlData := []byte(`<ints i="42" i8="127" i16="32767" i32="2147483647" i64="9223372036854775807" u="42" u8="255" u16="65535" u32="4294967295" u64="18446744073709551615"></ints>`)
	var ints AllInts
	err := xmlctx.Unmarshal(xmlData, &ints, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if ints.I != 42 {
		t.Errorf("I: got %d, want 42", ints.I)
	}
	if ints.I8 != 127 {
		t.Errorf("I8: got %d, want 127", ints.I8)
	}
	if ints.I16 != 32767 {
		t.Errorf("I16: got %d, want 32767", ints.I16)
	}
	if ints.I32 != 2147483647 {
		t.Errorf("I32: got %d, want 2147483647", ints.I32)
	}
	if ints.I64 != 9223372036854775807 {
		t.Errorf("I64: got %d, want 9223372036854775807", ints.I64)
	}
	if ints.U != 42 {
		t.Errorf("U: got %d, want 42", ints.U)
	}
	if ints.U8 != 255 {
		t.Errorf("U8: got %d, want 255", ints.U8)
	}
	if ints.U16 != 65535 {
		t.Errorf("U16: got %d, want 65535", ints.U16)
	}
	if ints.U32 != 4294967295 {
		t.Errorf("U32: got %d, want 4294967295", ints.U32)
	}
	if ints.U64 != 18446744073709551615 {
		t.Errorf("U64: got %d, want 18446744073709551615", ints.U64)
	}
}

// TestIntegerPointers tests integer pointer types
func TestIntegerPointers(t *testing.T) {
	type IntPtrs struct {
		XMLName xml.Name `xml:"ints"`
		I       *int     `xml:"i,attr"`
		I8      *int8    `xml:"i8,attr"`
		I16     *int16   `xml:"i16,attr"`
		I32     *int32   `xml:"i32,attr"`
		I64     *int64   `xml:"i64,attr"`
		U       *uint    `xml:"u,attr"`
		U8      *uint8   `xml:"u8,attr"`
		U16     *uint16  `xml:"u16,attr"`
		U32     *uint32  `xml:"u32,attr"`
		U64     *uint64  `xml:"u64,attr"`
	}

	xmlData := []byte(`<ints i="10" i8="20" i16="30" i32="40" i64="50" u="60" u8="70" u16="80" u32="90" u64="100"></ints>`)
	var ints IntPtrs
	err := xmlctx.Unmarshal(xmlData, &ints, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if ints.I == nil || *ints.I != 10 {
		t.Errorf("I: got %v, want 10", ints.I)
	}
	if ints.I8 == nil || *ints.I8 != 20 {
		t.Errorf("I8: got %v, want 20", ints.I8)
	}
	if ints.I16 == nil || *ints.I16 != 30 {
		t.Errorf("I16: got %v, want 30", ints.I16)
	}
	if ints.I32 == nil || *ints.I32 != 40 {
		t.Errorf("I32: got %v, want 40", ints.I32)
	}
	if ints.I64 == nil || *ints.I64 != 50 {
		t.Errorf("I64: got %v, want 50", ints.I64)
	}
	if ints.U == nil || *ints.U != 60 {
		t.Errorf("U: got %v, want 60", ints.U)
	}
	if ints.U8 == nil || *ints.U8 != 70 {
		t.Errorf("U8: got %v, want 70", ints.U8)
	}
	if ints.U16 == nil || *ints.U16 != 80 {
		t.Errorf("U16: got %v, want 80", ints.U16)
	}
	if ints.U32 == nil || *ints.U32 != 90 {
		t.Errorf("U32: got %v, want 90", ints.U32)
	}
	if ints.U64 == nil || *ints.U64 != 100 {
		t.Errorf("U64: got %v, want 100", ints.U64)
	}
}

// TestInvalidIntegerConversions tests error handling for invalid integer values
func TestInvalidIntegerConversions(t *testing.T) {
	type IntTest struct {
		XMLName xml.Name `xml:"test"`
		Value   int      `xml:"value,attr"`
	}

	xmlData := []byte(`<test value="not-a-number"></test>`)
	var test IntTest
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	if err == nil {
		t.Error("Expected error for invalid integer, got nil")
	}
}

// TestInvalidUintConversions tests error handling for invalid uint values
func TestInvalidUintConversions(t *testing.T) {
	type UintTest struct {
		XMLName xml.Name `xml:"test"`
		Value   uint     `xml:"value,attr"`
	}

	xmlData := []byte(`<test value="not-a-number"></test>`)
	var test UintTest
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	if err == nil {
		t.Error("Expected error for invalid uint, got nil")
	}
}

// TestUnsupportedTypes tests error handling for unsupported field types
func TestUnsupportedTypes(t *testing.T) {
	type Unsupported struct {
		XMLName xml.Name `xml:"test"`
		Value   float64  `xml:"value,attr"`
	}

	xmlData := []byte(`<test value="3.14"></test>`)
	var test Unsupported
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	if err == nil {
		t.Error("Expected error for unsupported type (float64), got nil")
	}
}

// TestNamespacedAttributes tests attributes with namespace prefixes
func TestNamespacedAttributes(t *testing.T) {
	type WithNSAttrs struct {
		XMLName    xml.Name `xml:"test"`
		NormalAttr string   `xml:"normal,attr"`
		NSAttr     string   `xml:"ns1:special,attr"`
	}

	xmlData := []byte(`<test xmlns:ns1="http://example.com/ns1" normal="value1" ns1:special="value2"></test>`)
	var test WithNSAttrs
	err := xmlctx.Unmarshal(xmlData, &test,
		xmlctx.WithNamespaces(map[string]string{
			"ns1": "http://example.com/ns1",
		}),
	)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if test.NormalAttr != "value1" {
		t.Errorf("NormalAttr: got %s, want value1", test.NormalAttr)
	}
	if test.NSAttr != "value2" {
		t.Errorf("NSAttr: got %s, want value2", test.NSAttr)
	}
}

// TestBooleanValues tests boolean field decoding
func TestBooleanValues(t *testing.T) {
	type BoolTest struct {
		XMLName xml.Name `xml:"test"`
		True    bool     `xml:"true"`
		False   bool     `xml:"false"`
	}

	xmlData := []byte(`<test><true>true</true><false>false</false></test>`)
	var test BoolTest
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if !test.True {
		t.Error("True field should be true")
	}
	if test.False {
		t.Error("False field should be false")
	}
}

// TestUnsupportedElementType tests decoding into unsupported element types
func TestUnsupportedElementType(t *testing.T) {
	type UnsupportedElem struct {
		XMLName xml.Name `xml:"test"`
		Value   float64  `xml:"value"`
	}

	xmlData := []byte(`<test><value>3.14</value></test>`)
	var test UnsupportedElem
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	if err == nil {
		t.Error("Expected error for unsupported element type (float64), got nil")
	}
}

// TestSliceOfUnsupportedTypes tests error propagation in slice decoding
func TestSliceOfUnsupportedTypes(t *testing.T) {
	type SliceTest struct {
		XMLName xml.Name  `xml:"test"`
		Values  []float64 `xml:"value"`
	}

	xmlData := []byte(`<test><value>1.1</value></test>`)
	var test SliceTest
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	if err == nil {
		t.Error("Expected error for slice of unsupported type, got nil")
	}
}

// TestStringPointer tests string pointer decoding
func TestStringPointer(t *testing.T) {
	type StrPtrTest struct {
		XMLName xml.Name `xml:"test"`
		Value   *string  `xml:"value"`
	}

	xmlData := []byte(`<test><value>hello</value></test>`)
	var test StrPtrTest
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if test.Value == nil || *test.Value != "hello" {
		t.Errorf("Value: got %v, want hello", test.Value)
	}
}

// TestBoolPointer tests bool pointer decoding
func TestBoolPointer(t *testing.T) {
	type BoolPtrTest struct {
		XMLName xml.Name `xml:"test"`
		Value   *bool    `xml:"value"`
	}

	xmlData := []byte(`<test><value>true</value></test>`)
	var test BoolPtrTest
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if test.Value == nil || !*test.Value {
		t.Errorf("Value: got %v, want true", test.Value)
	}
}

// TestNamespacedAttributeNotFound tests namespaced attributes that don't match
func TestNamespacedAttributeNotFound(t *testing.T) {
	type NSAttrTest struct {
		XMLName xml.Name `xml:"test"`
		Attr    string   `xml:"ns1:special,attr"`
	}

	// Attribute uses wrong namespace
	xmlData := []byte(`<test xmlns:ns1="http://example.com/wrong" ns1:special="value"></test>`)
	var test NSAttrTest
	err := xmlctx.Unmarshal(xmlData, &test,
		xmlctx.WithNamespaces(map[string]string{
			"ns1": "http://example.com/right",
		}),
	)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Attribute shouldn't match due to wrong namespace
	if test.Attr != "" {
		t.Errorf("Attr should be empty, got %s", test.Attr)
	}
}

// TestAttributeWithUnknownPrefix tests attributes with prefixes not in namespace map
func TestAttributeWithUnknownPrefix(t *testing.T) {
	type UnknownPrefixTest struct {
		XMLName xml.Name `xml:"test"`
		Attr    string   `xml:"unknown:attr,attr"`
	}

	xmlData := []byte(`<test xmlns:foo="http://example.com/foo" foo:attr="value"></test>`)
	var test UnknownPrefixTest
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Attribute shouldn't match due to unknown prefix in tag
	if test.Attr != "" {
		t.Errorf("Attr should be empty, got %s", test.Attr)
	}
}

// TestMatchesFieldWithoutNamespace tests matching fields when no default namespace is set
func TestMatchesFieldWithoutNamespace(t *testing.T) {
	type NoNSTest struct {
		XMLName xml.Name `xml:"test"`
		Value   string   `xml:"value"`
	}

	xmlData := []byte(`<test><value>hello</value></test>`)
	var test NoNSTest
	// No default namespace in the map
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if test.Value != "hello" {
		t.Errorf("Value: got %s, want hello", test.Value)
	}
}

// TestSkipUnknownElements tests that unknown elements are skipped (continue case)
func TestSkipUnknownElements(t *testing.T) {
	type SimpleStruct struct {
		XMLName xml.Name `xml:"root"`
		Name    string   `xml:"name"`
		Email   string   `xml:"email"`
	}

	// XML with unknown elements interspersed
	xmlData := []byte(`<root>
		<unknown1>This should be skipped</unknown1>
		<name>John Doe</name>
		<unknown2>Also skipped</unknown2>
		<email>john@example.com</email>
		<unknown3>Skipped too</unknown3>
	</root>`)

	var s SimpleStruct
	err := xmlctx.Unmarshal(xmlData, &s, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if s.Name != "John Doe" {
		t.Errorf("Name: got %s, want John Doe", s.Name)
	}
	if s.Email != "john@example.com" {
		t.Errorf("Email: got %s, want john@example.com", s.Email)
	}
}

// TestSkipNestedUnknownElements tests that nested unknown elements are properly skipped
func TestSkipNestedUnknownElements(t *testing.T) {
	type SimpleStruct struct {
		XMLName xml.Name `xml:"root"`
		Name    string   `xml:"name"`
		Value   string   `xml:"value"`
	}

	// XML with nested unknown elements
	xmlData := []byte(`<root>
		<name>Test</name>
		<unknown>
			<nested>
				<deeply>
					<nested>content</nested>
				</deeply>
			</nested>
		</unknown>
		<value>Result</value>
	</root>`)

	var s SimpleStruct
	err := xmlctx.Unmarshal(xmlData, &s, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if s.Name != "Test" {
		t.Errorf("Name: got %s, want Test", s.Name)
	}
	if s.Value != "Result" {
		t.Errorf("Value: got %s, want Result", s.Value)
	}
}

// TestSkipUnknownNamespacedElements tests skipping elements in unknown namespaces
func TestSkipUnknownNamespacedElements(t *testing.T) {
	type SimpleStruct struct {
		XMLName xml.Name `xml:"root"`
		Name    string   `xml:"ns1:name"`
		Email   string   `xml:"ns1:email"`
	}

	// XML with elements in unknown namespaces
	xmlData := []byte(`<root xmlns:ns1="http://example.com/ns1" xmlns:ns2="http://example.com/ns2">
		<ns2:unknown>Should be skipped</ns2:unknown>
		<ns1:name>John</ns1:name>
		<ns2:other>Also skipped</ns2:other>
		<ns1:email>john@example.com</ns1:email>
		<ns3:bad>Undeclared namespace</ns3:bad>
	</root>`)

	var s SimpleStruct
	err := xmlctx.Unmarshal(xmlData, &s,
		xmlctx.WithNamespaces(map[string]string{
			"ns1": "http://example.com/ns1",
		}),
	)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if s.Name != "John" {
		t.Errorf("Name: got %s, want John", s.Name)
	}
	if s.Email != "john@example.com" {
		t.Errorf("Email: got %s, want john@example.com", s.Email)
	}
}

// TestSkipUnknownElementsInComplexStruct tests continue case with the main User struct
func TestSkipUnknownElementsInComplexStruct(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<user xmlns="http://example.com/schema/user"
      xmlns:ns1="http://example.com/schema/profile"
      xmlns:ns2="http://example.com/schema/address"
      id="user-123" version="1.0">
  <unknown-field>Should be ignored</unknown-field>
  <name>John Doe</name>
  <extra>Also ignored</extra>
  <email>john.doe@example.com</email>
  <ns1:profile visibility="public" verified="true">
    <ns1:unknown>Ignored in profile</ns1:unknown>
    <ns1:bio>Software engineer</ns1:bio>
    <ns1:avatar-url>https://example.com/avatars/john.jpg</ns1:avatar-url>
  </ns1:profile>
  <random-element>
    <with>
      <nested>data</nested>
    </with>
  </random-element>
  <ns2:address type="home" primary="true">
    <ns2:street>123 Main Street</ns2:street>
    <ns2:unknown-field>Skipped</ns2:unknown-field>
    <ns2:city>San Francisco</ns2:city>
  </ns2:address>
</user>`)

	var user User
	err := xmlctx.Unmarshal(xmlData, &user,
		xmlctx.WithNamespaces(map[string]string{
			"":    DefaultNS,
			"ns1": NS1URL,
			"ns2": NS2URL,
		}),
	)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify known fields were parsed correctly
	if user.ID != "user-123" {
		t.Errorf("ID: got %s, want user-123", user.ID)
	}
	if user.Name != "John Doe" {
		t.Errorf("Name: got %s, want John Doe", user.Name)
	}
	if user.Email != "john.doe@example.com" {
		t.Errorf("Email: got %s, want john.doe@example.com", user.Email)
	}
	if user.Profile.Bio != "Software engineer" {
		t.Errorf("Profile.Bio: got %s, want Software engineer", user.Profile.Bio)
	}
	if user.Address.City != "San Francisco" {
		t.Errorf("Address.City: got %s, want San Francisco", user.Address.City)
	}
}

// TestSkipUnknownElementsWithAttributes tests that unknown elements with attributes are skipped
func TestSkipUnknownElementsWithAttributes(t *testing.T) {
	type SimpleStruct struct {
		XMLName xml.Name `xml:"root"`
		Value   string   `xml:"value"`
	}

	xmlData := []byte(`<root>
		<unknown attr1="val1" attr2="val2">
			<nested>Complex content</nested>
		</unknown>
		<value>Result</value>
	</root>`)

	var s SimpleStruct
	err := xmlctx.Unmarshal(xmlData, &s, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if s.Value != "Result" {
		t.Errorf("Value: got %s, want Result", s.Value)
	}
}

// TestMixedKnownUnknownElements tests XML with known and unknown elements mixed
func TestMixedKnownUnknownElements(t *testing.T) {
	type Product struct {
		XMLName xml.Name `xml:"product"`
		Name    string   `xml:"name"`
		Price   string   `xml:"price"`
		Stock   int      `xml:"stock"`
	}

	xmlData := []byte(`<product>
		<internal-id>12345</internal-id>
		<name>Widget</name>
		<supplier>ACME Corp</supplier>
		<price>19.99</price>
		<warehouse-location>A-5</warehouse-location>
		<stock>100</stock>
		<last-updated>2024-01-15</last-updated>
	</product>`)

	var p Product
	err := xmlctx.Unmarshal(xmlData, &p, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if p.Name != "Widget" {
		t.Errorf("Name: got %s, want Widget", p.Name)
	}
	if p.Price != "19.99" {
		t.Errorf("Price: got %s, want 19.99", p.Price)
	}
	if p.Stock != 100 {
		t.Errorf("Stock: got %d, want 100", p.Stock)
	}
}

// TestIntegerElements tests integer types in element content (not attributes)
func TestIntegerElements(t *testing.T) {
	type Numbers struct {
		XMLName xml.Name `xml:"numbers"`
		I       int      `xml:"i"`
		I8      int8     `xml:"i8"`
		I16     int16    `xml:"i16"`
		I32     int32    `xml:"i32"`
		I64     int64    `xml:"i64"`
		U       uint     `xml:"u"`
		U8      uint8    `xml:"u8"`
		U16     uint16   `xml:"u16"`
		U32     uint32   `xml:"u32"`
		U64     uint64   `xml:"u64"`
	}

	xmlData := []byte(`<numbers>
		<i>42</i>
		<i8>127</i8>
		<i16>32767</i16>
		<i32>2147483647</i32>
		<i64>9223372036854775807</i64>
		<u>42</u>
		<u8>255</u8>
		<u16>65535</u16>
		<u32>4294967295</u32>
		<u64>18446744073709551615</u64>
	</numbers>`)

	var nums Numbers
	err := xmlctx.Unmarshal(xmlData, &nums, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if nums.I != 42 {
		t.Errorf("I: got %d, want 42", nums.I)
	}
	if nums.I8 != 127 {
		t.Errorf("I8: got %d, want 127", nums.I8)
	}
	if nums.I16 != 32767 {
		t.Errorf("I16: got %d, want 32767", nums.I16)
	}
	if nums.I32 != 2147483647 {
		t.Errorf("I32: got %d, want 2147483647", nums.I32)
	}
	if nums.I64 != 9223372036854775807 {
		t.Errorf("I64: got %d, want 9223372036854775807", nums.I64)
	}
	if nums.U != 42 {
		t.Errorf("U: got %d, want 42", nums.U)
	}
	if nums.U8 != 255 {
		t.Errorf("U8: got %d, want 255", nums.U8)
	}
	if nums.U16 != 65535 {
		t.Errorf("U16: got %d, want 65535", nums.U16)
	}
	if nums.U32 != 4294967295 {
		t.Errorf("U32: got %d, want 4294967295", nums.U32)
	}
	if nums.U64 != 18446744073709551615 {
		t.Errorf("U64: got %d, want 18446744073709551615", nums.U64)
	}
}

// TestIntegerElementPointers tests integer pointer types in element content
func TestIntegerElementPointers(t *testing.T) {
	type Numbers struct {
		XMLName xml.Name `xml:"numbers"`
		I       *int     `xml:"i"`
		I8      *int8    `xml:"i8"`
		I16     *int16   `xml:"i16"`
		I32     *int32   `xml:"i32"`
		I64     *int64   `xml:"i64"`
		U       *uint    `xml:"u"`
		U8      *uint8   `xml:"u8"`
		U16     *uint16  `xml:"u16"`
		U32     *uint32  `xml:"u32"`
		U64     *uint64  `xml:"u64"`
	}

	xmlData := []byte(`<numbers>
		<i>100</i>
		<i8>10</i8>
		<i16>1000</i16>
		<i32>100000</i32>
		<i64>10000000</i64>
		<u>200</u>
		<u8>20</u8>
		<u16>2000</u16>
		<u32>200000</u32>
		<u64>20000000</u64>
	</numbers>`)

	var nums Numbers
	err := xmlctx.Unmarshal(xmlData, &nums, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if nums.I == nil || *nums.I != 100 {
		t.Errorf("I: got %v, want 100", nums.I)
	}
	if nums.I8 == nil || *nums.I8 != 10 {
		t.Errorf("I8: got %v, want 10", nums.I8)
	}
	if nums.I16 == nil || *nums.I16 != 1000 {
		t.Errorf("I16: got %v, want 1000", nums.I16)
	}
	if nums.I32 == nil || *nums.I32 != 100000 {
		t.Errorf("I32: got %v, want 100000", nums.I32)
	}
	if nums.I64 == nil || *nums.I64 != 10000000 {
		t.Errorf("I64: got %v, want 10000000", nums.I64)
	}
	if nums.U == nil || *nums.U != 200 {
		t.Errorf("U: got %v, want 200", nums.U)
	}
	if nums.U8 == nil || *nums.U8 != 20 {
		t.Errorf("U8: got %v, want 20", nums.U8)
	}
	if nums.U16 == nil || *nums.U16 != 2000 {
		t.Errorf("U16: got %v, want 2000", nums.U16)
	}
	if nums.U32 == nil || *nums.U32 != 200000 {
		t.Errorf("U32: got %v, want 200000", nums.U32)
	}
	if nums.U64 == nil || *nums.U64 != 20000000 {
		t.Errorf("U64: got %v, want 20000000", nums.U64)
	}
}

// TestInvalidIntegerElement tests error handling for invalid integer values in elements
func TestInvalidIntegerElement(t *testing.T) {
	type IntTest struct {
		XMLName xml.Name `xml:"test"`
		Value   int      `xml:"value"`
	}

	xmlData := []byte(`<test><value>not-a-number</value></test>`)
	var test IntTest
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	if err == nil {
		t.Error("Expected error for invalid integer element, got nil")
	}
}

// TestInvalidUintElement tests error handling for invalid uint values in elements
func TestInvalidUintElement(t *testing.T) {
	type UintTest struct {
		XMLName xml.Name `xml:"test"`
		Value   uint     `xml:"value"`
	}

	xmlData := []byte(`<test><value>not-a-number</value></test>`)
	var test UintTest
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	if err == nil {
		t.Error("Expected error for invalid uint element, got nil")
	}
}

// TestNegativeUintElement tests error handling for negative values in uint elements
func TestNegativeUintElement(t *testing.T) {
	type UintTest struct {
		XMLName xml.Name `xml:"test"`
		Value   uint     `xml:"value"`
	}

	xmlData := []byte(`<test><value>-100</value></test>`)
	var test UintTest
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	if err == nil {
		t.Error("Expected error for negative uint element, got nil")
	}
}

// TestEmptyStringElement tests decoding empty string elements
func TestEmptyStringElement(t *testing.T) {
	type EmptyTest struct {
		XMLName xml.Name `xml:"test"`
		Value   string   `xml:"value"`
		Empty   string   `xml:"empty"`
	}

	xmlData := []byte(`<test><value>has content</value><empty></empty></test>`)
	var test EmptyTest
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if test.Value != "has content" {
		t.Errorf("Value: got %s, want 'has content'", test.Value)
	}
	if test.Empty != "" {
		t.Errorf("Empty: got %s, want empty string", test.Empty)
	}
}

// TestEmptyIntegerElement tests decoding empty integer elements (should error)
func TestEmptyIntegerElement(t *testing.T) {
	type EmptyIntTest struct {
		XMLName xml.Name `xml:"test"`
		Value   int      `xml:"value"`
	}

	xmlData := []byte(`<test><value></value></test>`)
	var test EmptyIntTest
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	if err == nil {
		t.Error("Expected error for empty integer element, got nil")
	}
}

// TestEmptyBoolElement tests decoding empty bool elements
func TestEmptyBoolElement(t *testing.T) {
	type EmptyBoolTest struct {
		XMLName xml.Name `xml:"test"`
		Value   bool     `xml:"value"`
	}

	xmlData := []byte(`<test><value></value></test>`)
	var test EmptyBoolTest
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Empty bool should be false
	if test.Value {
		t.Error("Empty bool should be false")
	}
}

// TestWhitespaceOnlyElements tests elements with only whitespace
func TestWhitespaceOnlyElements(t *testing.T) {
	type WhitespaceTest struct {
		XMLName xml.Name `xml:"test"`
		Str     string   `xml:"str"`
		Num     int      `xml:"num"`
	}

	xmlData := []byte(`<test><str>   </str><num>  42  </num></test>`)
	var test WhitespaceTest
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Whitespace should be trimmed
	if test.Str != "" {
		t.Errorf("Str: got '%s', want empty string", test.Str)
	}
	if test.Num != 42 {
		t.Errorf("Num: got %d, want 42", test.Num)
	}
}

// TestIntegerOverflow tests integer overflow detection
func TestIntegerOverflow(t *testing.T) {
	type IntOverflowTest struct {
		XMLName xml.Name `xml:"test"`
		Value   int8     `xml:"value"`
	}

	xmlData := []byte(`<test><value>999999</value></test>`)
	var test IntOverflowTest
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	// Should succeed but value will overflow - Go's SetInt will handle the overflow
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
}

// TestSliceOfIntegers tests decoding slices of integers
func TestSliceOfIntegers(t *testing.T) {
	type IntSliceTest struct {
		XMLName xml.Name `xml:"test"`
		Values  []int    `xml:"value"`
	}

	xmlData := []byte(`<test><value>1</value><value>2</value><value>3</value></test>`)
	var test IntSliceTest
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if len(test.Values) != 3 {
		t.Errorf("Values length: got %d, want 3", len(test.Values))
	}
	if test.Values[0] != 1 || test.Values[1] != 2 || test.Values[2] != 3 {
		t.Errorf("Values: got %v, want [1 2 3]", test.Values)
	}
}

// TestSliceOfBools tests decoding slices of bools
func TestSliceOfBools(t *testing.T) {
	type BoolSliceTest struct {
		XMLName xml.Name `xml:"test"`
		Flags   []bool   `xml:"flag"`
	}

	xmlData := []byte(`<test><flag>true</flag><flag>false</flag><flag>true</flag></test>`)
	var test BoolSliceTest
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if len(test.Flags) != 3 {
		t.Errorf("Flags length: got %d, want 3", len(test.Flags))
	}
	if !test.Flags[0] || test.Flags[1] || !test.Flags[2] {
		t.Errorf("Flags: got %v, want [true false true]", test.Flags)
	}
}

// TestNestedStructPointers tests nested struct with pointers
func TestNestedStructPointers(t *testing.T) {
	type Inner struct {
		Value string `xml:"value"`
	}
	type Outer struct {
		XMLName xml.Name `xml:"outer"`
		Inner   *Inner   `xml:"inner"`
	}

	xmlData := []byte(`<outer><inner><value>test</value></inner></outer>`)
	var outer Outer
	err := xmlctx.Unmarshal(xmlData, &outer, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if outer.Inner == nil {
		t.Fatal("Inner should not be nil")
	}
	if outer.Inner.Value != "test" {
		t.Errorf("Inner.Value: got %s, want test", outer.Inner.Value)
	}
}

// TestCharDataWithMixedContent tests chardata with mixed content
func TestCharDataWithMixedContent(t *testing.T) {
	type MixedContent struct {
		XMLName xml.Name `xml:"para"`
		Text    string   `xml:",chardata"`
		Bold    string   `xml:"bold"`
	}

	xmlData := []byte(`<para>This is <bold>bold</bold> text</para>`)
	var para MixedContent
	err := xmlctx.Unmarshal(xmlData, &para, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if para.Bold != "bold" {
		t.Errorf("Bold: got %s, want bold", para.Bold)
	}
	// Chardata should accumulate text outside of child elements
	if para.Text == "" {
		t.Error("Text should not be empty")
	}
}

// TestStructWithoutCharDataField tests struct without chardata field but with text content
func TestStructWithoutCharDataField(t *testing.T) {
	type NoCharData struct {
		XMLName xml.Name `xml:"test"`
		Value   string   `xml:"value"`
	}

	// XML has text content but struct has no chardata field
	xmlData := []byte(`<test>Some text<value>data</value>More text</test>`)
	var test NoCharData
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if test.Value != "data" {
		t.Errorf("Value: got %s, want data", test.Value)
	}
}

// TestCharDataFieldEmpty tests chardata field with empty content
func TestCharDataFieldEmpty(t *testing.T) {
	type EmptyCharData struct {
		XMLName xml.Name `xml:"test"`
		Text    string   `xml:",chardata"`
		Value   string   `xml:"value"`
	}

	xmlData := []byte(`<test><value>data</value></test>`)
	var test EmptyCharData
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if test.Value != "data" {
		t.Errorf("Value: got %s, want data", test.Value)
	}
	if test.Text != "" {
		t.Errorf("Text: got %s, want empty string", test.Text)
	}
}

// TestStructWithOnlyWhitespaceCharData tests chardata with only whitespace
func TestStructWithOnlyWhitespaceCharData(t *testing.T) {
	type WhitespaceCharData struct {
		XMLName xml.Name `xml:"test"`
		Text    string   `xml:",chardata"`
		Value   string   `xml:"value"`
	}

	xmlData := []byte(`<test>
		<value>data</value>
	</test>`)
	var test WhitespaceCharData
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if test.Value != "data" {
		t.Errorf("Value: got %s, want data", test.Value)
	}
	// Whitespace-only chardata should be empty after trimming
	if test.Text != "" {
		t.Errorf("Text: got '%s', want empty string", test.Text)
	}
}

// TestMultipleCharDataFieldCandidates tests struct with multiple chardata tag possibilities
func TestMultipleCharDataFieldCandidates(t *testing.T) {
	type MultiCharData struct {
		XMLName xml.Name `xml:"test"`
		Text1   string   `xml:",chardata"`
		Value   string   `xml:"value"`
	}

	xmlData := []byte(`<test>some text<value>data</value></test>`)
	var test MultiCharData
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if test.Value != "data" {
		t.Errorf("Value: got %s, want data", test.Value)
	}
	// First chardata field should get the text
	if test.Text1 != "some text" {
		t.Errorf("Text1: got '%s', want 'some text'", test.Text1)
	}
}

// TestBoolVariations tests different bool value representations
func TestBoolVariations(t *testing.T) {
	type BoolTest struct {
		XMLName xml.Name `xml:"test"`
		V1      bool     `xml:"v1"`
		V2      bool     `xml:"v2"`
		V3      bool     `xml:"v3"`
		V4      bool     `xml:"v4"`
	}

	xmlData := []byte(`<test><v1>true</v1><v2>false</v2><v3>1</v3><v4>anything</v4></test>`)
	var test BoolTest
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if !test.V1 {
		t.Error("V1 should be true")
	}
	if test.V2 {
		t.Error("V2 should be false")
	}
	if test.V3 {
		t.Error("V3 (value='1') should be false (only 'true' string is true)")
	}
	if test.V4 {
		t.Error("V4 (value='anything') should be false")
	}
}

// TestSliceOfStructs tests slices containing structs
func TestSliceOfStructs(t *testing.T) {
	type Item struct {
		Name  string `xml:"name"`
		Value int    `xml:"value"`
	}
	type Container struct {
		XMLName xml.Name `xml:"container"`
		Items   []Item   `xml:"item"`
	}

	xmlData := []byte(`<container>
		<item><name>first</name><value>1</value></item>
		<item><name>second</name><value>2</value></item>
	</container>`)
	var c Container
	err := xmlctx.Unmarshal(xmlData, &c, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if len(c.Items) != 2 {
		t.Fatalf("Items length: got %d, want 2", len(c.Items))
	}
	if c.Items[0].Name != "first" || c.Items[0].Value != 1 {
		t.Errorf("Items[0]: got {%s, %d}, want {first, 1}", c.Items[0].Name, c.Items[0].Value)
	}
	if c.Items[1].Name != "second" || c.Items[1].Value != 2 {
		t.Errorf("Items[1]: got {%s, %d}, want {second, 2}", c.Items[1].Name, c.Items[1].Value)
	}
}

// TestNamespacedIntegerElements tests integer elements with namespaces
func TestNamespacedIntegerElements(t *testing.T) {
	type NSNumbers struct {
		XMLName xml.Name `xml:"root"`
		Count   int      `xml:"ns1:count"`
		Total   uint     `xml:"ns1:total"`
	}

	xmlData := []byte(`<root xmlns:ns1="http://example.com/ns1">
		<ns1:count>42</ns1:count>
		<ns1:total>100</ns1:total>
	</root>`)
	var nums NSNumbers
	err := xmlctx.Unmarshal(xmlData, &nums,
		xmlctx.WithNamespaces(map[string]string{
			"ns1": "http://example.com/ns1",
		}),
	)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if nums.Count != 42 {
		t.Errorf("Count: got %d, want 42", nums.Count)
	}
	if nums.Total != 100 {
		t.Errorf("Total: got %d, want 100", nums.Total)
	}
}

// TestStructWithUntaggedFields tests struct with fields without xml tags
func TestStructWithUntaggedFields(t *testing.T) {
	type MixedStruct struct {
		XMLName    xml.Name `xml:"test"`
		Tagged     string   `xml:"tagged"`
		Untagged   string   // No xml tag
		AlsoTagged string   `xml:"also"`
	}

	xmlData := []byte(`<test><tagged>value1</tagged><also>value2</also></test>`)
	var test MixedStruct
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if test.Tagged != "value1" {
		t.Errorf("Tagged: got %s, want value1", test.Tagged)
	}
	if test.AlsoTagged != "value2" {
		t.Errorf("AlsoTagged: got %s, want value2", test.AlsoTagged)
	}
	// Untagged should remain empty
	if test.Untagged != "" {
		t.Errorf("Untagged: got %s, want empty string", test.Untagged)
	}
}

// TestStructWithDashTag tests struct with fields tagged with "-"
func TestStructWithDashTag(t *testing.T) {
	type IgnoredFieldStruct struct {
		XMLName xml.Name `xml:"test"`
		Include string   `xml:"include"`
		Ignore  string   `xml:"-"`
	}

	xmlData := []byte(`<test><include>yes</include><ignore>no</ignore></test>`)
	var test IgnoredFieldStruct
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if test.Include != "yes" {
		t.Errorf("Include: got %s, want yes", test.Include)
	}
	// Field tagged with "-" should be ignored
	if test.Ignore != "" {
		t.Errorf("Ignore: got %s, want empty string", test.Ignore)
	}
}

// TestComplexNestedNamespaces tests deeply nested structures with multiple namespaces
func TestComplexNestedNamespaces(t *testing.T) {
	type Level3 struct {
		Value string `xml:"ns3:value"`
	}
	type Level2 struct {
		Item   string `xml:"ns2:item"`
		Level3 Level3 `xml:"ns3:deep"`
	}
	type Root struct {
		XMLName xml.Name `xml:"root"`
		Name    string   `xml:"ns1:name"`
		Level2  Level2   `xml:"ns2:mid"`
	}

	xmlData := []byte(`<root xmlns:ns1="http://ex.com/ns1" xmlns:ns2="http://ex.com/ns2" xmlns:ns3="http://ex.com/ns3">
		<ns1:name>test</ns1:name>
		<ns2:mid>
			<ns2:item>data</ns2:item>
			<ns3:deep>
				<ns3:value>nested</ns3:value>
			</ns3:deep>
		</ns2:mid>
	</root>`)

	var root Root
	err := xmlctx.Unmarshal(xmlData, &root,
		xmlctx.WithNamespaces(map[string]string{
			"ns1": "http://ex.com/ns1",
			"ns2": "http://ex.com/ns2",
			"ns3": "http://ex.com/ns3",
		}),
	)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if root.Name != "test" {
		t.Errorf("Name: got %s, want test", root.Name)
	}
	if root.Level2.Item != "data" {
		t.Errorf("Item: got %s, want data", root.Level2.Item)
	}
	if root.Level2.Level3.Value != "nested" {
		t.Errorf("Value: got %s, want nested", root.Level2.Level3.Value)
	}
}

// TestEdgeCaseTagFormats tests edge case tag formats that should be skipped
func TestEdgeCaseTagFormats(t *testing.T) {
	type EdgeCaseStruct struct {
		XMLName xml.Name `xml:"test"`
		// These unusual tag formats should be skipped when finding element fields
		AttrField   string `xml:"attrField"` // Contains "attr" but not as a flag
		XmlnsField  string `xml:"xmlnsField"` // Starts with "xmlns"
		NormalField string `xml:"normal"`
	}

	// Only "normal" should match as an element field
	xmlData := []byte(`<test>
		<normal>value</normal>
		<attrField>should not match</attrField>
		<xmlnsField>should not match</xmlnsField>
	</test>`)

	var test EdgeCaseStruct
	err := xmlctx.Unmarshal(xmlData, &test, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if test.NormalField != "value" {
		t.Errorf("NormalField: got %s, want value", test.NormalField)
	}
	// Fields with "attr" or starting with "xmlns" in the tag name should be skipped
	if test.AttrField != "" {
		t.Errorf("AttrField should be empty, got %s", test.AttrField)
	}
	if test.XmlnsField != "" {
		t.Errorf("XmlnsField should be empty, got %s", test.XmlnsField)
	}
}

// TestPathSyntaxBasic tests basic > path syntax
func TestPathSyntaxBasic(t *testing.T) {
	type Address struct {
		XMLName xml.Name `xml:"address"`
		Country string   `xml:"location>country"`
		City    string   `xml:"location>city"`
	}

	xmlData := []byte(`<address>
		<location>
			<country>USA</country>
			<city>San Francisco</city>
		</location>
	</address>`)

	var addr Address
	err := xmlctx.Unmarshal(xmlData, &addr, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if addr.Country != "USA" {
		t.Errorf("Country: got %s, want USA", addr.Country)
	}
	if addr.City != "San Francisco" {
		t.Errorf("City: got %s, want San Francisco", addr.City)
	}
}

// TestPathSyntaxWithNamespaces tests > path syntax with namespaces
func TestPathSyntaxWithNamespaces(t *testing.T) {
	type TradeInfo struct {
		XMLName xml.Name `xml:"trade"`
		Origin  string   `xml:"ram:OriginTradeCountry>ram:ID"`
	}

	xmlData := []byte(`<trade xmlns:ram="http://example.com/ram">
		<ram:OriginTradeCountry>
			<ram:ID>US</ram:ID>
		</ram:OriginTradeCountry>
	</trade>`)

	var trade TradeInfo
	err := xmlctx.Unmarshal(xmlData, &trade,
		xmlctx.WithNamespaces(map[string]string{
			"ram": "http://example.com/ram",
		}),
	)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if trade.Origin != "US" {
		t.Errorf("Origin: got %s, want US", trade.Origin)
	}
}

// TestPathSyntaxWithPointers tests > path syntax with pointer fields
func TestPathSyntaxWithPointers(t *testing.T) {
	type TradeInfo struct {
		XMLName xml.Name `xml:"trade"`
		Origin  *string  `xml:"ram:OriginTradeCountry>ram:ID,omitempty"`
	}

	xmlData := []byte(`<trade xmlns:ram="http://example.com/ram">
		<ram:OriginTradeCountry>
			<ram:ID>US</ram:ID>
		</ram:OriginTradeCountry>
	</trade>`)

	var trade TradeInfo
	err := xmlctx.Unmarshal(xmlData, &trade,
		xmlctx.WithNamespaces(map[string]string{
			"ram": "http://example.com/ram",
		}),
	)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if trade.Origin == nil {
		t.Fatal("Origin should not be nil")
	}
	if *trade.Origin != "US" {
		t.Errorf("Origin: got %s, want US", *trade.Origin)
	}
}

// TestPathSyntaxDeepNesting tests > path syntax with multiple levels
func TestPathSyntaxDeepNesting(t *testing.T) {
	type DeepStruct struct {
		XMLName xml.Name `xml:"root"`
		Value   string   `xml:"level1>level2>level3>data"`
	}

	xmlData := []byte(`<root>
		<level1>
			<level2>
				<level3>
					<data>deep value</data>
				</level3>
			</level2>
		</level1>
	</root>`)

	var deep DeepStruct
	err := xmlctx.Unmarshal(xmlData, &deep, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if deep.Value != "deep value" {
		t.Errorf("Value: got %s, want 'deep value'", deep.Value)
	}
}

// TestPathSyntaxWithSiblings tests > path syntax with sibling elements
func TestPathSyntaxWithSiblings(t *testing.T) {
	type Product struct {
		XMLName xml.Name `xml:"product"`
		Name    string   `xml:"name"`
		ID      string   `xml:"details>identifier>id"`
		Code    string   `xml:"details>identifier>code"`
	}

	xmlData := []byte(`<product>
		<name>Widget</name>
		<details>
			<identifier>
				<id>12345</id>
				<code>WDG-001</code>
			</identifier>
		</details>
	</product>`)

	var prod Product
	err := xmlctx.Unmarshal(xmlData, &prod, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if prod.Name != "Widget" {
		t.Errorf("Name: got %s, want Widget", prod.Name)
	}
	if prod.ID != "12345" {
		t.Errorf("ID: got %s, want 12345", prod.ID)
	}
	if prod.Code != "WDG-001" {
		t.Errorf("Code: got %s, want WDG-001", prod.Code)
	}
}

// TestPathSyntaxMixedWithNamespaces tests > path syntax with mixed namespace usage
func TestPathSyntaxMixedWithNamespaces(t *testing.T) {
	type Document struct {
		XMLName xml.Name `xml:"document"`
		Title   string   `xml:"meta>title"`
		Country string   `xml:"ns1:location>ns1:country"`
	}

	xmlData := []byte(`<document xmlns:ns1="http://example.com/loc">
		<meta>
			<title>Test Doc</title>
		</meta>
		<ns1:location>
			<ns1:country>Germany</ns1:country>
		</ns1:location>
	</document>`)

	var doc Document
	err := xmlctx.Unmarshal(xmlData, &doc,
		xmlctx.WithNamespaces(map[string]string{
			"ns1": "http://example.com/loc",
		}),
	)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if doc.Title != "Test Doc" {
		t.Errorf("Title: got %s, want 'Test Doc'", doc.Title)
	}
	if doc.Country != "Germany" {
		t.Errorf("Country: got %s, want Germany", doc.Country)
	}
}

// TestPathSyntaxWithIntegers tests > path syntax with integer fields
func TestPathSyntaxWithIntegers(t *testing.T) {
	type Quantity struct {
		XMLName xml.Name `xml:"order"`
		Amount  int      `xml:"details>quantity>amount"`
	}

	xmlData := []byte(`<order>
		<details>
			<quantity>
				<amount>42</amount>
			</quantity>
		</details>
	</order>`)

	var qty Quantity
	err := xmlctx.Unmarshal(xmlData, &qty, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if qty.Amount != 42 {
		t.Errorf("Amount: got %d, want 42", qty.Amount)
	}
}

// TestPathSyntaxNotFound tests > path syntax when path doesn't exist
func TestPathSyntaxNotFound(t *testing.T) {
	type MissingPath struct {
		XMLName xml.Name `xml:"root"`
		Value   string   `xml:"path>to>value"`
	}

	xmlData := []byte(`<root>
		<other>
			<element>data</element>
		</other>
	</root>`)

	var mp MissingPath
	err := xmlctx.Unmarshal(xmlData, &mp, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Field should remain empty if path not found
	if mp.Value != "" {
		t.Errorf("Value should be empty, got %s", mp.Value)
	}
}

// TestInnerXML tests ,innerxml tag
func TestInnerXML(t *testing.T) {
	type Document struct {
		XMLName  xml.Name `xml:"document"`
		ID       string   `xml:"id,attr"`
		Title    string   `xml:"title"`
		InnerXML string   `xml:",innerxml"`
	}

	xmlData := []byte(`<document id="doc-001">
		<title>Test</title>
		<content><data>value</data></content>
	</document>`)

	var doc Document
	err := xmlctx.Unmarshal(xmlData, &doc, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if doc.ID != "doc-001" {
		t.Errorf("ID: got %s, want doc-001", doc.ID)
	}
	// InnerXML should contain everything after title
	if !strings.Contains(doc.InnerXML, "<content>") || !strings.Contains(doc.InnerXML, "<data>value</data>") {
		t.Errorf("InnerXML: got %s, want to contain <content><data>value</data></content>", doc.InnerXML)
	}
}

// TestInnerXMLBytes tests ,innerxml with []byte field
func TestInnerXMLBytes(t *testing.T) {
	type Doc struct {
		XMLName  xml.Name `xml:"doc"`
		InnerXML []byte   `xml:",innerxml"`
	}

	xmlData := []byte(`<doc><a>1</a><b>2</b></doc>`)

	var doc Doc
	err := xmlctx.Unmarshal(xmlData, &doc, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	innerStr := string(doc.InnerXML)
	if !strings.Contains(innerStr, "<a>1</a>") || !strings.Contains(innerStr, "<b>2</b>") {
		t.Errorf("InnerXML bytes: got %s", innerStr)
	}
}

// TestAnyElement tests ,any tag for unmatched elements
func TestAnyElement(t *testing.T) {
	type Extension struct {
		XMLName xml.Name `xml:"extension"`
		Data    string   `xml:"data"`
	}

	type Config struct {
		XMLName xml.Name `xml:"config"`
		Name    string   `xml:"name"`
		Any     []Extension `xml:",any"`
	}

	xmlData := []byte(`<config>
		<name>test</name>
		<extension><data>ext1</data></extension>
		<extension><data>ext2</data></extension>
	</config>`)

	var cfg Config
	err := xmlctx.Unmarshal(xmlData, &cfg, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if cfg.Name != "test" {
		t.Errorf("Name: got %s, want test", cfg.Name)
	}
	if len(cfg.Any) != 2 {
		t.Errorf("Any elements: got %d, want 2", len(cfg.Any))
	}
	if len(cfg.Any) > 0 && cfg.Any[0].Data != "ext1" {
		t.Errorf("Any[0].Data: got %s, want ext1", cfg.Any[0].Data)
	}
}

// TestAnyAttr tests ,any,attr tag for unmatched attributes
func TestAnyAttr(t *testing.T) {
	type Element struct {
		XMLName xml.Name `xml:"element"`
		ID      string   `xml:"id,attr"`
		AnyAttr []xml.Attr `xml:",any,attr"`
	}

	xmlData := []byte(`<element id="123" version="1.0" status="active" />`)

	var elem Element
	err := xmlctx.Unmarshal(xmlData, &elem, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if elem.ID != "123" {
		t.Errorf("ID: got %s, want 123", elem.ID)
	}
	if len(elem.AnyAttr) != 2 {
		t.Errorf("AnyAttr: got %d attributes, want 2", len(elem.AnyAttr))
	}

	// Check that version and status are in AnyAttr
	foundVersion := false
	foundStatus := false
	for _, attr := range elem.AnyAttr {
		if attr.Name.Local == "version" && attr.Value == "1.0" {
			foundVersion = true
		}
		if attr.Name.Local == "status" && attr.Value == "active" {
			foundStatus = true
		}
	}
	if !foundVersion {
		t.Error("version attribute not found in AnyAttr")
	}
	if !foundStatus {
		t.Error("status attribute not found in AnyAttr")
	}
}

// TestCData tests ,cdata tag
func TestCData(t *testing.T) {
	type Article struct {
		XMLName xml.Name `xml:"article"`
		Title   string   `xml:"title"`
		Content string   `xml:",cdata"`
	}

	xmlData := []byte(`<article>
		<title>Test</title>
		<![CDATA[Content with <tags> & symbols]]>
	</article>`)

	var art Article
	err := xmlctx.Unmarshal(xmlData, &art, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if art.Title != "Test" {
		t.Errorf("Title: got %s, want Test", art.Title)
	}
	expected := "Content with <tags> & symbols"
	if art.Content != expected {
		t.Errorf("Content: got %s, want %s", art.Content, expected)
	}
}

// TestComments tests ,comment tag
func TestComments(t *testing.T) {
	type Doc struct {
		XMLName xml.Name `xml:"doc"`
		Title   string   `xml:"title"`
		Comment string   `xml:",comment"`
	}

	xmlData := []byte(`<doc>
		<!-- First comment -->
		<title>Test</title>
		<!-- Second comment -->
	</doc>`)

	var doc Doc
	err := xmlctx.Unmarshal(xmlData, &doc, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if doc.Title != "Test" {
		t.Errorf("Title: got %s, want Test", doc.Title)
	}
	if !strings.Contains(doc.Comment, "First comment") || !strings.Contains(doc.Comment, "Second comment") {
		t.Errorf("Comment: got %s, expected to contain both comments", doc.Comment)
	}
}

// TestCombinedSpecialTags tests multiple special tags together
func TestCombinedSpecialTags(t *testing.T) {
	type Advanced struct {
		XMLName xml.Name   `xml:"advanced"`
		ID      string     `xml:"id,attr"`
		AnyAttr []xml.Attr `xml:",any,attr"`
		Title   string     `xml:"title"`
		Comment string     `xml:",comment"`
		Data    string     `xml:",cdata"`
	}

	xmlData := []byte(`<advanced id="001" version="2.0" status="beta">
		<!-- Test comment -->
		<title>Advanced Test</title>
		<![CDATA[Special <data> here]]>
		<!-- Another comment -->
	</advanced>`)

	var adv Advanced
	err := xmlctx.Unmarshal(xmlData, &adv, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if adv.ID != "001" {
		t.Errorf("ID: got %s, want 001", adv.ID)
	}
	if len(adv.AnyAttr) != 2 {
		t.Errorf("AnyAttr: got %d, want 2", len(adv.AnyAttr))
	}
	if adv.Title != "Advanced Test" {
		t.Errorf("Title: got %s", adv.Title)
	}
	if !strings.Contains(adv.Comment, "Test comment") {
		t.Errorf("Comment missing 'Test comment': got %s", adv.Comment)
	}
	if adv.Data != "Special <data> here" {
		t.Errorf("Data: got %s", adv.Data)
	}
}

// TestInnerXMLWithNamespaces tests innerxml with namespaced content
func TestInnerXMLWithNamespaces(t *testing.T) {
	type Doc struct {
		XMLName  xml.Name `xml:"doc"`
		InnerXML string   `xml:",innerxml"`
	}

	xmlData := []byte(`<doc xmlns:ns="http://example.com">
		<ns:item>value</ns:item>
		<other>data</other>
	</doc>`)

	var doc Doc
	err := xmlctx.Unmarshal(xmlData, &doc, xmlctx.WithNamespaces(map[string]string{
		"ns": "http://example.com",
	}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// InnerXML should preserve namespace prefixes
	if !strings.Contains(doc.InnerXML, "ns:item") && !strings.Contains(doc.InnerXML, "<item") {
		t.Errorf("InnerXML should contain namespaced element: got %s", doc.InnerXML)
	}
}

// TestAnyWithPointer tests ,any with pointer field
func TestAnyWithPointer(t *testing.T) {
	type Extra struct {
		XMLName xml.Name `xml:"extra"`
		Value   string   `xml:"value"`
	}

	type Container struct {
		XMLName xml.Name `xml:"container"`
		Name    string   `xml:"name"`
		Any     *Extra   `xml:",any"`
	}

	xmlData := []byte(`<container>
		<name>test</name>
		<extra><value>extra data</value></extra>
	</container>`)

	var c Container
	err := xmlctx.Unmarshal(xmlData, &c, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if c.Name != "test" {
		t.Errorf("Name: got %s, want test", c.Name)
	}
	if c.Any == nil {
		t.Fatal("Any field is nil")
	}
	if c.Any.Value != "extra data" {
		t.Errorf("Any.Value: got %s, want 'extra data'", c.Any.Value)
	}
}

// TestEmptyComment tests comment field with no comments
func TestEmptyComment(t *testing.T) {
	type Doc struct {
		XMLName xml.Name `xml:"doc"`
		Title   string   `xml:"title"`
		Comment string   `xml:",comment"`
	}

	xmlData := []byte(`<doc><title>Test</title></doc>`)

	var doc Doc
	err := xmlctx.Unmarshal(xmlData, &doc, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if doc.Comment != "" {
		t.Errorf("Comment should be empty, got %s", doc.Comment)
	}
}

// TestEmptyInnerXML tests innerxml with no inner content
func TestEmptyInnerXML(t *testing.T) {
	type Doc struct {
		XMLName  xml.Name `xml:"doc"`
		InnerXML string   `xml:",innerxml"`
	}

	xmlData := []byte(`<doc></doc>`)

	var doc Doc
	err := xmlctx.Unmarshal(xmlData, &doc, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if doc.InnerXML != "" {
		t.Errorf("InnerXML should be empty, got %s", doc.InnerXML)
	}
}

// CustomType implements xml.Unmarshaler for custom unmarshaling
type CustomType struct {
	Value string
}

func (c *CustomType) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var s string
	if err := d.DecodeElement(&s, &start); err != nil {
		return err
	}
	c.Value = "custom:" + s
	return nil
}

// TestUnmarshalerInterface tests xml.Unmarshaler interface
func TestUnmarshalerInterface(t *testing.T) {
	type Doc struct {
		XMLName xml.Name    `xml:"doc"`
		Custom  CustomType  `xml:"custom"`
	}

	xmlData := []byte(`<doc><custom>test</custom></doc>`)

	var doc Doc
	err := xmlctx.Unmarshal(xmlData, &doc, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if doc.Custom.Value != "custom:test" {
		t.Errorf("Custom.Value: got %s, want 'custom:test'", doc.Custom.Value)
	}
}

// CustomAttr implements xml.UnmarshalerAttr for custom attribute unmarshaling
type CustomAttr struct {
	Value string
}

func (c *CustomAttr) UnmarshalXMLAttr(attr xml.Attr) error {
	c.Value = "attr:" + attr.Value
	return nil
}

// TestUnmarshalerAttrInterface tests xml.UnmarshalerAttr interface
func TestUnmarshalerAttrInterface(t *testing.T) {
	type Element struct {
		XMLName xml.Name   `xml:"element"`
		Custom  CustomAttr `xml:"custom,attr"`
	}

	xmlData := []byte(`<element custom="value" />`)

	var elem Element
	err := xmlctx.Unmarshal(xmlData, &elem, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if elem.Custom.Value != "attr:value" {
		t.Errorf("Custom.Value: got %s, want 'attr:value'", elem.Custom.Value)
	}
}

// TextUnmarshalType implements encoding.TextUnmarshaler
type TextUnmarshalType struct {
	Value int
}

func (t *TextUnmarshalType) UnmarshalText(text []byte) error {
	// Custom parsing: multiply by 10
	val, err := strconv.Atoi(string(text))
	if err != nil {
		return err
	}
	t.Value = val * 10
	return nil
}

// TestTextUnmarshalerInterface tests encoding.TextUnmarshaler interface
func TestTextUnmarshalerInterface(t *testing.T) {
	type Doc struct {
		XMLName xml.Name          `xml:"doc"`
		Custom  TextUnmarshalType `xml:"custom"`
	}

	xmlData := []byte(`<doc><custom>5</custom></doc>`)

	var doc Doc
	err := xmlctx.Unmarshal(xmlData, &doc, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if doc.Custom.Value != 50 {
		t.Errorf("Custom.Value: got %d, want 50", doc.Custom.Value)
	}
}

// TestTextUnmarshalerForAttr tests encoding.TextUnmarshaler for attributes
func TestTextUnmarshalerForAttr(t *testing.T) {
	type Element struct {
		XMLName xml.Name          `xml:"element"`
		Custom  TextUnmarshalType `xml:"custom,attr"`
	}

	xmlData := []byte(`<element custom="3" />`)

	var elem Element
	err := xmlctx.Unmarshal(xmlData, &elem, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if elem.Custom.Value != 30 {
		t.Errorf("Custom.Value: got %d, want 30", elem.Custom.Value)
	}
}

// TestXMLNameField tests XMLName field capturing
func TestXMLNameField(t *testing.T) {
	type Doc struct {
		XMLName xml.Name `xml:"document"`
		Title   string   `xml:"title"`
	}

	xmlData := []byte(`<document xmlns="http://example.com"><title>Test</title></document>`)

	var doc Doc
	err := xmlctx.Unmarshal(xmlData, &doc, xmlctx.WithNamespaces(map[string]string{
		"": "http://example.com",
	}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if doc.XMLName.Local != "document" {
		t.Errorf("XMLName.Local: got %s, want 'document'", doc.XMLName.Local)
	}
	if doc.XMLName.Space != "http://example.com" {
		t.Errorf("XMLName.Space: got %s, want 'http://example.com'", doc.XMLName.Space)
	}
	if doc.Title != "Test" {
		t.Errorf("Title: got %s, want Test", doc.Title)
	}
}

// TestXMLNameWithoutNamespace tests XMLName without namespace
func TestXMLNameWithoutNamespace(t *testing.T) {
	type Item struct {
		XMLName xml.Name `xml:"item"`
		Value   string   `xml:"value"`
	}

	xmlData := []byte(`<item><value>data</value></item>`)

	var item Item
	err := xmlctx.Unmarshal(xmlData, &item, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if item.XMLName.Local != "item" {
		t.Errorf("XMLName.Local: got %s, want 'item'", item.XMLName.Local)
	}
	if item.XMLName.Space != "" {
		t.Errorf("XMLName.Space: got %s, want empty", item.XMLName.Space)
	}
}

// TestTextUnmarshalerWithNestedElements tests TextUnmarshaler with nested XML elements
func TestTextUnmarshalerWithNestedElements(t *testing.T) {
	type Doc struct {
		XMLName xml.Name          `xml:"doc"`
		Custom  TextUnmarshalType `xml:"custom"`
	}

	// XML with nested elements that should be skipped by TextUnmarshaler
	xmlData := []byte(`<doc>
		<custom>
			<nested>ignored</nested>
			7
			<other>also ignored</other>
		</custom>
	</doc>`)

	var doc Doc
	err := xmlctx.Unmarshal(xmlData, &doc, xmlctx.WithNamespaces(map[string]string{}))
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// TextUnmarshaler should extract "7" and multiply by 10, ignoring nested elements
	if doc.Custom.Value != 70 {
		t.Errorf("Custom.Value: got %d, want 70", doc.Custom.Value)
	}
}
