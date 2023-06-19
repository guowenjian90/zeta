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

package autoscale

import (
	zetav1 "CentaurusAI/zeta/inference/apis/core/v1alpha1"
	"CentaurusAI/zeta/inference/pkg/helper"
	"CentaurusAI/zeta/modelctl/pkg/modelctl"
	"CentaurusAI/zeta/modelctl/pkg/types"
	"CentaurusAI/zeta/modelctl/pkg/util"
	"fmt"

	"context"

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	// "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ modelctl.ModelScalerInterface = (*ModelScaler)(nil)

type ModelScaler struct {
	client.Client
	recorder        record.EventRecorder
	log             logr.Logger
	scheduler       modelctl.ModelSchedulerInterface
	nodeManager     modelctl.NodeProviderInterface
	nodeInfoChannel chan *types.NodeEvent
}

var _ reconcile.Reconciler = (*ModelScaler)(nil)

// scale implements ModelScalerInterface
func (*ModelScaler) Scale(model string, quality int16) {
	panic("unimplemented")
}

func (ms *ModelScaler) handleNodeUpdate(nodeEvent *types.NodeEvent) error {
	v1node := nodeEvent.Node
	eventType := nodeEvent.EventType
	nodeName := util.Name(v1node)
	ms.log.Info("received node update", "node", nodeName, "eventType", eventType)
	// TODO
	return nil
}

// 1/ listen to api server, maintain app list
// 2/ listen to node monitor, maintain app runnning status
// 3/ listen to app statistics, decide how to scale up
// 4/ Scaler does not scale down, node to unload model itself when there is no incomming requests in a period
// node unload model in two steps,
//   1/ set model is soft unloaded status, send unload message
//   2/ if no new incomming request in a minute after soft unload status, then do hard unload
//   3/ if new incomming request in a minute after soft unload status, then reset soft unload status

func (ms *ModelScaler) Run(ctx context.Context) error {
	ms.log.Info("Starting modelctl model scaler")
	ms.nodeManager.Suscribe("ModelScaler", ms.nodeInfoChannel)
	for {
		select {
		case <-ctx.Done():
			close(ms.nodeInfoChannel)
			ms.log.Info("Model scaler exit")
			break
		case update := <-ms.nodeInfoChannel:
			ms.handleNodeUpdate(update)
		}
	}
}

// Reconcile implements reconcile.Reconciler
func (r *ModelScaler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {

	r.log.Info("reconciling InferenceApp", "name", request.NamespacedName)

	// Get InferenceApp instance
	var err error
	var app *zetav1.InferenceApp
	if app, err = helper.GetInferenceApp(r.Client, ctx, request.NamespacedName); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// currently only try to place model on single node
	// TODO, scale model on more nodes according model request metrics from node and gateway
	if app.ObjectMeta.DeletionTimestamp.IsZero() {
		if app.Status.NumOfInstance == 0 {
			// create a schedule request
			model := &types.Model{
				Uid:       app.UID,
				Name:      app.Name,
				Namespace: app.Namespace,
				Version:   fmt.Sprint(app.Generation),
				APIModel:  app.Spec.Model.DeepCopy(),
			}
			r.log.Info("Scheduling model", "name", app.Name, "resource requirement", model.APIModel.Resource)
			r.scheduler.Schedule(model)
		}
		return ctrl.Result{
			Requeue:      false,
			RequeueAfter: 0,
		}, nil
	} else {
		r.log.Info("App is being deleted", "DeletionTimestamp", app.ObjectMeta.DeletionTimestamp)
		// do nothing, service updater will unload all models when service is deleted
	}
	return ctrl.Result{}, nil
}

// NewScaler returns a new ModelScaler
func NewScaler(mgr manager.Manager, nodeManager modelctl.NodeManagerInterface, scheduler modelctl.ModelSchedulerInterface) (*ModelScaler, error) {
	r := &ModelScaler{
		Client:          mgr.GetClient(),
		log:             ctrl.Log.WithName("ModelCtl").WithName("AutoScaler"),
		recorder:        mgr.GetEventRecorderFor("zeta-modelctl-autoscaler"),
		scheduler:       scheduler,
		nodeManager:     nodeManager,
		nodeInfoChannel: make(chan *types.NodeEvent, 100),
	}

	err := ctrl.NewControllerManagedBy(mgr).
		For(&zetav1.InferenceApp{}).
		Complete(r)
	if err != nil {
		return nil, err
	}

	return r, nil
}
