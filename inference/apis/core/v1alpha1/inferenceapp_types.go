package v1alpha1

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// Important: Run "make" to regenerate code after modifying this file

type AppStatus string

const (
	AppStatusPending   AppStatus = "PENDING"
	AppStatusRunning   AppStatus = "RUNNING"
	AppStatusStopped   AppStatus = "STOPPED"
	AppStatusSucceeded AppStatus = "SUCCEEDED"
	AppStatusFailed    AppStatus = "FAILED"
)

// AppDeploymentStatus indicates InferenceApp status lifecycle management
type AppDeploymentStatus string

const (
	AppDeploymentStatusInitializing               AppDeploymentStatus = "Initializing"
	AppDeploymentStatusFailedToGetOrCreateService AppDeploymentStatus = "FailedToGetOrCreateService"
	AppDeploymentStatusWaitForDashboard           AppDeploymentStatus = "WaitForDashboard"
	AppDeploymentStatusFailedAppDeploy            AppDeploymentStatus = "FailedAppDeploy"
	AppDeploymentStatusRunning                    AppDeploymentStatus = "Running"
	AppDeploymentStatusFailedToGetAppStatus       AppDeploymentStatus = "FailedToGetAppStatus"
	AppDeploymentStatusComplete                   AppDeploymentStatus = "Complete"
)

// InferenceAppSpec defines the desired state of InferenceApp
type InferenceAppSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Model is versioned
	Model         Model         `json:"model,omitempty" protobuf:"bytes,1,opt,name=model"`
	ScalingPolicy ScalingPolicy `json:"scalingPolicy,omitempty" protobuf:"bytes,2,opt,name=scalingPolicy"`
}

type Runtime string

const (
	PytorchRuntime    Runtime = "pytorch"
	TensorflowRuntime Runtime = "tensorflow"
	ONNXRuntime       Runtime = "onnxruntime"
)

type Model struct {
	// where to download this model
	Url     string  `json:"url,omitempty" protobuf:"bytes,1,opt,name=url"`
	Runtime Runtime `json:"runtime,omitempty" protobuf:"bytes,2,opt,name=runtime,casttype=Runtime"`
	// Metadata is configs to initilize this model
	Metadata map[string]string `json:"metadata,omitempty" protobuf:"bytes,3,rep,name=metadata"`
	Resource v1.ResourceList   `json:"resource,omitempty" protobuf:"bytes,4,rep,name=resource,casttype=k8s.io/api/core/v1.ResourceList,castkey=k8s.io/api/core/v1.ResourceName"`
}

type ScalingPolicy struct {
	MinimumInstance uint32 `json:"minimumInstance,omitempty" protobuf:"varint,1,opt,name=minimumInstance"`
	MaximumInstance uint32 `json:"maximumInstance,omitempty" protobuf:"varint,2,opt,name=maximumInstance"`
	Burst           uint32 `json:"burst,omitempty" protobuf:"varint,3,opt,name=burst"`
}

// InferenceAppStatus defines the observed state of InferenceApp
type InferenceAppStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	EntrypointURL       string              `json:"entrypointURL,omitempty" protobuf:"bytes,1,opt,name=entrypointURL"`
	DashboardURL        string              `json:"dashboardURL,omitempty" protobuf:"bytes,2,opt,name=dashboardURL"`
	AppStatus           AppStatus           `json:"appStatus,omitempty" protobuf:"bytes,3,opt,name=appStatus,casttype=AppStatus"`
	AppDeploymentStatus AppDeploymentStatus `json:"appDeploymentStatus,omitempty" protobuf:"bytes,4,opt,name=appDeploymentStatus,casttype=AppDeploymentStatus"`
	Message             string              `json:"message,omitempty" protobuf:"bytes,5,opt,name=message"`
	NumOfInstance       uint32              `json:"numOfInstance,omitempty" protobuf:"varint,6,opt,name=numOfInstance"`
	ObservedGeneration  int64               `json:"observedGeneration,omitempty" protobuf:"varint,7,opt,name=observedGeneration"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +genclient
type InferenceApp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   InferenceAppSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status InferenceAppStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

//+kubebuilder:object:root=true

// InferenceAppList contains a list of InferenceApp
type InferenceAppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []InferenceApp `json:"items" protobuf:"bytes,2,rep,name=items"`
}

func init() {
	SchemeBuilder.Register(&InferenceApp{}, &InferenceAppList{})
}

var ApplicationGrv = schema.GroupVersionResource{
	Group:    "centaurusai.io",
	Version:  "v1alpha1",
	Resource: "inferenceapp",
}

var ApplicationKind = SchemeGroupVersion.WithKind("InferenceApp")
var ApplicationGrvKey = fmt.Sprintf("/%s/%s", ApplicationGrv.Group, ApplicationGrv.Resource)
