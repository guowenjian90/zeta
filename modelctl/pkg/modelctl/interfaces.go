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

package modelctl

import (
	"CentaurusAI/zeta/modelctl/pkg/types"
)

type ModelSchedulerInterface interface {
	Schedule(model *types.Model)
	Deschedule(model *types.Model)
}

type ModelScalerInterface interface {
	Scale(model string, quality int16)
}

type NodeManagerInterface interface {
	NodeAgentInterface
	NodeProviderInterface
}

// NodeAgentInterface send message to node
type NodeAgentInterface interface {
	SyncNode(nodeId string) error
	LoadModel(nodeId string, model *types.Model) error
	UnloadModel(nodeId string, model *types.Model) error
}

type NodeProviderInterface interface {
	Suscribe(subscriber string, channel chan *types.NodeEvent)
	Unsuscribe(subscriber string)
	Get(nodeIdentifier string) *types.Node
	List() []*types.Node
}
