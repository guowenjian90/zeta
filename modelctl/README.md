# Components
ModelCtl include two components

* ModelCtl server is a controler
  * collect node state from each ModelCtl agent
  * assign node to implement a inference app model instance
  * update inference app's service's endpoints using model instance state reported from each ModelCtl agent 
  * auto scale up/down nodes according to app request statistics received
* ModelCtl agent and Triton server is deployed together in one DaemonSet pod on each node, ModelCtl agent work as proxy of triton server
  * load/unload model instance into/from triton server according modelctl's requestl
  * proactively unload inactive model in local triton server
  * report node and model resource
  * collect model request statistics for auto scaling

# Communication
  ModelCtl server and agent using grpc protocol to communicate without relying on apiserver

# Deployment
* ModelCtl server is deployed as a controller

# TODO
* ModelCtl agent download model artifacts from remote model repository, e.g. s3
* ModelCtl agent proxy model request to triton server in pod, collect request statistics
* ModelCtl agent unload model instance proactively and gracefully 
* ModelCtl agent implement a real resource provider to collect GPU/CPU/Memory state
* ModelCtl server autoscale more model instances using model request statistics
* ModelCtl server autoscale more model instances using model request statistics
