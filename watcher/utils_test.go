package watcher

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	v1Networking "k8s.io/api/networking/v1"
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

func TestGetSvcAnnotation(t *testing.T) {
	annotations := map[string]string{
		"pifrost.tolson.io/domain": "example.com",
		"another.annotation":       "value",
	}

	// Test case 1: Valid annotation exists
	result, found := getSvcAnnotation(annotations)
	if !found {
		t.Errorf("Expected annotation to be found, but it was not")
	}
	if result != "example.com" {
		t.Errorf("Expected annotation value to be 'example.com', but got '%s'", result)
	}

	// Test case 2: Valid annotation does not exist
	annotations = map[string]string{
		"another.annotation": "value",
	}
	result, found = getSvcAnnotation(annotations)
	if found {
		t.Errorf("Expected annotation to not be found, but it was found")
	}
	if result != "" {
		t.Errorf("Expected empty string, but got '%s'", result)
	}

	// Test case 3: Empty annotations map
	annotations = map[string]string{}
	result, found = getSvcAnnotation(annotations)
	if found {
		t.Errorf("Expected annotation to not be found, but it was found")
	}
	if result != "" {
		t.Errorf("Expected empty string, but got '%s'", result)
	}
}

func TestHasIngressAnnotation(t *testing.T) {
	// Test case 1: Annotation "pifrost.tolson.io/ingress" is present with value "true"
	annotations1 := map[string]string{
		"pifrost.tolson.io/ingress": "true",
	}
	result1 := hasIngressAnnotation(annotations1)
	if !result1 {
		t.Errorf("Expected true, but got false")
	}

	// Test case 2: Annotation "pifrost.tolson.io/ingress" is present with value "false"
	annotations2 := map[string]string{
		"pifrost.tolson.io/ingress": "false",
	}
	result2 := hasIngressAnnotation(annotations2)
	if result2 {
		t.Errorf("Expected false, but got true")
	}

	// Test case 3: Annotation "pifrost.tolson.io/ingress" is present with an invalid value
	annotations3 := map[string]string{
		"pifrost.tolson.io/ingress": "invalid",
	}
	result3 := hasIngressAnnotation(annotations3)
	if result3 {
		t.Errorf("Expected false, but got true")
	}

	// Test case 4: Annotation "pifrost.tolson.io/ingress" is not present
	annotations4 := map[string]string{
		"another.annotation": "value",
	}
	result4 := hasIngressAnnotation(annotations4)
	if result4 {
		t.Errorf("Expected false, but got true")
	}

	// Test case 5: Empty annotations map
	annotations5 := map[string]string{}
	result5 := hasIngressAnnotation(annotations5)
	if result5 {
		t.Errorf("Expected false, but got true")
	}
}

func TestConvertToIngress(t *testing.T) {
	// Test case 1: Valid conversion from interface{} to *v1Networking.Ingress
	validIngress := &v1Networking.Ingress{}
	result1, err1 := convertToIngress(validIngress)
	if err1 != nil {
		t.Errorf("Expected no error, but got an error: %v", err1)
	}
	if result1 != validIngress {
		t.Errorf("Expected result to be the same as input, but they are different")
	}

	// Test case 2: Invalid conversion from interface{} to *v1Networking.Ingress
	obj2 := "invalid"
	result2, err2 := convertToIngress(obj2)
	if err2 == nil {
		t.Errorf("Expected an error, but got none")
	}
	expectedErrorMessage := fmt.Sprintf("cast failed %T to %T", obj2, validIngress)
	if err2.Error() != expectedErrorMessage {
		t.Errorf("Expected error message '%s', but got '%s'", expectedErrorMessage, err2.Error())
	}
	if result2 != nil {
		t.Errorf("Expected result to be nil, but it is not")
	}
}

func TestConvertToService(t *testing.T) {
	// Test case 1: Valid conversion from interface{} to *v1.Service
	validService := &v1.Service{}
	result1, err1 := convertToService(validService)
	if err1 != nil {
		t.Errorf("Expected no error, but got an error: %v", err1)
	}
	if result1 != validService {
		t.Errorf("Expected result to be the same as input, but they are different")
	}

	// Test case 2: Invalid conversion from interface{} to *v1.Service
	obj2 := "invalid"
	result2, err2 := convertToService(obj2)
	if err2 == nil {
		t.Errorf("Expected an error, but got none")
	}
	expectedErrorMessage := fmt.Sprintf("cast failed %T to %T", obj2, validService)
	if err2.Error() != expectedErrorMessage {
		t.Errorf("Expected error message '%s', but got '%s'", expectedErrorMessage, err2.Error())
	}
	if result2 != nil {
		t.Errorf("Expected result to be nil, but it is not")
	}
}

func TestSameHosts(t *testing.T) {
	// Test case 1: Both slices are empty, should return true
	slice1 := []string{}
	slice2 := []string{}
	result1 := sameHosts(slice1, slice2)
	if !result1 {
		t.Errorf("Expected true, but got false")
	}

	// Test case 2: Both slices have the same elements in the same order, should return true
	slice3 := []string{"example.com", "test.com", "sample.com"}
	slice4 := []string{"example.com", "test.com", "sample.com"}
	result2 := sameHosts(slice3, slice4)
	if !result2 {
		t.Errorf("Expected true, but got false")
	}

	// Test case 3: Both slices have the same elements in a different order, should return true
	slice5 := []string{"sample.com", "test.com", "example.com"}
	slice6 := []string{"example.com", "test.com", "sample.com"}
	result3 := sameHosts(slice5, slice6)
	if !result3 {
		t.Errorf("Expected true, but got false")
	}

	// Test case 4: Slices have different lengths, should return false
	slice7 := []string{"example.com", "test.com", "sample.com"}
	slice8 := []string{"example.com", "test.com"}
	result4 := sameHosts(slice7, slice8)
	if result4 {
		t.Errorf("Expected false, but got true")
	}

	// Test case 5: One slice is empty, should return false
	slice9 := []string{"example.com", "test.com", "sample.com"}
	slice10 := []string{}
	result5 := sameHosts(slice9, slice10)
	if result5 {
		t.Errorf("Expected false, but got true")
	}
}

func TestHostsAddedRemovedBoth(t *testing.T) {
	// Test case 1: Both slices are empty, should return empty slices for added, removed, and both
	slice1 := []string{}
	slice2 := []string{}
	added1, removed1, both1 := hostsAddedRemovedBoth(slice1, slice2)
	if len(added1) != 0 || len(removed1) != 0 || len(both1) != 0 {
		t.Errorf("Expected empty slices, but got non-empty slices")
	}

	// Test case 2: Both slices are the same, should return empty slices for added and removed, and non-empty for both
	slice3 := []string{"example.com", "test.com", "sample.com"}
	added2, removed2, both2 := hostsAddedRemovedBoth(slice3, slice3)
	if len(added2) != 0 || len(removed2) != 0 || !reflect.DeepEqual(both2, slice3) {
		t.Errorf("Expected empty slices for added and removed, and the same slice for both")
	}

	// Test case 3: One slice is empty, the other is not, should return added for the non-empty slice and empty for removed and both
	slice4 := []string{}
	slice5 := []string{"example.com", "test.com", "sample.com"}
	added3, removed3, both3 := hostsAddedRemovedBoth(slice4, slice5)
	if !reflect.DeepEqual(added3, slice5) || len(removed3) != 0 || len(both3) != 0 {
		t.Errorf("Expected added slice to be the non-empty slice, and empty slices for removed and both")
	}
}
