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

package agent

import (
	modelctl "CentaurusAI/zeta/modelctl/grpc"
	"CentaurusAI/zeta/modelctl/pkg/message"
	node "CentaurusAI/zeta/modelctl/pkg/node"
	"CentaurusAI/zeta/modelctl/pkg/node/model"
	noderesource "CentaurusAI/zeta/modelctl/pkg/node/resource"
	"CentaurusAI/zeta/modelctl/pkg/types"

	"context"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	ktypes "k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
)

type nodeAgent struct {
	nodeIdentifer     string
	nodeRevision      uint64
	nodeIp            string
	log               logr.Logger
	modelCtlProxy     *ModelCtlProxy
	innerActor        message.Actor
	node              *types.Node
	resourceProviders []node.ResourceProvider
	modelRepository   *model.ModelRepository
}

const (
	NodeStatusCheckDuration = 1 * time.Minute
)

func (ms *nodeAgent) Run(ctx context.Context) error {
	ms.innerActor.Start()
	ms.log.Info("Node agent started, looping for exit signal")
	if err := ms.initNode(); err != nil {
		return err
	}
	ms.reportNodeState()
	timer := time.NewTicker(NodeStatusCheckDuration)
	for {
		select {
		case <-timer.C:
			ms.reportNodeState()
		case <-ctx.Done():
			ms.log.Info("Node agent exit")
			ms.modelCtlProxy.innerActor.Stop()
			ms.innerActor.Stop()
			return nil
		}
	}
}

func (ms *nodeAgent) getNodeRevision() uint64 {
	return atomic.LoadUint64(&ms.nodeRevision)
}

func (ms *nodeAgent) getNodeCapacity() (map[v1.ResourceName]*resource.Quantity, error) {
	res := map[v1.ResourceName]*resource.Quantity{}
	for _, v := range ms.resourceProviders {
		cap, _, err := v.GetResourceList()
		if err != nil {
			return nil, err
		}
		for k, v := range cap {
			res[k] = &v
		}
	}
	return res, nil
}

func (ms *nodeAgent) getNodeAllocatable() (map[v1.ResourceName]*resource.Quantity, error) {
	res := map[v1.ResourceName]*resource.Quantity{}
	for _, v := range ms.resourceProviders {
		_, free, err := v.GetResourceList()
		if err != nil {
			return nil, err
		}
		for k, v := range free {
			res[k] = &v
		}
	}
	return res, nil
}

func (ms *nodeAgent) getModelStates() (map[string]*modelctl.ModelState, error) {
	models, err := ms.modelRepository.ListModels()
	if err != nil {
		ms.log.Error(err, "Failed to get model status from repository")
		return nil, err
	}

	modelStates := map[string]*modelctl.ModelState{}
	for _, v := range models {
		modelState := &modelctl.ModelState{
			ModelIdentifier: string(v.Uid),
			ModelName:       v.Name,
			ModelNamespace:  v.Namespace,
			State:           types.ToModelGrpcState(v.ModelState.ServingState),
			Resource:        &modelctl.ModelResource{},
			ModelStatistic:  &modelctl.ModelStatistic{},
		}
		modelStates[modelState.ModelIdentifier] = modelState
	}

	return modelStates, nil
}

func (ms *nodeAgent) initNode() error {
	node := &types.Node{
		NodeIdentifer: ms.nodeIdentifer,
		Node: &modelctl.NodeState{
			NodeRevision: int64(ms.getNodeRevision()),
			State:        modelctl.NodeState_Initializing,
			Role:         modelctl.NodeState_InferenceNode,
			NodeIp:       ms.nodeIp,
			Port:         8000, // default port
			Capacity:     map[string]*resource.Quantity{},
			Allocatable:  map[string]*resource.Quantity{},
			ModelStates:  map[string]*modelctl.ModelState{},
		},
	}

	ms.node = node

	return nil
}

func (ms *nodeAgent) reportNodeState() {
	msg := modelctl.NodeMessage{
		NodeIdentifier: &modelctl.NodeIdentifier{},
		MessageType:    modelctl.MessageType_NODE_STATE,
		MessageBody: &modelctl.NodeMessage_NodeState{
			NodeState: &modelctl.NodeState{
				NodeRevision: int64(ms.getNodeRevision()),
				State:        ms.node.Node.State,
				Role:         modelctl.NodeState_InferenceNode,
				NodeIp:       ms.nodeIp,
				Port:         8000, // default port
			},
		},
	}
	err := ms.refreshResourceAndModels()
	if err != nil {
		ms.node.Node.State = modelctl.NodeState_Unknown
	} else {
		ms.node.Node.State = modelctl.NodeState_Ready
	}
	msg.GetNodeState().State = ms.node.Node.State

	msg.GetNodeState().Capacity = make(map[string]*resource.Quantity)
	for k, v := range ms.node.Node.Capacity {
		vv := *v
		msg.GetNodeState().Capacity[k] = &vv
	}
	msg.GetNodeState().Allocatable = make(map[string]*resource.Quantity)
	for k, v := range ms.node.Node.Allocatable {
		vv := *v
		msg.GetNodeState().Allocatable[k] = &vv
	}
	msg.GetNodeState().ModelStates = make(map[string]*modelctl.ModelState, len(ms.node.Node.ModelStates))
	for k, v := range ms.node.Node.ModelStates {
		msg.GetNodeState().ModelStates[k] = v
	}

	ms.modelCtlProxy.forward(message.ActorMessage{
		Sender: ms.innerActor.Reference(),
		Body:   &msg,
	})
}

