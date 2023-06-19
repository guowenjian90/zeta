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

package types

import (
	"CentaurusAI/zeta/inference/apis/core/v1alpha1"
	"CentaurusAI/zeta/inference/pkg/helper"
	modelctl "CentaurusAI/zeta/modelctl/grpc"
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	ktypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ResourcePID v1.ResourceName = "pid"
	ResourceGPU v1.ResourceName = "gpu"
	MaxPID                      = 100
)

type ResourceEventType string

const (
	Add    ResourceEventType = "Add"
	Delete ResourceEventType = "Delete"
	Update ResourceEventType = "Update"
)

type ServingState string

const (
	ServingStateReady    ServingState = "READY"
	ServingStateUnloaded ServingState = "UNLOADED"
	ServingStateCreated  ServingState = "CREATED"
	ServingStateUnknown  ServingState = ""
)

type ModelState struct {
	ServingState    ServingState
	ModelStatistics ModelStatistics
}

type Model struct {
	Uid        ktypes.UID
	Name       string
	Namespace  string
	Version    string
	APIModel   *v1alpha1.Model
	ModelState *ModelState
}

type ModelStatistics struct {
	ResourceUsage    map[v1.ResourceName]ResourceUsage
	RequestStatistic RequestStatistic
}

type ResourceUsage struct {
	total resource.Quantity
	free  resource.Quantity
}

type RequestStatistic struct {
	requestInSecond int
	requestInMinute int
	requestInHour   int
}

func ToModelGrpcState(state ServingState) modelctl.ModelState_State {
	switch state {
	case ServingStateReady:
		return modelctl.ModelState_Loaded
	case ServingStateUnknown:
		return modelctl.ModelState_Creating
	case ServingStateUnloaded:
		return modelctl.ModelState_Evicting
	default:
		return modelctl.ModelState_Creating
	}
}

func FromModelGrpcState(state modelctl.ModelState_State) ServingState {
	switch state {
	case modelctl.ModelState_Loaded:
		return ServingStateReady
	case modelctl.ModelState_Creating:
		return ServingStateUnknown
	case modelctl.ModelState_Evicting:
		return ServingStateUnloaded
	default:
		return ServingStateUnknown
	}
}

func GetInferenceAppModel(cli client.Client, ctx context.Context, namespace, name string) (*Model, error) {
	app, err := helper.GetInferenceApp(cli, ctx, ktypes.NamespacedName{Namespace: namespace, Name: name})
	if err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	model := &Model{
		Uid:       app.UID,
		Name:      app.Name,
		Namespace: app.Namespace,
		Version:   fmt.Sprint(app.Generation),
		APIModel:  app.Spec.Model.DeepCopy(),
	}
	return model, nil
}
