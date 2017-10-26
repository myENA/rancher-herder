# Rancher service registrator for consul (WIP)
Still very mush in the POC stage.
# Purpose

With Rancher v2.0 there were issues with registrator not being able to properly view all container metadata from the
docker.sock. Instead of forking registrator I decided to write a similar service that talked directly to the Rancher
web socket.

Registering a service from Rancher will work slightly differently than registrator. In this app it is the initial intent to use labels to configure the
service instead of environment variables.

Initial thoughts for labels:

```yaml
## Rancher labels for the service account
## http://rancher.com/docs/rancher/v1.6/en/rancher-services/service-accounts/
 io.rancher.container.create_agent:   true
 
 ## Herder specific labels
 ## All labels are strings because that is how they are depicted in the WS event Data
 herder.include:                      string
 herder.service.name:                 string
 herder.service.check.http:           string
 herder.service.port:                 string
 herder.service.check.http.schema:    string
 herder.service.check.http.path:      string
 herder.service.check.tcp:            string
 herder.service.check.interval:       string 
 herder.service.check.timeout:        string 
 herder.service.check.script:         string
 herder.service.check.initial_status: string
 herder.service.check.ttl:            string
 herder.service.tags:                 string // A string with comma seperated tags
```

However, if a port is Exposed or mapped the application will register the service with consul unless the label 
`herder.include` is set to false.

# Run localy

#### Build

`docker build -t cattle-herder .`

#### Run

Running outside of Rancher
```
docker run -d --name herder \
  -e CATTLE_URL=YOUR_CATTLE_URL \
  -e CATTLE_ACCESS_KEY=YOUR_ACCESS_KEY \
  -e CATTLE_SECRET_KEY=YOUR_SECRET_KEY \
  cattle-herder
```

Running from rancher or through rancher cli/ui you MUST add the label: `io.rancher.container.create_agent=true`

Setting that label will create a service account api keys and set the CATTLE env variables.
```yaml
version: "2"

services:
  herder:
    image: dahendel/cattle-herder
    labels:
      io.rancher.container.create_agent: true
      REST_OF_LABELS 
```

#TODO
- Parse Expose and Ports fields in the json payload
- Detect agent host and only register containers on the host
- Create Consul client for service registration
- Build Service Registration functions
- Register the service
- Remove the service