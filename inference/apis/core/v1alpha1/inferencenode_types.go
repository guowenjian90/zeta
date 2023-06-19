package v1alpha1

//
// import (
// 	"fmt"
//
// 	v1 "k8s.io/api/core/v1"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"k8s.io/apimachinery/pkg/runtime/schema"
// )
//
// // EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// // NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// // Important: Run "make" to regenerate code after modifying this file
//
// type NodeStatus string
//
// const (
// 	NodeStatusPending NodeStatus = "PENDING"
// 	NodeStatusRunning NodeStatus = "RUNNING"
// 	NodeStatusStopped NodeStatus = "STOPPED"
// 	NodeStatusFailing NodeStatus = "FAILING"
// )
//
// // InferenceNodeSpec defines the desired state of InferenceNode
// type InferenceNodeSpec struct {
// 	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
// 	Resources v1.ResourceList `json:"resources,omitempty"`
// }
//
// // InferenceNodeStatus defines the observed state of InferenceNode
// type InferenceNodeStatus struct {
// 	NodeStatus           NodeStatus       `json:"nodeStatus,omitempty"`
// 	Message              string           `json:"message,omitempty"`
// 	Models               map[string]Model `json:"models,omitempty"`
// 	AllocatableResources v1.ResourceList  `json:"allocatableResources,omitempty"`
// }
//
// // +kubebuilder:object:root=true
// // +kubebuilder:subresource:status
// // +genclient
// type InferenceNode struct {
// 	metav1.TypeMeta   `json:",inline"`
// 	metav1.ObjectMeta `json:"metadata,omitempty"`
//
// 	Spec   InferenceNodeSpec   `json:"spec,omitempty"`
// 	Status InferenceNodeStatus `json:"status,omitempty"`
// }
//
// //+kubebuilder:object:root=true
//
// // InferenceNodeList contains a list of InferenceNode
// type InferenceNodeList struct {
// 	metav1.TypeMeta `json:",inline"`
// 	metav1.ListMeta `json:"metadata,omitempty"`
// 	Items           []InferenceNode `json:"items"`
// }
//
// func init() {
// 	// SchemeBuilder.Register(&InferenceNode{}, &InferenceNodeList{})
// }
//
// var InferenceNodeGrv = schema.GroupVersionResource{
// 	Group:    "centaurusai.io",
// 	Version:  "v1alpha1",
// 	Resource: "inferencenode",
// }
//
// var InferenceNodeKind = SchemeGroupVersion.WithKind("InferenceNode")
// var InferenceNodeGrvKey = fmt.Sprintf("/%s/%s", InferenceNodeGrv.Group, InferenceNodeGrv.Resource)
