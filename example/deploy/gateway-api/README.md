## Gateway API

HAProxy Unified Gateway currently supports v1.6.3 version, fot time being only specific routes are implemented:

- gateway.networking.k8s.io/v1
  - kind: HTTPRoute

## How to deploy

```sh
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.6.3/standard-install.yaml
```
