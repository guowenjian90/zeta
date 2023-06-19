SCRIPT_PATH=$( dirname "$0" )

$SCRIPT_PATH/kind-with-gpu create cluster --config $SCRIPT_PATH/zeta-kind-cluster.yaml

helm install --wait --generate-name -n gpu-operator --create-namespace nvidia/gpu-operator --set driver.enabled=false

kubectl apply -f $SCRIPT_PATH/envoy-gateway-crd.yaml

kubectl apply -f $SCRIPT_PATH/../../inference/config/crd/bases/centaurusai.io_inferenceapps.yaml

kubectl apply -f $SCRIPT_PATH/zeta-triton-daemonset.yaml

echo 'cluster is created successfully, wait for triton server created, follow test section in README'
