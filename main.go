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

// Get the rancher api data from Environment variables
var cattleURL = os.Getenv("CATTLE_URL")
var cattleAccessKey = os.Getenv("CATTLE_ACCESS_KEY")
var cattleSecretKey = os.Getenv("CATTLE_SECRET_KEY")

// Used to determine which host we are on
const metadataURL = "http://rancher-metadata/latest"

// Used to make sure we are only capturing events locally
var agentIP string
var hostUUID string
var hostID string

// Used for diffing services and reconciliation
var consulServices []interface{}
var rancherServices []interface{}

// Global flag variables
var interval time.Duration
var debug bool
var strict bool
var consulIP string
var consulDC string

// Set the rancher client
var c = &client.RancherClient{}
var err error

//set global consul client
var consul *consultant.Client

// Event defines a websocket message
type Event struct {
	Name         string          `json:"name"`
	ResourceType string          `json:"resourceType"`
	ResourceID   string          `json:"resourceId"`
	Data         json.RawMessage `json:"data"`
}

// ContainerPorts is used to unmarshal the json in the k8s ports label
type ContainerPorts struct {
	HostPort      int    `json:"hostPort"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol"`
}

// ContainerData is the data extracted to build the service registration from event.Data json
type ContainerData struct {
	Resource struct {
		ID               string `json:"id,omitempty"`
		HostID           string `json:"hostId,omitempty"`
		Name             string `json:"name,omitempty"`
		PrimaryIPAddress string `json:"primaryIpAddress,omitempty"`
		State            string `json:"state,omitempty"`
		Labels           struct {
			ContainerName                string `json:"io.rancher.container.name,omitempty" mapstructure:"io.rancher.container.name"`
			PortsString                  string `json:"annotation.io.kubernetes.container.ports,omitempty" mapstructure:"annotation.io.kubernetes.container.ports"`
			Ports                        []*ContainerPorts
			HerderServiceEnable          string `json:"herder.service.enable,omitempty" mapstructure:"herder.service.enable"`
			HerderServiceCheckPath       string `json:"herder.service.check.http.path,omitempty" mapstructure:"herder.service.check.http.path"`
			HerderServiceCheckPort       string `json:"herder.service.check.port,omitempty" mapstructure:"herder.service.check.http.port"`
			HerderServiceCheckHTTPSchema string `json:"herder.service.check.http.schema,omitempty" mapstructure:"herder.service.check.http.schema"`
			HerderServiceCheckInterval   string `json:"herder.service.check.interval,omitempty" mapstructure:"herder.service.check.interval"`
			HerderServiceCheckTCP        string `json:"herder.service.check.tcp,omitempty" mapstructure:"herder.service.check.tcp"`
			HerderServiceName            string `json:"herder.service.name,omitempty" mapstructure:"herder.service.name"`
			HerderServiceTags            string `json:"herder.service.tags,omitempty" mapstructure:"herder.service.tags"`
		} `json:"labels,omitempty"`
	} `json:"resource,omitempty"`
}

// Establish the rancher and consul clients as well as set host information for filtering events on the host the agent
// is running on
func init() {
	// Parse the flags and set values
	flag.StringVar(&consulIP, "consulIP", "", "Set the consul ip for the services to be registered to. If blank it tries to connect to the host external IP\n\t"+
		"If this flag is not passed the rancher host is detected and consul attempts to register t the agent on the public IP of the host")
	flag.StringVar(&consulDC, "consulDC", "", "Consul Datacenter")
	flag.BoolVar(&debug, "debug", false, "Enable debug logs")
	flag.BoolVar(&strict, "strict", false, "Enable this flag to enforce the herder.service.enable label")
	flag.DurationVar(&interval, "interval", 5*time.Minute, "How often to run reconcile ex. 5m")
	flag.Parse()

	// Configure cattle client
	config := &client.ClientOpts{
		Url:       cattleURL,
		AccessKey: cattleAccessKey,
		SecretKey: cattleSecretKey,
	}

	c, err = client.NewRancherClient(config)
	if err != nil {
		log.Panic(err)
	}

	log.Print("Client Connection Established...")

	// Get which rancher host we are on and set globa variables
	m := metadata.NewClient(metadataURL)

	self, err := m.GetSelfHost()

	if err != nil {
		log.Printf("Could not get host meta-data")
	}

	agentIP = self.AgentIP
	hostUUID = self.UUID

	hosts, err := c.Host.List(nil)

	if err != nil {
		log.Print("Failed to get list of hosts")
	}

	// Set the local rancher host Id by looping through the hosts list until the host UUid's match
	for _, h := range hosts.Data {
		if h.Uuid == hostUUID {
			hostID = h.Id
		}
	}

	log.Printf("Monitoring events on Host %s with HostID %s", agentIP, hostID)

	// Configure consul client
	var conf *api.Config

	// Set consul IP for consul client
	if consulIP != "" {
		conf = &api.Config{Address: consulIP}
	} else {
		conf = &api.Config{Address: agentIP + ":8500"}
	}

	// Set DC for consul client if datacenter is passed
	if consulDC != "" {
		conf.Datacenter = consulDC
	}

	consul, err = consultant.NewClient(conf)

	if err != nil {
		log.Fatalf("Unable to establish a consul client on the host %s", conf.Address)
	}

	log.Printf("Established consul connection to %s", conf.Address)

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

	// Listen to events and process them, listening for an error
	go func() {
		errChan <- processEvents(conn)
	}()

	// Run reconciliation every x minutes
	go func() {
		errChan <- reconcile()
	}()

	select {
	case sig := <-sigChan:
		log.Printf("Recieved and interrupt signal : %v", sig)
	case err := <-errChan:
		if err != nil {
			log.Printf("Error recieved: %v", err)
			os.Exit(1)
		}
	}
	os.Exit(0)
}
