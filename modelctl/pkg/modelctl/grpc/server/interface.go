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
	"errors"

	"CentaurusAI/zeta/modelctl/grpc"
)

var (
	NodeRevisionOutOfOrderError = errors.New("revision out of order between server and node")
	NodeNotFoundError           = errors.New("node not found")
	NodeAlreadyExistError       = errors.New("node already exist")
)

// NodeMonitorInterface handle message from node
type NodeMonitorInterface interface {
	OnNodeConnect(nodeId string) error
	OnNodeDisconnect(nodeId string) error
	OnNodeStateUpdate(message *grpc.NodeMessage) (*grpc.NodeMessage, error)
}
