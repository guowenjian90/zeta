This package includs two parts 
InferenceApp Resource types and Inference Controller

# InferenceApp 
Inference App represent a model configuration, including images, scaling policy.
Resource type is compiled to generate
* CRD configuration
* Clientset
* Protobuf

# InferenceApp Controller
InferenceApp controller use sigs.k8s.io/gateway-api to create a gateway for each infereance app, and provide each app unique url to access
currently, Enovy gateway implemenation is used in zeta. For each app, Controller will

* Create a gateway
* Create a http route
* Create a interal service to point to all model instances created for this app, service endpoints is maintained by ModelCtl server

# TODO
* Share gateway between inferance app
 * global shared one, shared domain name
 * user namespace shared one, user specific domain name
