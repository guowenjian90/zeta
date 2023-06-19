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
	"CentaurusAI/zeta/modelctl/grpc"
	"CentaurusAI/zeta/modelctl/pkg/modelctl"
	grpc_server "CentaurusAI/zeta/modelctl/pkg/modelctl/grpc/server"
	"CentaurusAI/zeta/modelctl/pkg/types"

	"context"
	"errors"
	"sync"

	"github.com/go-logr/logr"
	"google.golang.org/protobuf/proto"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type connectionState string

const (
	DisconnectedState connectionState = "Disconnected"
	InSyncState       connectionState = "InSync"
	OutofSyncState    connectionState = "OutofSync"
)

type nodeWithRevision struct {
	NodeId          string
	Revision        int64
	ConnectionState connectionState
	Node            *types.Node
}

type nodeRevisionMap struct {
	mu    sync.RWMutex
	nodes map[string]*nodeWithRevision
}

func (pool *nodeRevisionMap) add(name string, node *nodeWithRevision) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	pool.nodes[name] = node
}

func (pool *nodeRevisionMap) delete(name string) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	delete(pool.nodes, name)
}

func (pool *nodeRevisionMap) get(name string) *nodeWithRevision {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	if n, found := pool.nodes[name]; found {
		return n
	}

	return nil
}

func (pool *nodeRevisionMap) list() []*types.Node {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	nodes := []*types.Node{}
	for _, v := range pool.nodes {
		nodes = append(nodes, v.Node)
	}
	return nodes
}

type nodeInfoConsumerMap struct {
	mu                sync.RWMutex
	nodeInfoConsumers map[string]NodeInfoConsumer
}

func (m *nodeInfoConsumerMap) add(subscriber string, channel chan *types.NodeEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodeInfoConsumers[subscriber] = channel
}

func (m *nodeInfoConsumerMap) remove(subscriber string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.nodeInfoConsumers, subscriber)
}

func (m *nodeInfoConsumerMap) get() map[string]chan *types.NodeEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	mm := make(map[string]chan *types.NodeEvent, len(m.nodeInfoConsumers))
	for k, v := range m.nodeInfoConsumers {
		mm[k] = v
	}
	return mm
}

func (m *nodeInfoConsumerMap) list() map[string]chan *types.NodeEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	consumers := make(map[string]chan *types.NodeEvent, len(m.nodeInfoConsumers))
	for k, v := range m.nodeInfoConsumers {
		consumers[k] = v
	}
	return consumers
}

type NodeInfoConsumer chan *types.NodeEvent

type NodeManager struct {
	Scheme              *runtime.Scheme
	log                 logr.Logger
	nodes               nodeRevisionMap
	staleNodes          nodeRevisionMap
	nodeInfoConsumerMap nodeInfoConsumerMap
	grpcServer          grpc_server.ModelCtlServer
}

var _ grpc_server.NodeMonitorInterface = (*NodeManager)(nil)
var _ modelctl.NodeAgentInterface = (*NodeManager)(nil)
var _ modelctl.NodeProviderInterface = (*NodeManager)(nil)

// suscribe implements types.NodeInfoProviderInterface
func (nm *NodeManager) Suscribe(subscriber string, channel chan *types.NodeEvent) {
	nm.nodeInfoConsumerMap.add(subscriber, channel)
}

// unsuscribe implements types.NodeInfoProviderInterface
func (nm *NodeManager) Unsuscribe(subscriber string) {
	nm.nodeInfoConsumerMap.remove(subscriber)
}

// get implements types.NodeInfoProviderInterface
func (nm *NodeManager) Get(nodeIdentifier string) *types.Node {
	if n := nm.nodes.get(nodeIdentifier); n != nil {
		return n.Node
	}
	if n := nm.staleNodes.get(nodeIdentifier); n != nil {
		return n.Node
	}
	return nil
}

// list implements types.NodeInfoProviderInterface
func (nm *NodeManager) List() []*types.Node {
	return nm.nodes.list()
}

func (nm *NodeManager) boardcastNodeInfo(nodeEvent *types.NodeEvent) {
	send := func(channel chan *types.NodeEvent, msg *types.NodeEvent) error {
		var err error
		func() {
			defer func() {
				if err := recover(); err != nil {
					nm.log.Info("channel panic occurred")
					err = errors.New("channel panic")
				}
			}()
			channel <- msg
		}()
		return err
	}

	badConsumers := map[string]bool{}
	a := sync.WaitGroup{}
	for k, v := range nm.nodeInfoConsumerMap.list() {
		a.Add(1)
		go func(k string, channel chan *types.NodeEvent, msg *types.NodeEvent) {
			if err := send(channel, msg); err != nil {
				badConsumers[k] = true
			}
			a.Done()
		}(k, v, nodeEvent)
	}
	a.Wait()

	for k := range badConsumers {
		nm.nodeInfoConsumerMap.remove(k)
	}
}

// LoadModel implements nodeagent.NodeAgentInterface
func (nm *NodeManager) LoadModel(nodeId string, model *types.Model) error {
	nm.log.Info("Load model ", "namespace", model.Namespace, "model", model.Name, " node", nodeId)
	return nm.grpcServer.LoadModel(nodeId, model)
}

