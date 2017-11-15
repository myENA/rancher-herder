package main

import (
	"encoding/json"
	"flag"
	"github.com/hashicorp/consul/api"
	"github.com/myENA/consultant"
	"github.com/rancher/go-rancher-metadata/metadata"
	"github.com/rancherio/go-rancher/v3"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)


//TODO: Add strict flag

// Get the rancher api data from Environment variables
var cattle_url = os.Getenv("CATTLE_URL")
var cattle_access_key = os.Getenv("CATTLE_ACCESS_KEY")
var cattle_secret_key = os.Getenv("CATTLE_SECRET_KEY")

var agentIp string
var hostUUID string
var hostId string
var registered []string

var consulServices []interface{}
var rancherServices []interface{}

const metadataUrl = "http://rancher-metadata/latest"

var interval time.Duration

// Set the rancher client
var c = &client.RancherClient{}
var err error

//set global consul client
var consul *consultant.Client

// Define a websocket event
type Event struct {
	Name         string          `json:"name"`
	ResourceType string          `json:"resourceType"`
	ResourceID   string          `json:"resourceId"`
	Data         json.RawMessage `json:"data"`
}

type ContainerPorts struct {
	HostPort      int    `json:"hostPort"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol"`
}

// Define the data to be extracted from Event.Data
type ContainerData struct {
	Resource struct {
		ID               string `json:"id,omitempty"`
		HostId           string `json:"hostId,omitempty"`
		Name             string `json:"name,omitempty"`
		PrimaryIPAddress string `json:"primaryIpAddress,omitempty"`
		State            string `json:"state,omitempty"`
		Labels           struct {
			ContainerName                string `json:"io.rancher.container.name,omitempty" mapstructure:"io.rancher.container.name"`
			PortsString                  string `json:"annotation.io.kubernetes.container.ports,omitempty" mapstructure:"annotation.io.kubernetes.container.ports"`
			Ports                        []*ContainerPorts
			HerderServiceEnable          string `json:"herder.service.enable,omitempty" mapstructure:"herder.service.enable"`
			HerderServiceCheckPath       string `json:"herder.service.check.http.path,omitempty" mapstructure:"herder.service.check.http.path"`
			HerderServiceCheckPort       string `json:"herder.service.check.http.port,omitempty" mapstructure:"herder.service.check.http.port"`
			HerderServiceCheckHTTPSchema string `json:"herder.service.check.http.schema,omitempty" mapstructure:"herder.service.check.http.schema"`
			HerderServiceCheckInterval   string `json:"herder.service.check.interval,omitempty" mapstructure:"herder.service.check.interval"`
			HerderServiceCheckTCP        string `json:"herder.service.check.tcp,omitempty" mapstructure:"herder.service.check.tcp"`
			HerderServiceName            string `json:"herder.service.name,omitempty" mapstructure:"herder.service.name"`
			HerderServiceTags            string `json:"herder.service.tags,omitempty" mapstructure:"herder.service.tags"`
		} `json:"labels,omitempty"`
	} `json:"resource,omitempty"`
}

type ServiceName struct {
	HostId        string
	ContainerName string
	ContainerId   string
	ContainerPort string
	Protocol      string
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

	m := metadata.NewClient(metadataUrl)

	self, err := m.GetSelfHost()

	if err != nil {
		log.Printf("Could not get host meta-data")
	}

	agentIp = self.AgentIP
	hostUUID = self.UUID

	hosts, err := c.Host.List(nil)

	if err != nil {
		log.Print("Failed to get list of hosts")
	}

	for _, h := range hosts.Data {
		if h.Uuid == hostUUID {
			hostId = h.Id
		}
	}

	log.Printf("Monitoring events on Host %s with HostId %s", agentIp, hostId)

	conf := &api.Config{Address: agentIp + ":8500"}

	consul, err = consultant.NewClient(conf)

	if err != nil {
		log.Fatalf("Unable to establish a consul client on the host %s", agentIp)
	}

	log.Printf("Established consul connection to %s", agentIp)
	flag.DurationVar(&interval, "interval", 10, "How many minutes to run reconcile")
	flag.Parse()
}

func main() {

	ws := getWS()

	// Establish websocket connection
	conn, resp, err := c.Websocket(ws.String(), nil)
	if err != nil {
		log.Panic(err)
	}

	defer conn.Close()

	if resp.StatusCode != 101 {
		log.Printf("Bad status code: %d %s", resp.StatusCode, resp.Status)
	}

	log.Printf("Listening on websocket connection: %s", ws.String())

	sigChan := make(chan os.Signal, 1)
	errChan := make(chan error, 10)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		errChan <- processEvents(conn)
	}()

	go func() {
		errChan <- reconcile()
	}()

	select {
	case sig := <-sigChan:
		log.Printf("we got signal %s", sig)
	case err := <-errChan:
		if err != nil {
			log.Printf("we got error: %v", err)
			os.Exit(1)
		}
	}
	os.Exit(0)
}
