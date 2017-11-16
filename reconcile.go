package main

import (
	"encoding/json"
	"fmt"
	set "github.com/deckarep/golang-set"
	"github.com/mitchellh/mapstructure"
	"log"
	"strings"
	"time"
)

func getConsulServices() error {
	consulServices = nil

	services, err := consul.Agent().Services()

	if err != nil {
		log.Printf("Failed to get services from consul: %s", err)
		return err
	}

	for k, _ := range services {
		consulServices = append(consulServices, strings.ToLower(k))
	}
	if debug {
		log.Printf("Found Consul Services: %s", consulServices)
	}
	return nil
}

func getRancherContainers() error {

	rancherServices = nil
	containers, err := c.Container.List(nil)
	if err != nil {
		log.Printf("Failed to get Rancher containers: %s", err)
		return err
	}

	for _, c := range containers.Data {
		for l, v := range c.Labels {
			if l == "annotation.io.kubernetes.container.ports" {
				ports := make([]*ContainerPorts, 0)
				err := json.Unmarshal([]byte(v), &ports)

				if err != nil {
					log.Print(err)
				}

				for _, p := range ports {
					svcName := strings.ToLower(fmt.Sprintf("%s:%s:%s:%d:%s", c.HostId, c.Name, c.Id,
						p.ContainerPort, p.Protocol))
					rancherServices = append(rancherServices, svcName)
				}
			}
		}
	}

	if debug {
		log.Printf("Rancher Services: %+v", rancherServices)
	}
	return nil
}

func diffServices() []interface{} {
	rancherSet := set.NewSetFromSlice(rancherServices)
	consulSet := set.NewSetFromSlice(consulServices)

	diff := rancherSet.Difference(consulSet).ToSlice()

	if debug {
		log.Printf("Services Diff: %+v", diff)
	}
	return diff
}

func reconcile() error {

	for {
		err = getRancherContainers()
		if err != nil {
			return err
		}

		err2 := getConsulServices()
		if err2 != nil {
			return err2
		}

		// Get all of the stopped containers from Rancher and check if they are registered in Consul
		containers, err := c.Container.List(nil)

		if err != nil {
			log.Printf("Error getting container list for reconcile: %s", err)
			return nil
		}

		// Get all of the stopped containers
		var stopped []string
		for _, state := range containers.Data {
			if state.State == "stopped" {
				svcPrefix := fmt.Sprintf("%s:%s:%s", state.HostId, state.Name, state.Id)
				serviceId := getConsulServiceId(svcPrefix)
				// If the container was registered in consul serviceId should not be blank
				if serviceId != "" {
					stopped = append(stopped, serviceId)
				}
			}
		}

		// Determine which containers are missing from Consul
		var missing []*ContainerData
		for _, c := range diffServices() {
			split := strings.Split(c.(string), ":")
			if len(split) != 5 {
				continue
			}
			missing = append(missing, buildSvcData(split[2], c.(string)))
		}

		for _, m := range missing {
			if m.isValid() {
				log.Printf("Reconciling %s adding to consul", m.Resource.Name)
				registerSvc(m)
			}
		}

		for _, s := range stopped {
			log.Printf("Reconciling stopped service %s", s)
			deRegister(s)
		}

		time.Sleep(time.Minute * interval)
	}
}

func buildSvcData(containerId string, containerName string) *ContainerData {

	if debug {
		log.Printf("Building ContainerData for %s", containerName)
	}

	container, err := c.Container.ById(containerId)

	if err != nil || container == nil {
		log.Print("Container not Found")
		return nil
	}

	dataMap := &ContainerData{}

	if err != nil {
		log.Print(err)
	}

	err = mapstructure.Decode(container.Labels, &dataMap.Resource.Labels)
	if err != nil {
		log.Print(err)
	}

	dataMap.Resource.ID = container.Id
	dataMap.Resource.HostId = container.HostId
	dataMap.Resource.Name = containerName
	dataMap.Resource.PrimaryIPAddress = container.PrimaryIpAddress
	dataMap.Resource.State = container.State

	return dataMap

}
