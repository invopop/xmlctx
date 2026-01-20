package xmlctx_test

import (
	"encoding/xml"
	"fmt"
	"os"
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
