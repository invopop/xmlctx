package main

import (
	"encoding/xml"
	"fmt"
	"log"

	"github.com/invopop/xmlctx"
)

func main() {
	// This XML uses the addr: prefix for address elements
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<person xmlns="http://example.com/user" xmlns:addr="http://example.com/address">
  <name>Jane Smith</name>
  <email>jane@example.com</email>
  <addr:city>San Francisco</addr:city>
  <addr:country>USA</addr:country>
</person>`)

	// Define struct with namespace declarations for marshaling
	type Person struct {
		XMLName xml.Name `xml:"person"`
		Xmlns   string   `xml:"xmlns,attr"`
		XmlnsA  string   `xml:"xmlns:addr,attr"`
		Name    string   `xml:"name"`
		Email   string   `xml:"email"`
		City    *string  `xml:"addr:city"`
		Country *string  `xml:"addr:country"`
	}

	var person Person
	err := xmlctx.Unmarshal(xmlData, &person,
		xmlctx.WithNamespaces(map[string]string{
			"":     "http://example.com/user",
			"addr": "http://example.com/address",
		}),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(person.XmlnsA)
	fmt.Println(person.Xmlns)

	out, _ := xml.MarshalIndent(person, "", "  ")
	fmt.Println(string(out))

	fmt.Printf("Name: %s\n", person.Name)
	fmt.Printf("Email: %s\n", person.Email)
	fmt.Printf("City: %s\n", *person.City)
	fmt.Printf("Country: %s\n", *person.Country)
}
