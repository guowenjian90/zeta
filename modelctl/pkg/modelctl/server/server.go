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
	"CentaurusAI/zeta/modelctl/pkg/modelctl/autoscale"
	"CentaurusAI/zeta/modelctl/pkg/modelctl/scheduler"
	"context"

	"time"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	AppDefaultRequeueDuration = 3 * time.Second
)

type modelServer struct {
	client.Client
	scheme             *runtime.Scheme
	log                logr.Logger
	recorder           record.EventRecorder
	nodeMonitor        *NodeManager
	modelScheduler     *scheduler.ModelScheduler
	modelScaler        *autoscale.ModelScaler
	endpointReconciler *endpointReconciler
}

func (ms *modelServer) Run(ctx context.Context) error {
	ms.log.Info("Starting ModelCtl server")
	// TODO get certn and keyFile, and port from commandline flags, --tls-cert-file --tls-private-key-file
	go ms.nodeMonitor.Run(ctx, "", "", 18001)
	go ms.modelScheduler.Run(ctx)
	go ms.modelScaler.Run(ctx)
	go ms.endpointReconciler.Run(ctx)
	return nil
}

func NewServer(mgr manager.Manager) (*modelServer, error) {
	nodeManager := NewNodeManager(mgr)
	scheduler := scheduler.NewScheduler(nodeManager)

	updater, err := NewModelEndpointReconciler(mgr, nodeManager)
	if err != nil {
		return nil, err
	}

	autoScaler, err := autoscale.NewScaler(mgr, nodeManager, scheduler)
	if err != nil {
		return nil, err
	}

	return &modelServer{
		Client:             mgr.GetClient(),
		scheme:             mgr.GetScheme(),
		recorder:           mgr.GetEventRecorderFor("zeta-model-server"),
		log:                ctrl.Log.WithName("ModelServer"),
		nodeMonitor:        nodeManager,
		modelScheduler:     scheduler,
		modelScaler:        autoScaler,
		endpointReconciler: updater,
	}, nil
}
