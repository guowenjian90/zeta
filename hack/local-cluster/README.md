# start local k8s kind cluster
```
sh ./zeta-local-cluster.sh
```

# create test application
```
kubectl apply -f ./text-inference-app.yaml
```

This file create two model server, each server will have a gateway,
## verify gateway and model are loaded
```
kubectl get gateway -A
kubectl get inferenceapp -A
```

# setup port forward to access zeta gateway in k8s cluster
```
ENVOY_SERVICE=$(kubectl get svc -n envoy-gateway-system --selector=gateway.envoyproxy.io/owning-gateway-namespace=zeta,gateway.envoyproxy.io/owning-gateway-name=text-detection -o jsonpath='{.items[0].metadata.name}')
kubectl -n envoy-gateway-system port-forward service/${ENVOY_SERVICE} 8000:8000 &

ENVOY_SERVICE=$(kubectl get svc -n envoy-gateway-system --selector=gateway.envoyproxy.io/owning-gateway-namespace=zeta,gateway.envoyproxy.io/owning-gateway-name=text-recognition -o jsonpath='{.items[0].metadata.name}')
kubectl -n envoy-gateway-system port-forward service/${ENVOY_SERVICE} 8001:8000 &
```

## make infer call
```
inference app url is using app uid to differentiate, get uid firstly
```
kubectl get inferenceapp  -n zeta 'text-detection' -o jsonpath='{.metadata.uid}'
```
curl -X POST --verbose --header "Host: localhost" http://localhost:8000/v2/models/<app.metadata.uid>/infer -d {put real request document here}
```

and you can try ./client.py also
```
 python client.py ./img1.jpg  
```
