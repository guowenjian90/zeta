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

package server

import (
	zetav1 "CentaurusAI/zeta/inference/apis/core/v1alpha1"
	"CentaurusAI/zeta/inference/pkg/helper"
	"CentaurusAI/zeta/modelctl/pkg/modelctl"
	"CentaurusAI/zeta/modelctl/pkg/types"

	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ktypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	DEFAULT_REQUEUE_DURATION = 1 * time.Second
)

type modelInstanceMap struct {
	mu       sync.RWMutex
	services map[ktypes.UID]map[string]*modelInstance
}

type modelInstance struct {
	ip     string
	port   int32
	state  types.ServingState
	nodeId string
}

var _ reconcile.Reconciler = (*endpointReconciler)(nil)

type endpointReconciler struct {
	client.Client
	log             logr.Logger
	nodeInfoChannel chan *types.NodeEvent
	nodeManager     modelctl.NodeManagerInterface
	modelPool       *modelInstanceMap
}

func (pool *modelInstanceMap) add(modelUid ktypes.UID, ep *modelInstance) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	if m, f := pool.services[modelUid]; f {
		m[ep.nodeId] = ep
	} else {
		pool.services[modelUid] = map[string]*modelInstance{ep.nodeId: ep}
	}
}

func (pool *modelInstanceMap) remove(modelUid ktypes.UID, node string) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	if m, f := pool.services[modelUid]; f {
		delete(m, node)
	}
}

func (pool *modelInstanceMap) removeNode(node string) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	for _, v := range pool.services {
		delete(v, node)
	}
}

func (pool *modelInstanceMap) removeAll(modelUid ktypes.UID) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	delete(pool.services, modelUid)
}

func (pool *modelInstanceMap) get(modelUid ktypes.UID) map[string]*modelInstance {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	if n, found := pool.services[modelUid]; found {
		a := make(map[string]*modelInstance, len(n))
		for k, v := range n {
			a[k] = v
		}
		return a
	}
	return nil
}

// 1/ listen to api server to load k8s service
// 2/ listen to node monitor, maintain model instance of app implemeted by node
// 3/ update service endpoint
func (sr *endpointReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	sr.log.Info("reconciling InferenceApp service", "name", request.NamespacedName)

	// Get InferenceApp instance
	// inferance app, service, endpoints is supposed to use same namespaced name
	var err error
	var app *zetav1.InferenceApp
	if app, err = helper.GetInferenceApp(sr.Client, ctx, request.NamespacedName); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// by default use app uid as model id and use it also in node as model name in inference server
	model := &types.Model{
		Uid:       app.UID,
		Name:      app.Name,
		Namespace: app.Namespace,
		Version:   fmt.Sprint(app.Generation),
		APIModel:  app.Spec.Model.DeepCopy(),
	}

	if _, err = sr.reconcileEndpoint(ctx, model, app); err != nil {
		sr.log.Error(err, "Failed to reconcile endpoints for app", "service", app.Name)
		return ctrl.Result{Requeue: true, RequeueAfter: DEFAULT_REQUEUE_DURATION}, err
	}

	sr.log.Info("Successfully reconcile service", "service", app.Name)

	return ctrl.Result{Requeue: false, RequeueAfter: 0}, nil
}

