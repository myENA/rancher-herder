package main

import (
	"log"
	"strings"
)

func getConsulServiceID(prefix string) string {
	for _, s := range consulServices {
		if strings.Contains(s.(string), prefix) {
			return s.(string)
		}
	}

	return ""
}

func deRegister(serviceID string) {

	err = consul.Agent().ServiceDeregister(serviceID)

	if err != nil {
		log.Printf("Error deregistering %s: %s", serviceID, err)
		return
	}

	log.Printf("Service %s deregistered", serviceID)
}
