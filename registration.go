package main

import (
	"encoding/json"
	"github.com/myENA/consultant"
	"log"
	"net/url"
	"strconv"
	"strings"

	"fmt"
	"github.com/gorilla/websocket"
)

func parseTags(tags string) []string {
	return strings.Split(tags, ",")
}

// Make sure that the service is supposed to be tracked via labels and that there are exposed ports
func (d *ContainerData) isValid() bool {

	if d.Resource.Labels.PortsString == "" {
		if debug {
			log.Printf("Missing port string in labels for %s, skipping.", d.Resource.Name)
		}
		return false
	}

	// If the strict flag is passed check the herder.service.enable label
	if strict {
		if d.Resource.Labels.HerderServiceEnable == "" {
			log.Printf("Missing enable label for : %s", d.Resource.Name)
			return false
		}

		enable, err := strconv.ParseBool(d.Resource.Labels.HerderServiceEnable)

		if err != nil {
			log.Printf("Failed to parse enable, skipping. Error: %s", err)
			return false
		}

		return enable

	}

	return true
}

// Register the service in Consul
func registerSvc(data *ContainerData) {

	data.Resource.Labels.Ports = make([]*ContainerPorts, 0)
	// Unmarshall the k8s ports data
	err = json.Unmarshal([]byte(data.Resource.Labels.PortsString), &data.Resource.Labels.Ports)

	if err != nil {
		log.Printf("Failed to unmarshall exposed Ports: %v", err)
	}

	// If no ports return
	if len(data.Resource.Labels.Ports) == 0 {
		return
	}

	// For each port found register the service in consul as a separate service. Each port is reflected in the consul ID
	for _, p := range data.Resource.Labels.Ports {

		var checkPort int
		var checkTCP bool
		var scheme string

		// Setting defaults and checking label values
		if data.Resource.Labels.HerderServiceCheckTCP != "" {
			checkTCP, err = strconv.ParseBool(data.Resource.Labels.HerderServiceCheckTCP)

			if err != nil {
				log.Print(err)
				continue
			}
		}

		if data.Resource.Labels.HerderServiceName == "" {
			data.Resource.Labels.HerderServiceName = data.Resource.Labels.ContainerName
		}

		if data.Resource.Labels.HerderServiceCheckInterval == "" {
			data.Resource.Labels.HerderServiceCheckInterval = "15s"
		}

		if data.Resource.Labels.HerderServiceCheckPort != "" {
			checkPort, err = strconv.Atoi(data.Resource.Labels.HerderServiceCheckPort)

			if err != nil {
				log.Print(err)
				continue
			}
		}

		if data.Resource.Labels.HerderServiceCheckHTTPSchema == "" {
			scheme = "http"
		} else {
			scheme = data.Resource.Labels.HerderServiceCheckHTTPSchema
		}

		_, err := consul.SimpleServiceRegister(&consultant.SimpleServiceRegistration{
			Name:    data.Resource.Labels.HerderServiceName,
			Address: data.Resource.PrimaryIPAddress,
			ID: fmt.Sprintf("%s:%s:%s:%d:%s", data.Resource.HostID,
				data.Resource.Labels.ContainerName,
				data.Resource.ID, p.ContainerPort, p.Protocol),
			Port:        p.ContainerPort,
			Tags:        parseTags(data.Resource.Labels.HerderServiceTags),
			CheckPort:   checkPort,
			CheckPath:   data.Resource.Labels.HerderServiceCheckPath,
			Interval:    data.Resource.Labels.HerderServiceCheckInterval,
			CheckTCP:    checkTCP,
			CheckScheme: scheme},
		)

		if err != nil {
			log.Printf("Failed to register service %s: %s", data.Resource.Labels.ContainerName, err)
			continue
		}

		log.Printf("Registered %s", data.Resource.Name)
	}

}

// Grab the rancher WS url from the api
func getWS() *url.URL {
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

	if err != nil {
		log.Print(err)
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

	return u
}

// listen to events and register services
func processEvents(conn *websocket.Conn) error {
	// Print the WS Events
	for {
		v := Event{}
		_, r, err := conn.NextReader()
		if err != nil {
			return err
		}
		if err := json.NewDecoder(r).Decode(&v); err != nil {
			return err
		}

		dataMap := &ContainerData{}

		// Process Container messages
		if v.ResourceType == "container" {

			err = json.Unmarshal(v.Data, dataMap)

			if err != nil {
				log.Printf("Failed to unmarshall data: %v", err)
			}

			if dataMap.isValid() {
				// Is the running container on my host?
				if dataMap.Resource.State == "running" && hostID == dataMap.Resource.HostID {
					registerSvc(dataMap)
				}

				if dataMap.Resource.State == "stopped" && hostID == dataMap.Resource.HostID {
					servicePrefix := fmt.Sprintf("%s:%s", dataMap.Resource.HostID, dataMap.Resource.Name)
					serviceID := getConsulServiceID(servicePrefix)

					if serviceID == "" {
						continue
					}

					deRegister(serviceID)
				}

			} else {
				continue
			}
		}

	}
}
