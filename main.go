package main

import (
	"encoding/json"
	"github.com/myENA/consultant"
	"github.com/rancherio/go-rancher/v3"
	"log"
	"net/url"
	"os"
	"strconv"
)

// Get the rancher api data from Environment variables
var cattle_url = os.Getenv("CATTLE_URL")
var cattle_access_key = os.Getenv("CATTLE_ACCESS_KEY")
var cattle_secret_key = os.Getenv("CATTLE_SECRET_KEY")

// Set the rancher client
var c = &client.RancherClient{}
var err error

//func parsePorts(ports []string) int {
//	var port int
//	for _, s := range ports {
//		if strings.Contains(s, ":") {
//			log.Print("Stipping colon")
//			stringPort := strings.Split(s, ":")[0]
//			port, err := strconv.Atoi(stringPort)
//		}
//	}
//
//	return port
//
//}

// Define a websocket event
type Event struct {
	Name         string          `json:"name"`
	ResourceType string          `json:"resourceType"`
	ResourceID   string          `json:"resourceId"`
	Data         json.RawMessage `json:"data"`
}

// Define the data to be extracted from Event.Data
type ContainerData struct {
	Resource struct {
		Name             string   `json:"name,omitempty"`
		Expose           []string `json:"expose,omitempty"`
		Ports            []string `json:"ports,omitempty"`
		PrimaryIPAddress string   `json:"primaryIpAddress,omitempty"`
		State            string   `json:"state,omitempty"`
		Labels           struct {
			HerderInclude                   string `json:"herder.include,omitempty"`
			HerderServiceCheckHTTP          string `json:"herder.service.check.http,omitempty"`
			HerderServiceCheckHTTPPath      string `json:"herder.service.check.http.path,omitempty"`
			HerderServiceCheckHTTPSchema    string `json:"herder.service.check.http.schema,omitempty"`
			HerderServiceCheckInitialStatus string `json:"herder.service.check.initial_status,omitempty"`
			HerderServiceCheckInterval      string `json:"herder.service.check.interval,omitempty"`
			HerderServiceCheckScript        string `json:"herder.service.check.script,omitempty"`
			HerderServiceCheckTCP           string `json:"herder.service.check.tcp,omitempty"`
			HerderServiceCheckTimeout       string `json:"herder.service.check.timeout,omitempty"`
			HerderServiceCheckTTL           string `json:"herder.service.check.ttl,omitempty"`
			HerderServiceName               string `json:"herder.service.name,omitempty"`
			HerderServicePort               string `json:"herder.service.port,omitempty"`
			HerderServiceTags               string `json:"herder.service.tags,omitempty"`
		} `json:"labels,omitempty"`
	} `json:"resource,omitempty"`
}

// Build the Consul ServiceRegistration struct
func buildSvcConfig(data *ContainerData) *consultant.SimpleServiceRegistration {

	tags := parseTags(data.Resource.Labels.HerderServiceTags)
	tcp, err := strconv.ParseBool(data.Resource.Labels.HerderServiceCheckTCP)

	if err != nil {
		log.Print(err)
		return nil
	}

	svc := &consultant.SimpleServiceRegistration{
		Name:        data.Resource.Name,
		Tags:        tags,
		CheckPort:   8080,
		Address:     data.Resource.PrimaryIPAddress,
		CheckPath:   data.Resource.Labels.HerderServiceCheckHTTPPath,
		CheckTCP:    tcp,
		Interval:    data.Resource.Labels.HerderServiceCheckInterval,
		CheckScheme: data.Resource.Labels.HerderServiceCheckHTTPSchema,
	}

	return svc
}

func init() {

	// Configure cattle client
	config := &client.ClientOpts{
		Url:       cattle_url,
		AccessKey: cattle_access_key,
		SecretKey: cattle_secret_key,
	}

	c, err = client.NewRancherClient(config)
	if err != nil {
		log.Panic(err)
	}

	log.Print("Client Connection Established...")

}

func main() {

	// Get the subscribe schema
	schemas, _ := c.GetSchemas().CheckSchema("subscribe")

	// Extract the url
	urlString := schemas.Links["collection"]

	// Encode the URL and Query
	// Pulled from the rancher-cli code
	u, err := url.Parse(urlString)
	if err != nil {
		log.Panic(err)
	}

	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}

	q := u.Query()
	q.Add("eventNames", "resource.change")
	q.Add("eventNames", "service.kubernetes.change")
	u.RawQuery = q.Encode()

	// Establish websocket connection
	conn, resp, err := c.Websocket(u.String(), nil)
	if err != nil {
		log.Panic(err)
	}

	if resp.StatusCode != 101 {
		log.Printf("Bad status code: %d %s", resp.StatusCode, resp.Status)
	}

	log.Printf("Listening on websocket connection: %s", u.String())
	defer conn.Close()

	// Print the WS Events
	for {
		v := Event{}
		_, r, err := conn.NextReader()
		if err != nil {
			log.Print(err)
		}
		if err := json.NewDecoder(r).Decode(&v); err != nil {
			log.Print("Failed to parse json in message")
			continue
		}

		// Process Container messages
		if v.ResourceType == "container" {
			dataMap := &ContainerData{}

			//Get the container data we want
			err = json.Unmarshal(v.Data, dataMap)
			if err != nil {
				log.Print(err)
			}

			rsc := dataMap.Resource

			// Check for exposed ports
			if rsc.Expose != nil {
				log.Printf("%s", rsc.Expose)
			} else {
				continue
			}

			// Check for Mapped ports
			if rsc.Ports != nil {
				log.Printf("%s", rsc.Ports)
			}

			log.Printf("%s", rsc.Labels)
		}
	}

}