func (sr *endpointReconciler) reconcileEndpoint(ctx context.Context, model *types.Model, app *zetav1.InferenceApp) (*corev1.Endpoints, error) {
	var err error
	namespaceName := ktypes.NamespacedName{
		Namespace: app.Namespace,
		Name:      app.Name,
	}

	if app == nil || !app.ObjectMeta.DeletionTimestamp.IsZero() {
		sr.log.Info("Inference application service is being or already deleted, unload all model instances of this app")
		modelInstances := sr.modelPool.get(model.Uid)
		for _, v := range modelInstances {
			sr.nodeManager.UnloadModel(v.nodeId, model)
		}
		// TODO, delete app's k8s service endpoint???, app's k8s service will be deleted by app controller, maybe do it in app controller
	} else {
		modelInstances := sr.modelPool.get(model.Uid)
		if len(modelInstances) == 0 {
			// this app does not have any model instance, skip
			return nil, nil
		} else {
			var service *corev1.Service
			service, err = helper.GetInferenceAppService(sr.Client, ctx, namespaceName)
			if err != nil && !apierrors.IsNotFound(err) {
				return nil, err
			}
			if service == nil {
				// app service could not be created yet, do not delete endpoints unless we know application is deleted
				return nil, nil
			}

			// get current app service endpoint and reconcile it with model instances report from node
			svcEndpoint, err := helper.GetInferenceAppServiceEndpoint(sr.Client, ctx, namespaceName)
			if err != nil && !apierrors.IsNotFound(err) {
				sr.log.Error(err, "Failed to get endpoint, backoff and retry", "service", namespaceName)
				return nil, err
			}

			var updatedSvcEndpoint *corev1.Endpoints
			if svcEndpoint == nil {
				updatedSvcEndpoint = helper.BuildEndpointsForService(service)
			} else {
				updatedSvcEndpoint = svcEndpoint.DeepCopy()
			}

			// save old ready and not ready ips
			subsets := updatedSvcEndpoint.Subsets
			oldReadyIps := map[string]bool{}
			oldNotReadyIps := map[string]bool{}
			for _, v := range subsets {
				for _, v := range v.Addresses {
					oldReadyIps[v.IP] = true
				}
				for _, v := range v.NotReadyAddresses {
					oldNotReadyIps[v.IP] = true
				}
			}

			// get new ready and not ready ips from node reported endponts, and reconstruct readyAddress and notReadyAddress
			newReadyIps := map[string]bool{}
			newNotReadyIps := map[string]bool{}
			readyAddress := []corev1.EndpointAddress{}
			notReadyAddress := []corev1.EndpointAddress{}
			for _, v := range modelInstances {
				podIp := v.ip
				vstate := v.state
				if vstate == types.ServingStateReady {
					newReadyIps[podIp] = true
					readyAddress = append(readyAddress, corev1.EndpointAddress{
						IP: podIp,
						// Hostname:  "",
						// NodeName:  new(string),
						// TargetRef: &v1.ObjectReference{},
					})
				}
				if vstate == types.ServingStateCreated {
					newReadyIps[podIp] = true
					readyAddress = append(readyAddress, corev1.EndpointAddress{
						IP: podIp,
					})
				}
				if vstate == types.ServingStateUnloaded {
					newNotReadyIps[podIp] = true
					notReadyAddress = append(notReadyAddress, corev1.EndpointAddress{
						IP: podIp,
					})
				}
			}

			if reflect.DeepEqual(newReadyIps, oldReadyIps) && reflect.DeepEqual(newNotReadyIps, oldNotReadyIps) {
				// no change, return
				return svcEndpoint, nil
			} else {
				// update endpoint or create endpoint
				var port int32 = 0
				for _, v := range modelInstances {
					port = v.port
					break
				}
				if port == 0 {
					return nil, errors.New("port no should not be 0")
				}

				updatedSvcEndpoint.Subsets = []corev1.EndpointSubset{corev1.EndpointSubset{
					Addresses:         readyAddress,
					NotReadyAddresses: notReadyAddress,
					Ports: []corev1.EndpointPort{corev1.EndpointPort{
						Name:     "http",
						Port:     port,
						Protocol: "TCP",
					}},
				}}

				if svcEndpoint == nil {
					err := helper.CreateResource(sr.Client, ctx, updatedSvcEndpoint)
					if err != nil {
						return nil, err
					}
				} else {
					err := helper.UpdateResource(sr.Client, ctx, updatedSvcEndpoint)
					if err != nil {
						return nil, err
					}
				}
				return updatedSvcEndpoint, nil
			}
		}
	}

	return nil, nil
}

func (sr *endpointReconciler) handleNodeUpdate(nodeEvent *types.NodeEvent) error {
	sr.log.Info("Received node event, updating node pool", "event", nodeEvent)
	v1node := nodeEvent.Node
	eventType := nodeEvent.EventType
	nodeIdentifier := v1node.NodeIdentifer
	if eventType == types.Delete {
		sr.log.Info("Deleting a node", "node", nodeIdentifier)
		for _, v := range nodeEvent.Node.Node.GetModelStates() {
			sr.nodeManager.UnloadModel(nodeIdentifier, &types.Model{Uid: ktypes.UID(v.ModelIdentifier)})
		}
		sr.modelPool.removeNode(nodeIdentifier)
		return nil
	} else {
		models := nodeEvent.Node.Node.GetModelStates()
		for _, v := range models {
			instance := modelInstance{
				ip:     v1node.Node.NodeIp,
				port:   v1node.Node.Port,
				state:  types.FromModelGrpcState(v.State),
				nodeId: nodeIdentifier,
			}
			sr.modelPool.add(ktypes.UID(v.ModelIdentifier), &instance)
			if len(v.ModelNamespace) == 0 && len(v.ModelName) == 0 {
				// if reported state does not have namespace and name, consider its a dead model, and unload
				// TODO load app and double check and unload model if app is deleted
				// modelId := v.ModelIdentifier
				// modelEndpoints := sr.modelPool.get(ktypes.UID(modelId))
				sr.nodeManager.UnloadModel(nodeIdentifier, &types.Model{Uid: ktypes.UID(v.ModelIdentifier)})
			} else {
				sr.Reconcile(context.Background(), reconcile.Request{
					NamespacedName: ktypes.NamespacedName{
						Namespace: v.ModelNamespace,
						Name:      v.ModelName,
					},
				})
			}
		}
		return nil
	}
}

func (sr *endpointReconciler) Run(ctx context.Context) error {
	sr.log.Info("Starting ModelCtl service endpoint updater")
	sr.nodeManager.Suscribe("ModelEndpointReconciler", sr.nodeInfoChannel)
	sr.log.Info("ModelCtl model service endpoint updater started")

	for {
		select {
		case <-ctx.Done():
			sr.log.Info("ModelCtl Service Updater exit")
			return nil
		case update := <-sr.nodeInfoChannel:
			sr.handleNodeUpdate(update)
		}
	}
}

func NewModelEndpointReconciler(mgr manager.Manager, nodeMonitor modelctl.NodeManagerInterface) (*endpointReconciler, error) {
	sr := &endpointReconciler{
		Client:          mgr.GetClient(),
		log:             ctrl.Log.WithName("ModelCtl").WithName("SericeUpdater"),
		nodeInfoChannel: make(chan *types.NodeEvent, 100),
		nodeManager:     nodeMonitor,
		modelPool: &modelInstanceMap{
			mu:       sync.RWMutex{},
			services: map[ktypes.UID]map[string]*modelInstance{},
		},
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&zetav1.InferenceApp{}).
		Complete(sr); err != nil {
		return nil, err
	}
	return sr, nil
}
