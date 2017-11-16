package main

import (
	"log"
	"strings"
)

func getConsulServiceId(prefix string) string {
	for _, s := range consulServices {
		if strings.Contains(s.(string), prefix) {
			return s.(string)
		}
	}

	return ""
}

func deRegister(serviceId string) {

	err = consul.Agent().ServiceDeregister(serviceId)

	if err != nil {
		log.Printf("Error deregistering %s: %s", serviceId, err)
		return
	}

	log.Printf("Service %s deregistered", serviceId)
}