func (ms *nodeAgent) refreshResourceAndModels() error {

	res, err := ms.getNodeCapacity()
	if err != nil {
		ms.node.Node.State = modelctl.NodeState_Unknown
		return err
	} else {
		for k, v := range res {
			ms.node.Node.Capacity[string(k)] = v
		}
	}
	res, err = ms.getNodeAllocatable()
	if err != nil {
		ms.node.Node.State = modelctl.NodeState_Unknown
		return err
	} else {
		for k, v := range res {
			ms.node.Node.Allocatable[string(k)] = v
		}
	}

	models, err := ms.getModelStates()
	if err != nil {
		ms.node.Node.State = modelctl.NodeState_Unknown
		return err
	} else {
		ms.node.Node.ModelStates = make(map[string]*modelctl.ModelState, len(models))
		for k, v := range models {
			ms.node.Node.ModelStates[string(k)] = v
		}
	}

	return nil
}

func (ms *nodeAgent) loadModel(cmd *modelctl.ModelLoad) (interface{}, error) {
	ms.log.Info("Load model", "model", cmd)

	model := types.Model{
		Uid:       ktypes.UID(cmd.ModelIdentifier),
		Name:      cmd.ModelName,
		Namespace: cmd.ModelNamespace,
		Version:   "1", //TODO, could be generation of app
		APIModel:  cmd.Model.DeepCopy(),
	}
	if err := ms.modelRepository.LoadModel(model, cmd.LazyLoad); err != nil {
		return nil, err
	}
	ms.log.Info("Model loaded", "model", cmd)
	// TODO, report model state immediately to modelctl server
	return nil, nil
}

func (ms *nodeAgent) unloadModel(cmd *modelctl.ModelUnload) (interface{}, error) {
	ms.log.Info("UnLoad model", "model", cmd)
	model := types.Model{
		Uid: ktypes.UID(cmd.ModelIdentifier),
	}
	if err := ms.modelRepository.UnloadModel(model); err != nil {
		return nil, err
	}
	ms.log.Info("Model unloaded", "model", cmd)
	// TODO, report model state immediately to modelctl server
	return nil, nil
}

func (ms *nodeAgent) processMessage(msg message.ActorMessage) (interface{}, error) {
	msgBody := msg.Body.(*modelctl.NodeMessage)
	msgType := msgBody.GetMessageType()
	ms.log.Info("Received message to node", "msg", msgBody)
	switch {
	case msgType == modelctl.MessageType_MODEL_LOAD:
		return ms.loadModel(msgBody.GetModelLoad())
	case msgType == modelctl.MessageType_MODEL_UNLOAD:
		return ms.unloadModel(msgBody.GetModelUnload())
	default:
		ms.log.Info("Received unhandled message, skip", "msgType", msgType)
	}
	return nil, nil
}

func NewAgent(nodeName, modelCtlEndpoint, nodeIp string) (*nodeAgent, error) {
	agent := &nodeAgent{
		nodeIdentifer:     nodeName,
		nodeRevision:      1,
		log:               ctrl.Log.WithName("NodeAgent"),
		resourceProviders: []node.ResourceProvider{noderesource.NewNvidiaProvider()},
		// modelRepository:   model.NewModelRepository(nodeIp),
		modelRepository: model.NewModelRepository("172.18.0.2"), // TODO, use real trition node ip
	}
	agent.innerActor = message.NewLocalChannelActor(nodeName, agent.processMessage)
	agent.modelCtlProxy = NewModelCtlProxy(agent.log, "", nodeName, modelCtlEndpoint)
	err := agent.modelCtlProxy.Start(agent.innerActor.Reference())
	if err != nil {
		return nil, err
	}
	if len(nodeIp) > 0 {
		agent.nodeIp = nodeIp
	} else {
		localIps, err := node.GetLocalV4IP()
		if err != nil {
			return nil, err
		}
		agent.nodeIp = localIps[0].String()
	}
	return agent, nil
}
