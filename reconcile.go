package main

import (
	set "github.com/deckarep/golang-set"
	"log"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"github.com/mitchellh/mapstructure"
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
	//log.Printf("Found Consul Services: %s", consulServices)
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

			//log.Printf("LOGGING CONTAINER: %s RANCHER LABELS: %+v", c.Name, c.Labels)

			if l == "annotation.io.kubernetes.container.ports" {
				ports := make([]*ContainerPorts, 0)
				err := json.Unmarshal([]byte(v), &ports)

				if err != nil {
					log.Print(err)
				}

				//log.Printf("CNAME: %s", cName)
				for _, p := range ports {
					svcName := strings.ToLower(fmt.Sprintf("%s:%s:%s:%d:%s", c.HostId, c.Name, c.Id,
						p.ContainerPort, p.Protocol))
					rancherServices = append(rancherServices, svcName)
				}
			}
		}
	}

	//log.Printf("Rancher Services: %+v", rancherServices)

	return nil
}

func diffServices() []interface{} {
	rancherSet := set.NewSetFromSlice(rancherServices)
	consulSet := set.NewSetFromSlice(consulServices)

	diff := rancherSet.Difference(consulSet).ToSlice()
	log.Printf("%+v", diff)
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

		missing := func() []*ContainerData {

			var data []*ContainerData

			for _, c := range diffServices() {
				split := strings.Split(c.(string), ":")
				if len(split) != 5 { continue }
				data = append(data, buildSvcData(split[2], c.(string)))
			}

			return data

		}

		for _, s := range missing(){
			if s.isValid() {
				log.Printf("Reconciling %s", s.Resource.Name)
				registerSvc(s)
			}
		}

		//log.Printf("DATA: %s", missing())
		time.Sleep(time.Second * interval)
	}
}

func buildSvcData(containerId string, containerName string) *ContainerData {

	//log.Printf("Building ContainerData for %s", containerName)
	container, err := c.Container.ById(containerId)

	if err != nil || container == nil{
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

	//log.Printf("ContainerLabel: %s", dataMap.Resource.Labels)

	return dataMap

}
