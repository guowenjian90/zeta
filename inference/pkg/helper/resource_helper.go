/*
Copyright 2022 The CentaurusAI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helper

import (
	"context"
	"fmt"
	"time"

	// "google.golang.org/grpc/profiling/service"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/controller-runtime/pkg/client"

	gw "sigs.k8s.io/gateway-api/apis/v1beta1"

	zetav1 "CentaurusAI/zeta/inference/apis/core/v1alpha1"
)

const (
	MaxServiceEndpoints               = 200
	GatewayKind                       = "Gateway"
	ServiceKind                       = "Service"
	HttpRouteKind                     = "HttpRoute"
	DefaultDeletionGracePeriodSeconds = int64(600)
	DefaultGatewayClass               = "eg"
)

func currentTime() *metav1.Time {
	return &metav1.Time{
		Time: time.Now(),
	}
}

func GetInferenceApp(cli client.Client, ctx context.Context, namespaced_name types.NamespacedName) (*zetav1.InferenceApp, error) {
	app := &zetav1.InferenceApp{}
	if err := cli.Get(ctx, namespaced_name, app); err != nil {
		return nil, err
	}
	return app, nil
}

func GetInferenceAppService(cli client.Client, ctx context.Context, namespaced_name types.NamespacedName) (*corev1.Service, error) {
	svc := &corev1.Service{}
	if err := cli.Get(ctx, namespaced_name, svc); err != nil {
		return nil, err
	}
	return svc, nil
}

func GetInferenceAppServiceEndpoint(cli client.Client, ctx context.Context, namespaced_name types.NamespacedName) (*corev1.Endpoints, error) {
	endpoint := &corev1.Endpoints{}
	if err := cli.Get(ctx, namespaced_name, endpoint); err != nil {
		return nil, err
	}
	return endpoint, nil
}

func GetInferenceAppGateway(cli client.Client, ctx context.Context, namespaced_name types.NamespacedName) (*gw.Gateway, error) {
	gateway := &gw.Gateway{}
	if err := cli.Get(ctx, namespaced_name, gateway); err != nil {
		return nil, err
	}
	return gateway, nil
}

func GetInferenceAppHttpRoute(cli client.Client, ctx context.Context, namespaced_name types.NamespacedName) (*gw.HTTPRoute, error) {
	route := &gw.HTTPRoute{}
	if err := cli.Get(ctx, namespaced_name, route); err != nil {
		return nil, err
	}
	return route, nil
}

func GetPod(cli client.Client, ctx context.Context, namespaced_name types.NamespacedName) (*corev1.Pod, error) {
	pod := &corev1.Pod{}
	if err := cli.Get(ctx, namespaced_name, pod); err != nil {
		return nil, err
	}
	return pod, nil
}

func CreateResource(cli client.Client, ctx context.Context, obj client.Object) error {
	err := cli.Create(ctx, obj, &client.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func UpdateResource(cli client.Client, ctx context.Context, obj client.Object) error {
	err := cli.Update(ctx, obj, &client.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func DeleteResource(cli client.Client, ctx context.Context, obj client.Object) error {
	err := cli.Delete(ctx, obj, &client.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

func GetDomainName(app *zetav1.InferenceApp) string {
	domain := fmt.Sprintf("%s.centaurus.ai", app.Name)
	return domain
}

func BuildGatewayForInferenceApp(app *zetav1.InferenceApp) *gw.Gateway {
	block_owner_deletion := true
	ps := DefaultDeletionGracePeriodSeconds
	gateway := &gw.Gateway{
		TypeMeta: metav1.TypeMeta{
			Kind:       GatewayKind,
			APIVersion: gw.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:                       app.Name,
			Namespace:                  app.Namespace,
			Generation:                 0,
			CreationTimestamp:          *currentTime(),
			DeletionGracePeriodSeconds: &ps,
			Labels:                     map[string]string{"InferenceApp": app.Name, "domain": GetDomainName(app)},
			Annotations:                map[string]string{},
			OwnerReferences: []metav1.OwnerReference{metav1.OwnerReference{
				APIVersion:         zetav1.ApplicationGrv.GroupVersion().String(),
				Kind:               zetav1.ApplicationKind.Kind,
				Name:               app.Name,
				UID:                app.GetUID(),
				BlockOwnerDeletion: &block_owner_deletion,
			}},
			Finalizers: []string{},
		},
		Spec: gw.GatewaySpec{
			GatewayClassName: DefaultGatewayClass,
			Listeners: []gw.Listener{gw.Listener{
				Name:     "http",
				Port:     8000,
				Protocol: "HTTP",
				// Hostname: &hostname,
				// TLS:           &gw.GatewayTLSConfig{},
				// AllowedRoutes: &gw.AllowedRoutes{},
			}},
			// Addresses: []gw.GatewayAddress{gw.GatewayAddress{
			//  Type:  &"",
			//  Value: "",
			// }},
		},
	}

	return gateway
}

func BuildHttpRouteForInferenceApp(app *zetav1.InferenceApp, gateway *gw.Gateway, service *corev1.Service) *gw.HTTPRoute {
	block_owner_deletion := true
	ps := DefaultDeletionGracePeriodSeconds
	match_type := gw.PathMatchPathPrefix
	// TODO, use friendly name
	// path := fmt.Sprintf("/v2/models/%s/infer", string(app.Name))
	path := fmt.Sprintf("/v2/models/%s/infer", string(app.UID))
	port := gw.PortNumber(service.Spec.Ports[0].Port)
	backend_kind := gw.Kind(ServiceKind)
	route := &gw.HTTPRoute{
		TypeMeta: metav1.TypeMeta{
			Kind:       HttpRouteKind,
			APIVersion: gw.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:                       app.Name,
			Namespace:                  app.Namespace,
			Generation:                 0,
			CreationTimestamp:          *currentTime(),
			DeletionGracePeriodSeconds: &ps,
			Labels:                     map[string]string{"InferenceApp": app.Name},
			Annotations:                map[string]string{},
			OwnerReferences: []metav1.OwnerReference{metav1.OwnerReference{
				APIVersion:         zetav1.GroupVersion.String(),
				Kind:               zetav1.ApplicationKind.Kind,
				Name:               app.Name,
				UID:                app.GetUID(),
				BlockOwnerDeletion: &block_owner_deletion,
			}},
			Finalizers: []string{},
		},
		Spec: gw.HTTPRouteSpec{
			CommonRouteSpec: gw.CommonRouteSpec{
				ParentRefs: []gw.ParentReference{
					gw.ParentReference{
						Name:        gw.ObjectName(gateway.GetName()),
						SectionName: &gateway.Spec.Listeners[0].Name,
						// Port:        &0,
					},
				},
			},
			Hostnames: []gw.Hostname{gw.Hostname("localhost"), gw.Hostname(GetDomainName(app))},
			Rules: []gw.HTTPRouteRule{gw.HTTPRouteRule{
				Matches: []gw.HTTPRouteMatch{
					gw.HTTPRouteMatch{
						Path: &gw.HTTPPathMatch{
							Type:  &match_type,
							Value: &path,
						},
						// Headers:     []gw.HTTPHeaderMatch{},
						// QueryParams: []gw.HTTPQueryParamMatch{},
						// Method:      &"",
					}},
				BackendRefs: []gw.HTTPBackendRef{gw.HTTPBackendRef{
					BackendRef: gw.BackendRef{
						BackendObjectReference: gw.BackendObjectReference{
							Kind: &backend_kind,
							Name: gw.ObjectName(service.GetName()),
							Port: &port,
						},
						Weight: new(int32),
					},
				}},
				// Filters:     []gw.HTTPRouteFilter{},
			}},
		},
	}

	return route
}

func BuildServiceForInferenceApp(app *zetav1.InferenceApp) *corev1.Service {
	blockOwnerDeletion := true
	ps := DefaultDeletionGracePeriodSeconds
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       ServiceKind,
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:                       app.Name,
			Namespace:                  app.Namespace,
			Generation:                 0,
			CreationTimestamp:          *currentTime(),
			DeletionGracePeriodSeconds: &ps,
			Labels:                     map[string]string{"InferenceApp": app.Name},
			Annotations:                map[string]string{},
			OwnerReferences: []metav1.OwnerReference{metav1.OwnerReference{
				APIVersion:         zetav1.ApplicationGrv.GroupVersion().String(),
				Kind:               zetav1.ApplicationKind.Kind,
				Name:               app.Name,
				UID:                app.GetUID(),
				BlockOwnerDeletion: &blockOwnerDeletion,
			}},
			Finalizers: []string{},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{corev1.ServicePort{
				Name:       "http",
				Protocol:   "TCP",
				Port:       8000,
				TargetPort: intstr.FromInt(8000),
				// AppProtocol: new(string),
				// NodePort: 0,
			}},
			Selector: map[string]string{},
			// ClusterIP:                "",
			// ClusterIPs:               []string{},
			// Type:                     "",
			// ExternalIPs:              []string{},
			// SessionAffinity:          "",
			// LoadBalancerIP:           "",
			// LoadBalancerSourceRanges: []string{},
			// ExternalName:             "",
			// ExternalTrafficPolicy:    "",
			// HealthCheckNodePort:      0,
			// PublishNotReadyAddresses: false,
			// SessionAffinityConfig:    &corev1.SessionAffinityConfig{},
			// IPFamilies:               []corev1.IPFamily{},
			// IPFamilyPolicy:                &"",
			// AllocateLoadBalancerNodePorts: new(bool),
			// LoadBalancerClass:             new(string),
			// InternalTrafficPolicy:         &"",
		},
	}

	return service
}

func BuildEndpointsForService(svc *corev1.Service) *corev1.Endpoints {
	blockOwnerDeletion := true
	ps := DefaultDeletionGracePeriodSeconds
	endpoints := &corev1.Endpoints{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Endpoints",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:                       svc.Name,
			Namespace:                  svc.Namespace,
			Generation:                 0,
			CreationTimestamp:          *currentTime(),
			DeletionGracePeriodSeconds: &ps,
			Labels:                     map[string]string{"InferenceApp": svc.Name},
			Annotations:                map[string]string{},
			OwnerReferences: []metav1.OwnerReference{metav1.OwnerReference{
				APIVersion:         corev1.SchemeGroupVersion.String(),
				Kind:               ServiceKind,
				Name:               svc.Name,
				UID:                svc.GetUID(),
				BlockOwnerDeletion: &blockOwnerDeletion,
			}},
			Finalizers: []string{},
		},
		Subsets: []corev1.EndpointSubset{},
	}

	return endpoints
}
