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

	"fmt"

	"github.com/go-logr/logr"
)

// work as a communication proxy with modelctl,
// it receive messages using modelctl.GetMessage stream and send to node actor
// it send messages to modelctl using modelctl.PutMessage
type ModelCtlProxy struct {
	log          logr.Logger
	nodeIP       string
	nodeName     string
	stop         bool
	innerActor   message.Actor
	modelCtl     ModelCtlClient
	proxyChannel chan *modelctl.NodeMessage
	nodeActor    message.ActorRef
}

func (n *ModelCtlProxy) Start(nodeActor message.ActorRef) error {
	// node actor is passed in as node actor initialization require modelctl actor ref
	n.nodeActor = nodeActor

	// init innerActor to start processing message for other actors
	n.innerActor.Start()

	// listen to grpc message
	if err := n.modelCtl.GetMessage(fmt.Sprintf("ModelCtlProxy@%s", n.nodeName), n.proxyChannel); err != nil {
		return err
	}

	// process grpc message in a go routine
	go func() {
		for {
			if n.stop {
				close(n.proxyChannel)
				n.log.Info("ModelCtl proxy exit")
				break
			}
			select {
			case msg, ok := <-n.proxyChannel:
				if ok {
					if msg.GetNodeIdentifier().GetIdentifier() != n.nodeName {
						n.log.Info("Received a message meant to send to different node %s, skip it", msg.GetNodeIdentifier().GetIdentifier())
					}

					msgType := msg.GetMessageType()
					switch {
					case msgType == modelctl.MessageType_UNSPECIFIED:
						n.log.Info("Received message with unspecified type, skip")
					default:
						n.notify(n.nodeActor, msg)
					}
				} else {
					n.log.Info("Receiving message from model ctl server meet error")
				}
			}
		}
	}()
	return nil
}

func (n *ModelCtlProxy) notify(receiver message.ActorRef, msg interface{}) error {
	return message.Send(n.innerActor.Reference(), receiver, msg)
}

func (n *ModelCtlProxy) Stop() error {
	n.stop = true
	n.innerActor.Stop()
	n.modelCtl.Stop()
	return nil
}

// when proxy received a message, it forward to modelctl
// do not return error as it works as proxy, node actors are supposed to resend new state if sending message failed
func (n *ModelCtlProxy) forward(msg message.ActorMessage) (interface{}, error) {
	msgBody := msg.Body.(*modelctl.NodeMessage)
	msgBody.NodeIdentifier = &modelctl.NodeIdentifier{
		Ip:         n.nodeIP,
		Identifier: n.nodeName,
	}
	n.log.Info("send message to modelctl", "msg", msgBody)
	if err := n.modelCtl.PutMessage(msgBody); err != nil {
		n.log.Error(err, "Failed to send message to modelctl")
	}
	return nil, nil
}

func initClient(log logr.Logger, nodeIP, nodeName, modelCtlEndpoint string) ModelCtlClient {
	f := NewModelCtlClient(
		log,
		&modelctl.NodeIdentifier{
			Ip:         nodeIP,
			Identifier: nodeName,
		},
		&ClientConfiguration{
			endpoint:       modelCtlEndpoint,
			connTimeout:    DefaultConnTimeout,
			callTimeout:    DefaultCallTimeout,
			maxRecvMsgSize: DefaultMaxRecvMsgSize,
		})
	f.Start()

	return f
}

func NewModelCtlProxy(log logr.Logger, nodeIP, nodeName, modelCtlEndpoint string) *ModelCtlProxy {
	client := initClient(log, nodeIP, nodeName, modelCtlEndpoint)
	proxy := &ModelCtlProxy{
		log:          log,
		nodeIP:       nodeIP,
		nodeName:     nodeName,
		stop:         false,
		modelCtl:     client,
		proxyChannel: make(chan *modelctl.NodeMessage, 30),
	}

	proxy.innerActor = message.NewLocalChannelActor(nodeName, proxy.forward)
	return proxy
}
