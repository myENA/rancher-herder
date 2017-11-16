# Rancher service registrator for consul (WIP)
This service is compatible with Rancher 2.0, and more specifically for embeded kubernetes/cattle orchestration.

This service is opinionated on how it detects and registers services. It pays special attention to the kubernetes label `annotation.io.kubernetes.container.ports`
and uses it to detect exposed ports. In general if it detects the label it will register the service in consul, however, 
it will not register a health check. To register a health check some labels must be applied to the service; specifically `herder.service.check.http.path` ` herder.service.check.port`.

If the `-strict` flag is passed then the `herder.service.enable` label must be set on the service or else the service will be ignored. It is also important to
not that this service registers containers in the environment (k8s namespace) that is deployed in. To run the service globally you will need to supply the container
with admin api keys by setting the ENV variables `CATTLE_URL` `CATTLE_SECRET_KEY` `CATTLE_ACCESS_KEY` and deploy with the cluster level core-services.  

#### CLI Flags
```bash
Usage of ./cattle-herder:
  -consulDc string
        Consul Datacenter
  -consulIp string
        Set the consul ip for the services to be registered to. If blank it tries to connect to the host external IP
        If this flag is not passed the rancher host is detected and consul attempts to register t the agent on the public IP of the host
  -debug
        Enable debug logs
  -interval duration
        How often to run reconcile calculated by value * time.Minute (default 10ns)
  -strict
        Enable this flag to enforce the herder.service.enable label

```

# Compose Labels
Herder compose labels:
```yaml
## Rancher labels for the service account
## http://rancher.com/docs/rancher/v1.6/en/rancher-services/service-accounts/
 io.rancher.container.create_agent: true
 io.rancher.container.agent.role: environmentAdmin	
 ```
 
Service labels:
 ```yaml
 ## Herder specific labels
 ## All labels are strings because that is how they are depicted in the WS event Data
 herder.service.enable:               string  # If strict flag is passed this must be set to true to register the service
 herder.service.name:                 string  # The service name to display in the services view in consul
 herder.service.check.port:           string  # The port to register for the service check (NOT ASSUMED)
 herder.service.check.http.schema:    string  # HTTP or HTTPS
 herder.service.check.http.path:      string  # The health check path "/health"
 herder.service.check.tcp:            string  # True|False gets parsed to a bool for the service definition
 herder.service.check.interval:       string  # Check interval
 herder.service.tags:                 string  # A string with comma separated tags ex. "Tag1,Tag2"
```
