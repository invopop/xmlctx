package main

import (
	"encoding/xml"
	"fmt"
	"log"

	"github.com/invopop/xmlctx"
)

func main() {
	// The same XML using different prefixes (a: instead of addr:)
	// demonstrates that xmlctx matches on namespace URI, not prefix
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<person xmlns="http://example.com/user" xmlns:a="http://example.com/address">
  <name>Jane Smith</name>
  <email>jane@example.com</email>
  <a:city>San Francisco</a:city>
  <a:country>USA</a:country>
</person>`)

	type Person struct {
		XMLName xml.Name `xml:"http://example.com/user person"`
		XmlnsA  string   `xml:"xmlns:addr,attr"`
		Name    string   `xml:"name"`
		Email   string   `xml:"email"`
		City    string   `xml:"addr:city"`
		Country string   `xml:"addr:country"`
	}

	var person Person
	// xmlctx maps "addr" to the address namespace URI
	// This works even though the XML uses "a:" as the prefix
	err := xmlctx.Parse(xmlData, &person,
		xmlctx.WithNamespaces(map[string]string{
			"":     "http://example.com/user",
			"addr": "http://example.com/address",
		}),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("City: %s\n", person.City)
	fmt.Printf("Country: %s\n", person.Country)
	fmt.Println("\nNote: The XML used 'a:' as the prefix, but our struct tags use 'addr:'.")
	fmt.Println("This works because xmlctx matches on namespace URI, not prefix name!")
}
