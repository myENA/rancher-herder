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

func registerSvc(data *ContainerData) []string {

	var registered []string

	for _, p := range data.Resource.Labels.Ports {

		var checkPort int
		var checkTcp bool
		var scheme string

		if data.Resource.Labels.HerderServiceCheckTCP != "" {
			checkTcp, err = strconv.ParseBool(data.Resource.Labels.HerderServiceCheckTCP)

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

		reg, err := consul.SimpleServiceRegister(&consultant.SimpleServiceRegistration{
			Name:        data.Resource.Labels.ContainerName,
			Address:     data.Resource.PrimaryIPAddress,
			ID:          fmt.Sprintf("%s-%s-%s-%d-%s", data.Resource.HostId, data.Resource.Labels.ContainerName, data.Resource.ID, p.ContainerPort, p.Protocol),
			Port:        p.ContainerPort,
			Tags:        parseTags(data.Resource.Labels.HerderServiceTags),
			CheckPort:   checkPort,
			CheckPath:   data.Resource.Labels.HerderServiceCheckPath,
			Interval:    data.Resource.Labels.HerderServiceCheckInterval,
			CheckTCP:    checkTcp,
			CheckScheme: scheme},
		)

		if err != nil {
			log.Printf("Failed to register service %s: %s", data.Resource.Labels.ContainerName, err)
			continue
		}

		registered = append(registered, reg)
	}

	return registered
}

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

			// Do we register the service
			if dataMap.Resource.Labels.HerderIgnore != "" {
				ignore, err := strconv.ParseBool(dataMap.Resource.Labels.HerderIgnore)

				if err != nil {
					log.Printf("Failed to parse ignore, skipping. Error: %s", err)
					continue
				}

				if ignore {
					continue
				}

			}

			if dataMap.Resource.Labels.PortsString == "" {
				continue
			}

			dataMap.Resource.Labels.Ports = make([]*ContainerPorts, 0)

			err = json.Unmarshal([]byte(dataMap.Resource.Labels.PortsString), &dataMap.Resource.Labels.Ports)

			if err != nil {
				log.Printf("Failed to unmarshall exposed Ports: %v", err)
			}

			if len(dataMap.Resource.Labels.Ports) == 0 {
				continue
			}

			if dataMap.Resource.State == "running" && hostId == dataMap.Resource.HostId {

				regs := registerSvc(dataMap)
				log.Printf("Registered Services: %s", strings.Join(regs, ", "))
			}

		}

	}

	return nil
}
