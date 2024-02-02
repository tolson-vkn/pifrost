package provider

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func startMockServer(t *testing.T) (*httptest.Server, string) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var mockResponse string
		action := r.URL.Query().Get("action")
		switch action {
		case "get":
			mockResponse = `{"data":[["example.com","192.168.1.2"]]}[]`
		case "add":
			mockResponse = `{"success":true,"message":""}{"FTLnotrunning":true}`
		case "delete":
			mockResponse = `{"success":true,"message":""}{"FTLnotrunning":true}`
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponse))
	}))

	return mockServer, strings.Replace(mockServer.URL, "http://", "", 1)
}

func TestGetDNSProvider(t *testing.T) {
	mockServer, serverURL := startMockServer(t)
	defer mockServer.Close()

	// Test case 1: Valid get DNS
	expected := []domain{
		{
			ip:     "192.168.1.2",
			domain: "example.com",
		},
	}

	mockPHR := &PiHoleRequest{
		insecure:      true,
		piholeAddress: serverURL,
		token:         "mocktoken",
	}

	domains, err := mockPHR.GetDNS()
	if err != nil {
		t.Errorf("Error from GetDNS: %s", err)
	}
	if !reflect.DeepEqual(expected, domains) {
		t.Error("Did not get valid example domains")
	}
}

func TestModifyDNS(t *testing.T) {
	// Test case 1: Add DNS
	mockServer, serverURL := startMockServer(t)
	defer mockServer.Close()

	dcs := &dnsChangeSet{
		domain: domain{
			domain: "example.com",
			ip:     "192.168.1.1",
		},
		action: "add",
	}

	mockPHR := &PiHoleRequest{
		insecure:      true,
		piholeAddress: serverURL,
		token:         "mocktoken",
	}

	err := mockPHR.add(dcs)
	if err != nil {
		t.Errorf("Error from GetDNS: %s", err)
	}

	// Test case 2: Add Duplicate
	dcs = &dnsChangeSet{
		domain: domain{
			domain: "example.com",
			ip:     "192.168.1.2",
		},
		action: "add",
	}

	mockPHR = &PiHoleRequest{
		insecure:      true,
		piholeAddress: serverURL,
		token:         "mocktoken",
	}

	err = mockPHR.add(dcs)
	if err != nil {
		t.Errorf("Error from GetDNS: %s", err)
	}

	// Test case 3: Delete
	dcs = &dnsChangeSet{
		domain: domain{
			domain: "example.com",
			ip:     "192.168.1.2",
		},
		action: "delete",
	}

	mockPHR = &PiHoleRequest{
		insecure:      true,
		piholeAddress: serverURL,
		token:         "mocktoken",
	}

	err = mockPHR.delete(dcs)
	if err != nil {
		t.Errorf("Error from GetDNS: %s", err)
	}

	// Test case 4: Delete record not found
	dcs = &dnsChangeSet{
		domain: domain{
			domain: "boop.example.com",
			ip:     "192.168.1.1",
		},
		action: "delete",
	}

	mockPHR = &PiHoleRequest{
		insecure:      true,
		piholeAddress: serverURL,
		token:         "mocktoken",
	}

	err = mockPHR.delete(dcs)
	if err.Error() != "Record does not exist." {
		t.Errorf("Error from GetDNS: %s", err)
	}
}

func TestValidChangeSet(t *testing.T) {
	// Test case 1: Valid changeset
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
	// Test case 1: Domain must be RFC1123
	expected := "Could not parse change set domain [10.+1.1.1]"
	_, err := CreateChangeSet("8.8.8.8", "10.+1.1.1", "add")
	errMsg := err.Error()
	if errMsg != expected {
		t.Errorf("Error: %v, Expected: %v.", errMsg, expected)
	}

	// Test case 2: Change is actually an add
	expected = "Change set action must be add or delete."
	_, err = CreateChangeSet("8.8.8.8", "google.tolson.io", "change")
	errMsg = err.Error()
	if errMsg != expected {
		t.Errorf("Error: %v, Expected: %v.", errMsg, expected)
	}

	// Test case 3: We don't support CNAME
	expected = "Could not parse IP [google.com]"
	_, err = CreateChangeSet("google.com", "google.tolson.io", "add")
	errMsg = err.Error()
	if errMsg != expected {
		t.Errorf("Error: %v, Expected: %v.", errMsg, expected)
	}
}

func TestValidProvider(t *testing.T) {
	// Test case 1: Valid provider
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

func TestValidateProvider(t *testing.T) {
	// Validate
	var i int = 0

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var mockResponse string = `{"data":[["example.com","192.168.1.1"],["boop.example.com","192.168.1.2"]]}[]`
		if i < 2 {
			mockResponse = ""
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			mockResponse = `{"data":[["example.com","192.168.1.1"],["test.com","192.168.1.2"]]}[]`
			w.WriteHeader(http.StatusOK)
		}
		w.Write([]byte(mockResponse))
		i = i + 1
	}))
	defer mockServer.Close()

	mockPHR := &PiHoleRequest{
		insecure:      true,
		piholeAddress: strings.Replace(mockServer.URL, "http://", "", 1),
		token:         "mocktoken",
	}

	err := mockPHR.ValidateProvider()
	if err != nil {
		t.Error("Didn't validate the provider in time")
	}
}

func TestInvalidProvider(t *testing.T) {
	// Test case 1: Domain parse failure
	expected := "Could not parse pi-hole host/domain [not^domain.com]"
	_, err := InitDNSProvider(false, "not^domain.com", "foobar")
	errMsg := err.Error()
	if errMsg != expected {
		t.Errorf("Error: %v, Expected: %v.", errMsg, expected)
	}
}

func TestDomainExists(t *testing.T) {
	// Test case 1: Domain exists in list of domains
	var expected bool = true
	d := domain{
		"8.8.8.8",
		"boop.example.com",
	}

	var ds = []domain{
		{
			"8.8.8.8",
			"boop.example.com",
		},
		{
			"10.1.1.1",
			"gateway.example.com",
		},
		{
			"10.2.1.4",
			"homeassistant.example.com",
		},
	}

	exists := domainExists(d.domain, ds)
	if expected != exists {
		t.Error("Domain not found in list of domains")
	}

	// Test case 2: Domain does not exist in list of domains
	expected = false
	d = domain{
		"8.8.8.8",
		"donthave.example.com",
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

	googleBytes := []byte("{\"data\":[[\"google.example.com\",\"8.8.8.8\"]]}[]")
	domains, _ = decodeDomains(googleBytes)
	if len(domains) != 1 {
		t.Error("Did not find domain.")
	}
}

func TestDecodeSuccess(t *testing.T) {
	// Test case 1: Normal case
	successBytes := []byte("{\"success\":true,\"message\":\"\"}{\"FTLnotrunning\":true}")
	expected := &successResponse{
		Success: true,
		Message: "",
	}
	decoded, _ := decodeSuccess(successBytes)
	if !reflect.DeepEqual(expected, decoded) {
		t.Error("Expected response was success, decoded was not")
	}

	// Test case 2: Duplicate
	successBytes = []byte("{\"success\":false,\"message\":\"This domain\\\\/ip association does not exist\"}[]")
	expected = &successResponse{
		Success: false,
		Message: "This domain\\/ip association does not exist",
	}
	decoded, _ = decodeSuccess(successBytes)
	if !reflect.DeepEqual(expected, decoded) {
		t.Error("Expected response reflection error")
	}

	// Test case 3: Already exists
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
