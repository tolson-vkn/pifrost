# Example

Throw out a namespace:

```
kubectl apply -f 0-namespace.yaml
```

Deploy app:

```
kubectl apply -f deploy.yaml
```

## Pick a path

Either Ingress based or Service Load Balancer based.

See top level README for functions.

### Ingress

```
kubectl apply -f clusterip-service.yaml
```

If using `--ingress-auto`:

```
kubectl apply -f ingress.yaml
```

If not using `--ingress-auto`:

```
kubectl apply -f ingress-anno.yaml
```

### Load Balancer

```
kubectl apply -f lb-service.yaml
```