// SyncNode implements nodeagent.NodeAgentInterface
func (nm *NodeManager) SyncNode(nodeId string) error {
	message := grpc.NodeMessage{
		NodeIdentifier: &grpc.NodeIdentifier{
			Identifier: nodeId,
		},
		MessageType: grpc.MessageType_NODE_SYNC,
		MessageBody: &grpc.NodeMessage_NodeSync{
			NodeSync: &grpc.NodeSync{},
		},
	}
	return nm.grpcServer.DispatchNodeMessage(nodeId, &message)
}

// UnloadModel implements nodeagent.NodeAgentInterface
func (nm *NodeManager) UnloadModel(nodeId string, model *types.Model) error {
	nm.log.Info("Unload model ", "modelId", model.Uid, "node", nodeId)
	return nm.grpcServer.UnloadModel(nodeId, model)
}

// OnNodeConnect implements nodeagent.NodeMonitorInterface
func (nm *NodeManager) OnNodeConnect(nodeId string) error {
	nm.log.Info("A node connected", "node", nodeId)
	nodeWRev := nm.staleNodes.get(nodeId)
	if nodeWRev != nil {
		nodeWRev.ConnectionState = OutofSyncState
		nm.staleNodes.delete(nodeId)
	}
	// ask node to full sync since this node was disconnected before
	return grpc_server.NodeRevisionOutOfOrderError
}

// OnNodeDisconnect implements nodeagent.NodeMonitorInterface
func (nm *NodeManager) OnNodeDisconnect(nodeId string) error {
	nm.log.Info("A node disconnected", "node", nodeId)
	nodeWRev := nm.nodes.get(nodeId)
	if nodeWRev != nil {
		nodeWRev.ConnectionState = DisconnectedState
		nm.staleNodes.add(nodeId, nodeWRev)
		nm.boardcastNodeInfo(&types.NodeEvent{
			EventType: types.Delete,
			Node:      nodeWRev.Node,
		})
		nm.staleNodes.add(nodeId, nodeWRev)
	}
	return nil
}

// OnNodeStateUpdate implements nodeagent.NodeMonitorInterface
func (nm *NodeManager) OnNodeStateUpdate(message *grpc.NodeMessage) (*grpc.NodeMessage, error) {
	nodeId := message.GetNodeIdentifier().GetIdentifier()
	revision := message.GetNodeState().GetNodeRevision()

	nodeWRev := nm.nodes.get(nodeId)
	if nodeWRev == nil {
		nodeWRev = &nodeWithRevision{
			NodeId:   nodeId,
			Revision: revision,
			Node: &types.Node{
				NodeIdentifer: nodeId,
				Node:          proto.Clone(message.GetNodeState()).(*grpc.NodeState),
			},
		}
		nm.nodes.add(nodeId, nodeWRev)
	} else {
		// if node revision is outof order, as a full sync
		currRevision := nodeWRev.Revision
		nodeWRev.Node.Node = proto.Clone(message.GetNodeState()).(*grpc.NodeState)
		nodeWRev.Revision = revision
		if revision < currRevision {
			nodeWRev.ConnectionState = OutofSyncState
			nm.log.Info("A node state is out of sync", "node", nodeId, "revision", revision, "revisionInNode", currRevision)
			// maybe still boradcast node info although revision is out of sync
			return nil, grpc_server.NodeRevisionOutOfOrderError
		}
	}

	nodeWRev.ConnectionState = InSyncState
	nm.log.Info("A node is updating state", "node", nodeId, "revision", revision, "allocated", nodeWRev.Node.Node.Allocatable, "capacity", nodeWRev.Node.Node.Capacity, "model", nodeWRev.Node.Node.ModelStates)
	nm.boardcastNodeInfo(&types.NodeEvent{
		EventType: types.Add,
		Node: &types.Node{
			NodeIdentifer: nodeId,
			Node:          nodeWRev.Node.Node,
		},
	})

	return nil, nil
}

// TODO get certn and keyFile from commandline flags, --tls-cert-file --tls-private-key-file
// run grpc server to receive node registry and status, and maintain message stream channel with node
func (nm *NodeManager) Run(ctx context.Context, certFile, keyFile string, port int) error {
	nm.log.Info("Starting ModelCtl node manager")
	grpcServer := grpc_server.NewGrpcServer(nm)
	nm.grpcServer = grpcServer
	err := grpcServer.RunGrpcServer(ctx, port, certFile, keyFile)
	if err != nil {
		return err
	}
	nm.log.Info("ModelCtl node manager started")

	for {
		select {
		case <-ctx.Done():
			nm.log.Info("ModelCtl Node Manager exit")
			return nil
		}
	}
}

func NewNodeManager(mgr manager.Manager) *NodeManager {
	nm := &NodeManager{
		Scheme: mgr.GetScheme(),
		log:    ctrl.Log.WithName("ModelServer").WithName("NodeManager"),
		nodes: nodeRevisionMap{
			mu:    sync.RWMutex{},
			nodes: map[string]*nodeWithRevision{},
		},
		staleNodes: nodeRevisionMap{
			mu:    sync.RWMutex{},
			nodes: map[string]*nodeWithRevision{},
		},
		nodeInfoConsumerMap: nodeInfoConsumerMap{
			mu:                sync.RWMutex{},
			nodeInfoConsumers: map[string]NodeInfoConsumer{},
		},
	}
	return nm
}
