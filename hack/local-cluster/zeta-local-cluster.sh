INSTALL_PATH=$( dirname "$0" )

$INSTALL_PATH/kind-with-gpu create cluster --config $INSTALL_PATH/zeta-local-cluster.yaml

helm install --wait --generate-name -n gpu-operator --create-namespace nvidia/gpu-operator --set driver.enabled=false

kubectl apply -f $INSTALL_PATH/envoy-gateway-crd.yaml

kubectl apply -f $INSTALL_PATH/zeta-local-server.yaml

echo 'cluster is created successfully, follow test section in README'
