# start local k8s kind cluster
in this README document folder
```
sh ./zeta-kind-local-cluster.sh
```

# setup port forward to access zeta gateway in k8s cluster
```
ENVOY_SERVICE=$(kubectl get svc -n envoy-gateway-system --selector=gateway.envoyproxy.io/owning-gateway-namespace=default,gateway.envoyproxy.io/owning-gateway-name=zeta-gateway -o jsonpath='{.items[0].metadata.name}')
kubectl -n envoy-gateway-system port-forward service/${ENVOY_SERVICE} 8888:80 &
```

# test
## list models
```
curl -X POST --verbose --header "Host: localhost" http://localhost:8000/v2/repository/index
```

## model unload/load
```
curl -X POST --verbose --header "Host: localhost" http://localhost:8000/v2/repository/models/<model-name>/load
curl -X POST --verbose --header "Host: localhost" http://localhost:8000/v2/repository/models/<model-name>/unload
```

 # infer
```
curl -X POST --verbose --header "Host: localhost" http://localhost:8000/v2/models/<model-name>/infer -d {put real request document here}
```

better use triton infer client
```
    # Setting up client
    client = httpclient.InferenceServerClient(url="localhost:8000")

    # Read image and create input object
    # raw_image = cv2.imread("./img1.jpg")
    import sys
    raw_image = cv2.imread(sys.argv[1])
    preprocessed_image = detection_preprocessing(raw_image)

    detection_input = httpclient.InferInput(
        "input_images:0", preprocessed_image.shape, datatype="FP32"
    )
    detection_input.set_data_from_numpy(preprocessed_image, binary_data=True)
    # print(detection_input)

    # Query the server
    detection_response = client.infer(
        model_name="text_detection",
        inputs=[detection_input],
        headers={'Host': 'localhost'}
    )
    print(detection_response.get_response())
```
