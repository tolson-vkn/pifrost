# pifrost

An external DNS covering service and ingress objects for the upstream DNS; [pi-hole](https://pi-hole.net/).

_It has come to my attention that [external-dns](https://github.com/kubernetes-sigs/external-dns) now supports pi-hole.
It did not when I wrote this, so I wrote this... you may wnat to go check that out instead._

## Demo

[![asciicast](https://asciinema.org/a/471236.svg)](https://asciinema.org/a/471236)

## Usage

```
Usage:
  pifrost server [flags]

Flags:
  -h, --help                        help for server
      --ingress-auto                do not require annotation on ingress resources (default: false)
      --ingress-externalip string   force use of provided external ip (default: use ingress external ip)
      --insecure                    communicate over http:// (default: https://)
      --kubeconfig string           absolute path to kubeconfig (default: in cluster config)
      --pihole-host string          hostname or IP of pihole instance
      --pihole-token string         API token for pihole

Global Flags:
      --log-level string   log level (debug, info, warn, error, fatal, panic (default "warning")
```

Further Flag Flags:

#### `--ingress-auto`

Auto discover the ingress objects in the cluster and create DNS records in pi-hole. This is the default
behavior of externaldns. All host records regardless of the domain will be sent to pi-hole. If you do
not use this flag you then must put the annotation `pifrost.tolson.io/ingress: "true"` if you want it
picked up.

#### `--ingress-externalip string`

Some installs, partuclarly homelab-ed kubernetes, may display the ingress controller load balancer as
having the node IP as the loadbalancer IP. This can be fixed, but if you prefer to specify the load
balancer IP use this flag.

#### `--insecure`

For users not using HTTPS on pi-hole, this flag must be supplied.

#### `--pihole-host string`

Hostname or IP address of pi-hole instance.

#### `--pihole-token string`

pi-hole api token, can be found at: `<pi-hole address>/admin/settings.php?tab=api`

API Settings -> Show API Token

#### `--kubeconfig string`

Path to kubeconfig, not used outside of development.

## Kubernetes Deployment

See `deployment/` for example deployment

### Annotations

#### Service Object

```
pifrost.tolson.io/domain: foo.tolson.io
```

The annotation applied to a service object. The loadbalancer IP and annotation domain are sent to pi-hole.

#### Ingress Object

```
pifrost.tolson.io/ingress: "true"
```

Only required if `--ingress-auto` is not supplied. For an ingress object to be added to pi-hole it must have
this annotation.

### Secrets

As seen in the `deployment/` directory, but called out here. Pass the `--pihole-token` with:

```
[... snip ...]
containers:
- args:
  --pihole-token=$(PIHOLE_TOKEN)
[... snip ...]
env:
- name: PIHOLE_TOKEN
  valueFrom:
    secretKeyRef:
      name: pifrost
      key: pihole_token
[... snip ...]
```

### Other

See `examples/` for a test deployment

See `api-responses.md` for pihole dns API.

## Testing

This is not exhaustive but things that should be tested in addition to go tests.

``` bash
# docker run pihole or point  at one...
# ---
cd examples/
kubectl apply -f lb-service.yaml
# delete annotation from lb-service.yaml
sed -i '/pifrost.tolson.io\/domain: "env-echgo-lb.tolson.io"/d' lb-service.yaml
kubectl apply -f lb-service.yaml
# put it back to normal it should pick it back up.
git checkout -- lb-service
kubectl apply -f lb-service.yaml
# add a new random annotation back to the lb-service.yaml did it change the record?
sed -i 's#pifrost.tolson.io/domain: "env-echgo-lb.tolson.io"#pifrost.tolson.io/domain: "env-echgo-lb-two.tolson.io"#' lb-service.yaml
kubectl apply -f lb-service.yaml
# ---
# rename the ingress
kubectl apply -f ingress.yaml
sed -i 's#env-echgo.example.com#env-echgo-two.example.com#' ingress.yaml
kubectl apply -f ingress.yaml
# remove it
git checkout -- ingress.yaml
kubectl delete -f ingress.yaml
```
