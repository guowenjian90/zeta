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

package resource

import (
	"CentaurusAI/zeta/modelctl/pkg/node"
	"CentaurusAI/zeta/modelctl/pkg/types"
	"CentaurusAI/zeta/modelctl/pkg/util"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type NvidiaProvider struct {
	total v1.ResourceList
	free  v1.ResourceList
}

var _ node.ResourceProvider = (*NvidiaProvider)(nil)

// GetResourceList implements node.ResourceProvider
func (n *NvidiaProvider) GetResourceList() (v1.ResourceList, v1.ResourceList, error) {
	return n.total, n.free, nil
}

func NewNvidiaProvider() *NvidiaProvider {
	// TODO, regually check and save
	return &NvidiaProvider{
		total: map[v1.ResourceName]resource.Quantity{
			types.ResourceGPU: util.ResourceQuantity(8000*1000*1000, types.ResourceGPU),
		},
		free: map[v1.ResourceName]resource.Quantity{
			types.ResourceGPU: util.ResourceQuantity(8000*1000*1000, types.ResourceGPU),
		},
	}
}
