package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"reflect"
	"regexp"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	apiPath = "/admin/api.php"
)

type PiHoleRequest struct {
	insecure      bool
	piholeAddress string
	token         string
}

type domain struct {
	ip     string
	domain string
}

type dnsChangeSet struct {
	domain domain
	action string
}

type successResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Create a change set struct
func CreateChangeSet(ip, d, action string) (*dnsChangeSet, error) {
	// Is it really an IP?
	if pIP := net.ParseIP(ip); pIP == nil {
		return nil, fmt.Errorf("Could not parse IP [%s]", ip)
	}

	// Is the domain valid?
	re := regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*)([^a-z0-9-]|$)$`)
	if match := re.MatchString(d); match == false {
		return nil, fmt.Errorf("Could not parse change set domain [%s]", d)
	}

	// Is the action add or delete?
	if action != "add" && action != "delete" {
		return nil, errors.New("Change set action must be add or delete.")
	}

	logrus.WithFields(logrus.Fields{
		"ip":     ip,
		"domain": d,
		"action": action,
	}).Info("Creating change set")

	dnsChangeSet := &dnsChangeSet{
		domain{
			ip,
			d,
		},
		action,
	}

	return dnsChangeSet, nil
}

// Create a DNS provider request struct.
func InitDNSProvider(insecure bool, host, token string) (*PiHoleRequest, error) {
	// Check RFC1123 hostname
	re := regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*)([^a-z0-9-]|$)$`)
	if match := re.MatchString(host); match == false {
		return nil, fmt.Errorf("Could not parse pi-hole host/domain [%s]", host)
	}

	logrus.WithFields(logrus.Fields{
		"insecure": insecure,
		"host":     host,
	}).Info("Creating DNS Provider")

	piHoleRequest := &PiHoleRequest{
		insecure,
		host,
		token,
	}

	return piHoleRequest, nil
}

// Make GET request to check if the pi-hole accepts connections.
func (phr *PiHoleRequest) ValidateProvider() error {
	var count int = 1
	const tries int = 8
	for {
		logrus.Info("Attempting to reach pi-hole...")
		_, err := phr.GetDNS()
		// Have connection
		if err == nil {
			logrus.Info("Connected.")
			return nil
		}
		time.Sleep(1 << count * time.Second)
		count++
		if count == tries {
			break
		}
	}

	return errors.New("Failed to connect to pi-hole.")
}

func getDomain(d string, domains []domain) (*domain, error) {
	for _, domain := range domains {
		// Given domain in list of domains
		if d == domain.domain {
			return &domain, nil
		}
	}
	return nil, fmt.Errorf("Domain not found: %s", d)
}

func domainExists(d string, domains []domain) bool {
	for _, domain := range domains {
		// Given domain in list of domains
		if d == domain.domain {
			return true
		}
	}
	return false
}

func (phr *PiHoleRequest) GetDNS() ([]domain, error) {
	response, err := phr.doRequest("GET", nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to get DNS records: %s", err)
	}

	domains, err := decodeDomains(response)
	if err != nil {
		return nil, fmt.Errorf("Failed decode domains: %s", err)
	}

	return domains, nil
}

// Call safe add function or delete function.
func (phr *PiHoleRequest) ModifyDNS(dcs *dnsChangeSet) error {
	var err error = nil

	switch dcs.action {
	case "add":
		err = phr.add(dcs)
		if err != nil {
			return err
		}
	case "delete":
		err = phr.delete(dcs)
		if err != nil {
			return err
		}
	}

	return nil
}

// Add action but is also a change action.
func (phr *PiHoleRequest) add(dcs *dnsChangeSet) error {
	// Get all the current domains.
	domains, err := phr.GetDNS()
	if err != nil {
		return fmt.Errorf("Failed to add: %s", err)
	}

	logrus.WithFields(logrus.Fields{
		"domain": dcs.domain.domain,
		"ip":     dcs.domain.ip,
	}).Info("Creating record.")

	// If the domain exists, delete it to add new record.
	if domainExists(dcs.domain.domain, domains) {
		// We need the IP of this domain
		d, err := getDomain(dcs.domain.domain, domains)
		if err != nil {
			return fmt.Errorf("Domain: [%s] not found in list", dcs.domain.domain)
		}

		// We might already have done to work, so skip
		if reflect.DeepEqual(&dcs.domain, d) {
			logrus.WithFields(logrus.Fields{
				"domain": dcs.domain.domain,
			}).Info("Domain already exists with hostname and ip")
			return nil
		}

		// Domain exists but differs on IP
		logrus.WithFields(logrus.Fields{
			"domain": dcs.domain.domain,
		}).Info("Record with domain exists, change")
		var existingIP string
		for _, d := range domains {
			if dcs.domain.domain == d.domain {
				existingIP = d.ip
			}
		}
		err = phr.delete(&dnsChangeSet{
			domain: domain{
				existingIP,
				dcs.domain.domain,
			},
			action: "delete",
		})
		if err != nil {
			return fmt.Errorf("Could not change record: %s", err)
		}
	}

	response, err := phr.doRequest("POST", dcs)
	if err != nil {
		return fmt.Errorf("Could not add record: %s", err)
	}

	logrus.WithFields(logrus.Fields{
		"domain": dcs.domain.domain,
		"ip":     dcs.domain.ip,
	}).Info("Created record.")

	sR, err := decodeSuccess(response)
	if !sR.Success {
		return fmt.Errorf("Could not add record: %s", sR.Message)
	} else {
		return nil
	}
}

