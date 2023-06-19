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

package scheduler

import (
	"CentaurusAI/zeta/modelctl/pkg/collection"
	"CentaurusAI/zeta/modelctl/pkg/types"
	"CentaurusAI/zeta/modelctl/pkg/util"

	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
)

type SchedulableNode struct {
	mu                      sync.Mutex
	NodeIdentifier          string
	Node                    *types.Node
	LastSeen                time.Time
	LastUsed                time.Time
	ResourceList            v1.ResourceList
	PreOccupiedResourceList v1.ResourceList
}

func (snode *SchedulableNode) AdmitOccupiedResourceList(resourceList *v1.ResourceList) {
	snode.mu.Lock()
	nodeCpu := snode.PreOccupiedResourceList.Cpu().DeepCopy()
	cpu := resourceList.Cpu().DeepCopy()
	nodeCpu.Add(cpu)
	if nodeCpu.Sign() <= 0 {
		nodeCpu.Set(0)
	}
	snode.PreOccupiedResourceList[v1.ResourceCPU] = nodeCpu.DeepCopy()

	nodeMemory := snode.PreOccupiedResourceList.Memory().DeepCopy()
	memory := resourceList.Memory().DeepCopy()
	nodeMemory.Add(memory)
	if nodeMemory.Sign() <= 0 {
		nodeMemory.Set(0)
	}
	snode.PreOccupiedResourceList[v1.ResourceMemory] = nodeMemory

	nodeStorage := snode.PreOccupiedResourceList.Storage().DeepCopy()
	storage := resourceList.Storage().DeepCopy()
	nodeStorage.Add(storage)
	if nodeStorage.Sign() <= 0 {
		nodeStorage.Set(0)
	}
	snode.PreOccupiedResourceList[v1.ResourceStorage] = nodeStorage
	snode.mu.Unlock()
}

func (snode *SchedulableNode) GetAllocatableResources() v1.ResourceList {
	snode.mu.Lock()
	allocatedResources := v1.ResourceList{}

	nodeCpu := snode.ResourceList.Cpu().DeepCopy()
	cpu := snode.PreOccupiedResourceList.Cpu().DeepCopy()
	nodeCpu.Sub(cpu)
	if nodeCpu.Sign() <= 0 {
		nodeCpu.Set(0)
	}
	allocatedResources[v1.ResourceCPU] = nodeCpu.DeepCopy()

	nodeMemory := snode.ResourceList.Memory().DeepCopy()
	memory := snode.PreOccupiedResourceList.Memory().DeepCopy()
	nodeMemory.Sub(memory)
	if nodeMemory.Sign() <= 0 {
		nodeMemory.Set(0)
	}
	allocatedResources[v1.ResourceMemory] = nodeMemory

	nodeStorage := snode.ResourceList.Storage().DeepCopy()
	storage := snode.PreOccupiedResourceList.Storage().DeepCopy()
	nodeStorage.Sub(storage)
	if nodeStorage.Sign() <= 0 {
		nodeStorage.Set(0)
	}
	allocatedResources[v1.ResourceStorage] = nodeStorage

	nodeGPU := snode.ResourceList.Name(types.ResourceGPU, resource.BinarySI).DeepCopy()
	gpu := snode.PreOccupiedResourceList.Name(types.ResourceGPU, resource.BinarySI).DeepCopy()
	nodeGPU.Sub(gpu)
	if nodeGPU.Sign() <= 0 {
		nodeGPU.Set(0)
	}
	allocatedResources[types.ResourceGPU] = nodeGPU

	snode.mu.Unlock()
	return allocatedResources
}

func (snode *SchedulableNode) ReleaseOccupiedResourceList(resourceList *v1.ResourceList) {
	snode.mu.Lock()
	nodeCpu := snode.PreOccupiedResourceList.Cpu().DeepCopy()
	cpu := resourceList.Cpu().DeepCopy()
	nodeCpu.Sub(cpu)
	if nodeCpu.Sign() <= 0 {
		nodeCpu.Set(0)
	}
	snode.PreOccupiedResourceList[v1.ResourceCPU] = nodeCpu.DeepCopy()

	nodeMemory := snode.PreOccupiedResourceList.Memory().DeepCopy()
	memory := resourceList.Memory().DeepCopy()
	nodeMemory.Sub(memory)
	if nodeMemory.Sign() <= 0 {
		nodeMemory.Set(0)
	}
	snode.PreOccupiedResourceList[v1.ResourceMemory] = nodeMemory

	nodeStorage := snode.PreOccupiedResourceList.Storage().DeepCopy()
	storage := resourceList.Storage().DeepCopy()
	nodeStorage.Sub(storage)
	if nodeStorage.Sign() <= 0 {
		nodeStorage.Set(0)
	}
	snode.PreOccupiedResourceList[v1.ResourceStorage] = nodeStorage
	snode.mu.Unlock()
}

type SortedNodes struct {
	nodes    []*SchedulableNode
	lessFunc collection.LessFunc
}

func (sn *SortedNodes) Len() int {
	return len(sn.nodes)
}

func (sn *SortedNodes) Less(i, j int) bool {
	return sn.lessFunc(sn.nodes[i], sn.nodes[j])
}

func (sn *SortedNodes) Swap(i, j int) {
	sn.nodes[i], sn.nodes[j] = sn.nodes[j], sn.nodes[i]
}

type SchedulableNodePool struct {
	mu    sync.RWMutex
	nodes map[string]*SchedulableNode
}

func (pool *SchedulableNodePool) GetNode(name string) *SchedulableNode {
	pool.mu.RLock()
	if n, f := pool.nodes[name]; f {
		pool.mu.RUnlock()
		return n
	}
	pool.mu.RUnlock()
	return nil
}

func (pool *SchedulableNodePool) DeleteNode(name string) {
	pool.mu.Lock()
	delete(pool.nodes, name)
	pool.mu.Unlock()
}

func (pool *SchedulableNodePool) AddNode(name string, node *SchedulableNode) {
	pool.mu.Lock()
	pool.nodes[name] = node
	pool.mu.Unlock()
}

func (pool *SchedulableNodePool) GetNodes() []*SchedulableNode {
	nodes := []*SchedulableNode{}
	pool.mu.RLock()
	for _, v := range pool.nodes {
		nodes = append(nodes, v)
	}
	pool.mu.RUnlock()
	return nodes
}

func (pool *SchedulableNodePool) size() int {
	return len(pool.nodes)
}

func (pool *SchedulableNodePool) printSummary() {
	nodes := pool.nodes
	for _, v := range nodes {
		allocatable := v.GetAllocatableResources()
		klog.InfoS("Scheduleable node reource", "node", v.NodeIdentifier, "capacity", v.ResourceList, "allocatable", allocatable)
	}
}

func GetNodeAllocatableResourceList(node *types.Node) v1.ResourceList {
	resourceList := v1.ResourceList{}
	res := node.Node.GetAllocatable()
	for k, v := range res {
		if v.Sign() > 0 {
			resourceList[v1.ResourceName(k)] = *v
		} else {
			resourceList[v1.ResourceName(k)] = util.ResourceQuantity(0, v1.ResourceName(k))
		}
	}

	return resourceList
}
