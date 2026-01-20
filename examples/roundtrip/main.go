package main

import (
	"encoding/xml"
	"fmt"
	"log"

	"github.com/invopop/xmlctx"
)

func main() {
	// Demonstrates marshaling and unmarshaling with the same struct
	type Person struct {
		XMLName xml.Name `xml:"person"`
		Xmlns   string   `xml:"xmlns,attr"`
		XmlnsA  string   `xml:"xmlns:addr,attr"`
		Name    string   `xml:"name"`
		Email   string   `xml:"email"`
		City    string   `xml:"addr:city"`
		Country string   `xml:"addr:country"`
	}

	// Create and marshal a Person
	person := Person{
		Xmlns:   "http://example.com/user",
		XmlnsA:  "http://example.com/address",
		Name:    "Jane Smith",
		Email:   "jane@example.com",
		City:    "San Francisco",
		Country: "USA",
	}

	xmlData, err := xml.MarshalIndent(person, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Marshaled XML:")
	fmt.Println(xml.Header + string(xmlData))

	// Unmarshal back using xmlctx
	var decoded Person
	err = xmlctx.Unmarshal(xmlData, &decoded,
		xmlctx.WithNamespaces(map[string]string{
			"":     "http://example.com/user",
			"addr": "http://example.com/address",
		}),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nDecoded:\n")
	fmt.Printf("Name: %s\n", decoded.Name)
	fmt.Printf("City: %s\n", decoded.City)
	fmt.Printf("\nRound-trip successful! Same struct works for both marshal and unmarshal.\n")
}