// Delete
func (phr *PiHoleRequest) delete(dcs *dnsChangeSet) error {
	domains, err := phr.GetDNS()
	if err != nil {
		return fmt.Errorf("Failed to delete: %s", err)
	}

	logrus.WithFields(logrus.Fields{
		"domain": dcs.domain.domain,
		"ip":     dcs.domain.ip,
	}).Info("Deleting record.")

	// If the domain exists, delete it to add new record.
	if domainExists(dcs.domain.domain, domains) {
		response, err := phr.doRequest("POST", dcs)
		if err != nil {
			fmt.Errorf("Could not delete record: %s", err)
		}
		sR, err := decodeSuccess(response)
		if !sR.Success {
			return fmt.Errorf("Could not delete record: %s", sR.Message)
		} else {
			logrus.WithFields(logrus.Fields{
				"domain": dcs.domain.domain,
				"ip":     dcs.domain.ip,
			}).Info("Deleted record.")
			return nil
		}
	} else {
		return errors.New("Record does not exist.")
	}
}

// Perform request against pi-hole API
func (phr *PiHoleRequest) doRequest(method string, dcs *dnsChangeSet) ([]byte, error) {
	var protocol string
	if phr.insecure {
		protocol = "http"
	} else {
		protocol = "https"
	}
	url := fmt.Sprintf("%s://%s%s", protocol, phr.piholeAddress, apiPath)

	// Make request
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, errors.New("Failed to create HTTP request.")
	}

	// Params
	q := req.URL.Query()
	// Key customdns specifices DNS api, has no value.
	q.Add("customdns", "")
	q.Add("auth", phr.token)

	if dcs != nil {
		q.Add("action", dcs.action)
		q.Add("ip", dcs.domain.ip)
		q.Add("domain", dcs.domain.domain)
	} else {
		q.Add("action", "get")
	}

	req.URL.RawQuery = q.Encode()
	// Scary secrets.
	logrus.Debugf("Query: %s", req.URL)

	// Perform request
	client := &http.Client{}
	// This API returns 200 on failure...
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("Error sending request to the server.")
	}

	defer resp.Body.Close()
	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.New("Failed to read response body.")
	}

	return responseBody, nil
}

// Decode the domains response
func decodeDomains(responseBody []byte) ([]domain, error) {
	// Hacky - Post returns two json objects
	// {"data":[["foo.example.xyz","10.1.1.1"],["bar.example.xyz","10.1.1.2"]]}[]
	// Use decoder to get the first object.
	// But then do some nasty stuff to turn list of list into list of struct... has to be better way.
	type domainResponse struct {
		Data [][]string `json:"data"`
	}

	reader := bytes.NewReader(responseBody)
	dec := json.NewDecoder(reader)
	var dR domainResponse
	loop := 0
	for {
		if loop == 1 {
			break
		}
		err := dec.Decode(&dR)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("Error decoding GET: %s", err)
		}
		logrus.Debugf("Found object: [%s]", dR)
		loop += 1
	}

	var domains []domain
	for _, value := range dR.Data {
		domain := domain{
			// ip
			value[1],
			// domain
			value[0],
		}

		domains = append(domains, domain)
	}
	logrus.Debugf("Created domain struct: [%s]", domains)

	return domains, nil
}

// Decode the success response
func decodeSuccess(responseBody []byte) (*successResponse, error) {
	// Hacky - Post returns two json objects
	// "{"success":true,"message":""}{"FTLnotrunning":true}"
	// Use a decoder, it won't decode second object into the struct.
	reader := bytes.NewReader(responseBody)
	dec := json.NewDecoder(reader)
	var sR successResponse
	loop := 0
	for {
		if loop == 1 {
			break
		}
		err := dec.Decode(&sR)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("Error decoding POST: %s", err)
		}
		loop += 1
	}

	return &sR, nil
}
