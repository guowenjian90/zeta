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
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"sync"

	modelctl "CentaurusAI/zeta/modelctl/grpc"
	"CentaurusAI/zeta/modelctl/pkg/types"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/protobuf/types/known/emptypb"

	ctrl "sigs.k8s.io/controller-runtime"
)

const NodeOutgoingChanBufferSize = 20
const DefaultNodeIncomingHandlerNum = 5

type ModelCtlServer interface {
	DispatchNodeMessage(nodeId string, message *modelctl.NodeMessage) error
	LoadModel(nodeIdentifier string, model *types.Model) error
	UnloadModel(nodeIdentifier string, model *types.Model) error
}

var _ ModelCtlServer = (*grpcServer)(nil)
var _ modelctl.ModelCtlServiceServer = (*grpcServer)(nil)

type grpcServer struct {
	sync.RWMutex
	modelctl.UnimplementedModelCtlServiceServer
	nodeMonitor             NodeMonitorInterface
	nodeOutgoingChans       map[string]chan<- *modelctl.NodeMessage
	nodeIncommingChans      map[string]chan *modelctl.NodeMessage
	nodeIncommingChansMutex sync.RWMutex
	nodeMessageHandlerChans []chan *modelctl.NodeMessage
}

func (g *grpcServer) getNodeChan(nodeIdentifer string) (chan<- *modelctl.NodeMessage, error) {
	g.RLock()
	defer g.RUnlock()

	ch, ok := g.nodeOutgoingChans[nodeIdentifer]
	if !ok {
		return nil, fmt.Errorf("unknown destination")
	}

	return ch, nil
}

func (g *grpcServer) DispatchNodeMessage(nodeIdentifier string, message *modelctl.NodeMessage) (err error) {
	func() {
		defer func() {
			if err := recover(); err != nil {
				// node could be disconnected when dispacthing message
				err = errors.New("node channel panic")
			}
		}()

		var ch chan<- *modelctl.NodeMessage
		ch, err = g.getNodeChan(nodeIdentifier)
		if err == nil && ch != nil {
			ch <- message
		}
	}()
	return err
}

// SyncNode implements ModelCtlServer
func (g *grpcServer) SyncNode(nodeIdentifier string) (err error) {
	err = g.DispatchNodeMessage(nodeIdentifier, newSyncRequest())
	if err != nil {
		ctrl.Log.Error(err, "Failed to dispatch full sync message to node", "node", nodeIdentifier)
	}
	return err
}

func (g *grpcServer) RunGrpcServer(ctx context.Context, port int, certFile, keyFile string) (err error) {
	listner, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		ctrl.Log.Error(err, "Modelctl grpc server failed to listen on port:", port)
		return err
	}

	var opts []grpc.ServerOption
	if certFile != "" && keyFile != "" {
		creds, err := credentials.NewServerTLSFromFile(certFile, keyFile)
		if err != nil {
			ctrl.Log.Error(err, "Modelctl grpc server failed to generate credentials", "certFile", certFile, "keyFile", keyFile)
			return err
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}

	// start grpc server
	grpcServer := grpc.NewServer(opts...)
	modelctl.RegisterModelCtlServiceServer(grpcServer, g)
	go func() {
		err = grpcServer.Serve(listner)
		if err != nil {
			ctrl.Log.Error(err, "Modelctl grpc server stopped to serve")
		}
	}()

	for _, v := range g.nodeMessageHandlerChans {
		go func(ch chan *modelctl.NodeMessage) {
			for {
				select {
				case <-ctx.Done():
					return
				case msg := <-ch:
					g.handleMessages(msg)
				}
			}
		}(v)
	}
	return nil
}

func (g *grpcServer) enlistNode(node string, ch chan<- *modelctl.NodeMessage) (err error) {
	g.Lock()
	if _, ok := g.nodeOutgoingChans[node]; ok {
		g.Unlock()
		return fmt.Errorf("node %s already has channel", node)
	}
	g.nodeOutgoingChans[node] = ch
	g.Unlock()
	err = g.nodeMonitor.OnNodeConnect(node)
	if err != nil {
		if err == NodeRevisionOutOfOrderError {
			g.DispatchNodeMessage(node, newSyncRequest())
		} else {
			return err
		}
	}
	return nil
}

func (g *grpcServer) delistNode(node string) {
	g.Lock()
	g.nodeMonitor.OnNodeDisconnect(node)
	if ch, found := g.nodeOutgoingChans[node]; found {
		delete(g.nodeOutgoingChans, node)
		close(ch)
	}
	g.Unlock()
}

func (g *grpcServer) GetMessage(identifier *modelctl.NodeIdentifier, server modelctl.ModelCtlService_GetMessageServer) error {
	ch := make(chan *modelctl.NodeMessage, NodeOutgoingChanBufferSize)
	if err := g.enlistNode(identifier.GetIdentifier(), ch); err != nil {
		close(ch)
		return fmt.Errorf("Modectl server has established stream channel with this node: %s", identifier)
	}

	chDone := server.Context().Done()
	for {
		select {
		case <-chDone:
			g.delistNode(identifier.GetIdentifier())
			return nil
		case msg := <-ch:
			msg.NodeIdentifier = identifier
			if err := server.Send(msg); err != nil {
				ctrl.Log.Error(err, "Failed to send message to node via GetMessage stream channel", "node", identifier)
				g.delistNode(identifier.GetIdentifier())
				return err
			}
		}
	}
}

