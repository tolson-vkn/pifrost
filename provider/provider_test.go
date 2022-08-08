package provider

import (
	"reflect"
	"testing"
)

// TODO mock the API calls...

func TestValidChangeSet(t *testing.T) {
	expected := &dnsChangeSet{
		domain: domain{
			"1.2.3.4",
			"one.two.three.org.uk",
		},
		action: "add",
	}

	changeSet, _ := CreateChangeSet("1.2.3.4", "one.two.three.org.uk", "add")
	if !reflect.DeepEqual(expected, changeSet) {
		t.Error("Valid add changeset not parsed")
	}

	expected = &dnsChangeSet{
		domain: domain{
			"8.8.8.8",
			"google.tolson.io",
		},
		action: "add",
	}

	changeSet, _ = CreateChangeSet("8.8.8.8", "google.tolson.io", "add")
	if !reflect.DeepEqual(expected, changeSet) {
		t.Error("Valid add changeset not parsed")
	}

	expected = &dnsChangeSet{
		domain: domain{
			"8.8.8.8",
			"google.tolson.io",
		},
		action: "delete",
	}

	changeSet, _ = CreateChangeSet("8.8.8.8", "google.tolson.io", "delete")
	if !reflect.DeepEqual(expected, changeSet) {
		t.Error("Valid delete changeset not parsed")
	}
}

func TestInvalidChangeSet(t *testing.T) {
	// Domain must be RFC1123
	expected := "Could not parse change set domain [10.+1.1.1]"
	_, err := CreateChangeSet("8.8.8.8", "10.+1.1.1", "add")
	errMsg := err.Error()
	if errMsg != expected {
		t.Errorf("Error: %v, Expected: %v.", errMsg, expected)
	}

	// Change is actually an add
	expected = "Change set action must be add or delete."
	_, err = CreateChangeSet("8.8.8.8", "google.tolson.io", "change")
	errMsg = err.Error()
	if errMsg != expected {
		t.Errorf("Error: %v, Expected: %v.", errMsg, expected)
	}

	// We don't support CNAME
	expected = "Could not parse IP [google.com]"
	_, err = CreateChangeSet("google.com", "google.tolson.io", "add")
	errMsg = err.Error()
	if errMsg != expected {
		t.Errorf("Error: %v, Expected: %v.", errMsg, expected)
	}
}

func TestValidProvider(t *testing.T) {
	expected := &PiHoleRequest{
		insecure:      true,
		piholeAddress: "10.1.1.5",
		token:         "foobar",
	}

	provider, _ := InitDNSProvider(true, "10.1.1.5", "foobar")
	if !reflect.DeepEqual(expected, provider) {
		t.Error("Valid provider not returned")
	}

	expected = &PiHoleRequest{
		insecure:      false,
		piholeAddress: "pihole.tolson.io",
		token:         "foobar",
	}

	provider, _ = InitDNSProvider(false, "pihole.tolson.io", "foobar")
	if !reflect.DeepEqual(expected, provider) {
		t.Error("Valid provider not returned")
	}
}

func TestInvalidProvider(t *testing.T) {
	expected := "Could not parse pi-hole host/domain [not^domain.com]"
	_, err := InitDNSProvider(false, "not^domain.com", "foobar")
	errMsg := err.Error()
	if errMsg != expected {
		t.Errorf("Error: %v, Expected: %v.", errMsg, expected)
	}
}

func TestDomainExists(t *testing.T) {
	var expected bool = true
	d := domain{
		"8.8.8.8",
		"google.tolson.io",
	}

	var ds = []domain{
		{
			"8.8.8.8",
			"google.tolson.io",
		},
		{
			"10.1.1.1",
			"gateway.tolson.io",
		},
		{
			"10.2.1.4",
			"homeassistant.tolson.io",
		},
	}

	exists := domainExists(d.domain, ds)
	if expected != exists {
		t.Error("Domain not found in list of domains")
	}

	expected = false
	d = domain{
		"8.8.8.8",
		"donthave.tolson.io",
	}
	exists = domainExists(d.domain, ds)
	if expected != exists {
		t.Error("Domain was found in domains but shouldn't have")
	}
}

func TestDecodeDomains(t *testing.T) {
	noDataBytes := []byte("{\"data\":[]}[]")
	domains, _ := decodeDomains(noDataBytes)
	if len(domains) != 0 {
		t.Error("Found domains but should not have")
	}

	googleBytes := []byte("{\"data\":[[\"google.tolson.io\",\"8.8.8.8\"]]}[]")
	domains, _ = decodeDomains(googleBytes)
	if len(domains) != 1 {
		t.Error("Did not find domain.")
	}
}

func TestDecodeSuccess(t *testing.T) {
	// Normal case
	successBytes := []byte("{\"success\":true,\"message\":\"\"}{\"FTLnotrunning\":true}")
	expected := &successResponse{
		Success: true,
		Message: "",
	}
	decoded, _ := decodeSuccess(successBytes)
	if !reflect.DeepEqual(expected, decoded) {
		t.Error("Expected response was success, decoded was not")
	}

	// Duplicate
	successBytes = []byte("{\"success\":false,\"message\":\"This domain\\\\/ip association does not exist\"}[]")
	expected = &successResponse{
		Success: false,
		Message: "This domain\\/ip association does not exist",
	}
	decoded, _ = decodeSuccess(successBytes)
	if !reflect.DeepEqual(expected, decoded) {
		t.Error("Expected response reflection error")
	}

	// Already exists
	successBytes = []byte("{\"success\":false,\"message\":\"This domain already has a custom DNS entry for an IPv4\"}[]")
	expected = &successResponse{
		Success: false,
		Message: "This domain already has a custom DNS entry for an IPv4",
	}
	decoded, _ = decodeSuccess(successBytes)
	if !reflect.DeepEqual(expected, decoded) {
		t.Error("Expected response reflection error")
	}
}