// get handeller channel to handle incomming message from this node, if not found,
// randomly assign one channel in nodeMessageHandlerChans to this node and save it for following incomming messages from this node
func (g *grpcServer) getNodeMessageHandlerChannel(nodeId string) chan *modelctl.NodeMessage {
	g.nodeIncommingChansMutex.RLock()
	if messageCh, found := g.nodeIncommingChans[nodeId]; found {
		g.nodeIncommingChansMutex.RUnlock()
		return messageCh
	} else {
		g.nodeIncommingChansMutex.RUnlock()
		i := rand.Intn(len(g.nodeMessageHandlerChans))
		channel := g.nodeMessageHandlerChans[i]
		g.nodeIncommingChansMutex.Lock()
		g.nodeIncommingChans[nodeId] = channel
		g.nodeIncommingChansMutex.Unlock()
		return channel
	}
}

// PutMessage send node's message to handler to process message and return
func (g *grpcServer) PutMessage(ctx context.Context, message *modelctl.NodeMessage) (*empty.Empty, error) {
	messageCh := g.getNodeMessageHandlerChannel(message.GetNodeIdentifier().GetIdentifier())
	messageCh <- message
	return &emptypb.Empty{}, nil
}

func (g *grpcServer) handleMessages(message *modelctl.NodeMessage) {
	var err error
	var reply *modelctl.NodeMessage
	switch message.GetMessageType() {
	case modelctl.MessageType_NODE_STATE:
		reply, err = g.nodeMonitor.OnNodeStateUpdate(message)
	default:
		ctrl.Log.Info(fmt.Sprintf("not supported message type %s, message %v", message.GetMessageType(), message))
	}
	if err != nil {
		ctrl.Log.Error(err, "Failed to process a node message", "node", message.GetNodeIdentifier(), "msgType", message.GetMessageType())
	}
	if err == NodeRevisionOutOfOrderError {
		g.DispatchNodeMessage(message.GetNodeIdentifier().GetIdentifier(), newSyncRequest())
	}
	if reply != nil {
		g.DispatchNodeMessage(message.GetNodeIdentifier().GetIdentifier(), reply)
	}
}

func NewGrpcServer(nodeMonitor NodeMonitorInterface) *grpcServer {
	handlerChans := []chan *modelctl.NodeMessage{}
	for i := 0; i < DefaultNodeIncomingHandlerNum; i++ {
		handlerChans = append(handlerChans, make(chan *modelctl.NodeMessage, 1000))
	}

	return &grpcServer{
		RWMutex:                            sync.RWMutex{},
		nodeOutgoingChans:                  make(map[string]chan<- *modelctl.NodeMessage),
		nodeIncommingChans:                 make(map[string]chan *modelctl.NodeMessage),
		nodeMonitor:                        nodeMonitor,
		UnimplementedModelCtlServiceServer: modelctl.UnimplementedModelCtlServiceServer{},
		nodeMessageHandlerChans:            handlerChans,
	}
}

// Load model
func (g *grpcServer) LoadModel(nodeIdentifier string, model *types.Model) (err error) {
	messageType := modelctl.MessageType_MODEL_LOAD
	body := modelctl.NodeMessage_ModelLoad{
		ModelLoad: &modelctl.ModelLoad{
			ModelIdentifier: string(model.Uid),
			ModelName:       model.Name,
			ModelNamespace:  model.Namespace,
			Version:         model.Version,
			Model:           model.APIModel.DeepCopy(),
			LazyLoad:        false,
		},
	}
	m := &modelctl.NodeMessage{
		NodeIdentifier: &modelctl.NodeIdentifier{
			Ip:         "",
			Identifier: nodeIdentifier,
		},
		MessageType: messageType,
		MessageBody: &body,
	}

	// ctrl.Log.Info("Dispatch message to node", "msg", m.GetMessageBody())
	err = g.DispatchNodeMessage(nodeIdentifier, m)
	if err != nil {
		ctrl.Log.Error(err, "Failed to dispatch message to node", "node", nodeIdentifier)
		return err
	}
	return nil
}

// Unload model
func (g *grpcServer) UnloadModel(nodeIdentifier string, model *types.Model) (err error) {
	messageType := modelctl.MessageType_MODEL_UNLOAD
	body := modelctl.NodeMessage_ModelUnload{
		ModelUnload: &modelctl.ModelUnload{
			ModelIdentifier: string(model.Uid),
		},
	}
	m := &modelctl.NodeMessage{
		NodeIdentifier: &modelctl.NodeIdentifier{
			Ip:         "",
			Identifier: nodeIdentifier,
		},
		MessageType: messageType,
		MessageBody: &body,
	}

	err = g.DispatchNodeMessage(nodeIdentifier, m)
	if err != nil {
		ctrl.Log.Error(err, "Failed to dispatch message to node", "node", nodeIdentifier)
		return err
	}
	return nil
}

func newSyncRequest() *modelctl.NodeMessage {
	return &modelctl.NodeMessage{
		MessageType: modelctl.MessageType_NODE_SYNC,
		MessageBody: &modelctl.NodeMessage_NodeSync{},
	}
}
