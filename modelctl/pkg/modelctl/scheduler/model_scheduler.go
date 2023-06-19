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
	"CentaurusAI/zeta/modelctl/pkg/modelctl"
	"CentaurusAI/zeta/modelctl/pkg/types"
	"CentaurusAI/zeta/modelctl/pkg/util"
	"context"

	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"

	ctrl "sigs.k8s.io/controller-runtime"
)

type ModelScheduler struct {
	ctx                       context.Context
	log                       logr.Logger
	stop                      bool
	nodeManager               modelctl.NodeManagerInterface
	nodeInfoChannel           chan *types.NodeEvent
	scheduleQueue             *ModelScheduleQueue
	nodePool                  *SchedulableNodePool
	scheduleConditionBuilders []ConditionBuildFunc
	policy                    *SchedulePolicy
	nodeGroups                []*nodeGroup
}

var _ modelctl.ModelSchedulerInterface = (*ModelScheduler)(nil)

// deschedule remove a model from schedule queue
func (ms *ModelScheduler) Deschedule(model *types.Model) {
	ms.scheduleQueue.RemoveModel(model)
}

// client call schedule method to find a server to load a model
// schedule request is added into schedule queue,
// scheduler listen to node monitor to find node meet resource requirement
// and call node agnet interface to send load model command to node
func (ms *ModelScheduler) Schedule(model *types.Model) {
	ms.scheduleQueue.AddModel(model, 0)
}

type NodeSortingMethod string

const (
	DefaultBackoffRetryDuration                    = 5 * time.Second
	NodeSortingMethodMoreMemory  NodeSortingMethod = "more_memory"   // choose node with more memory
	NodeSortingMethodLessLastUse NodeSortingMethod = "less_last_use" // choose node least used before
	NodeSortingMethodLessUse     NodeSortingMethod = "less_use"      // choose node with less use
)

var (
	ModelIsDeletedError       = fmt.Errorf("Model has a deletion timestamp")
	InsufficientResourceError = fmt.Errorf("can not find node with sufficient resources")
	ModelBindToNodeError      = fmt.Errorf("Model bind to node error")
)

// we want to use more memory node, so, lager value are put ahead in sorted list
func NodeHasMoreMemorySortFunc(pi, pj interface{}) bool {
	piResource := pi.(*SchedulableNode).GetAllocatableResources()
	piResourceV, _ := piResource.Memory().AsInt64()
	pjResource := pj.(*SchedulableNode).GetAllocatableResources()
	pjResourceV, _ := pjResource.Memory().AsInt64()
	return piResourceV > pjResourceV
}

// we want to use less used node, so, smaller value are put ahead in sorted list
func NodeLeastLastUseSortFunc(pi, pj interface{}) bool {
	piLastUsed := pi.(*SchedulableNode).LastUsed.UnixNano()
	pjLastUsed := pj.(*SchedulableNode).LastUsed.UnixNano()
	return piLastUsed < pjLastUsed
}

func NodeNameKeyFunc(pj interface{}) string {
	return pj.(*SchedulableNode).NodeIdentifier
}

func BuildNodeSortingFunc(sortingMethod NodeSortingMethod) collection.LessFunc {
	switch sortingMethod {
	case NodeSortingMethodMoreMemory:
		return NodeHasMoreMemorySortFunc
	case NodeSortingMethodLessLastUse:
		return NodeLeastLastUseSortFunc
	default:
		return NodeLeastLastUseSortFunc
	}
}

type SchedulePolicy struct {
	NumOfEvaluatedNodes int
	BackoffDuration     time.Duration
	NodeSortingMethod   NodeSortingMethod
}

// Model scheduler group available nodes into multiple groups(100 node a group by default),
// and do parallel scheduling on node groups, one group work for a model only at a time,
// if can not find available node in a group, model scheduler move this model to next group
// node group is rebuild when there are new nodes join or leave
type nodeGroup struct {
	name          string
	mu            sync.Mutex
	nodes         []*SchedulableNode
	scheduler     *ModelScheduler
	sortingMethod NodeSortingMethod
}

// use copy on write to build pool.sortedNodes to avoid concurrent map iteration and modification,
// sorting method are like free memory or models on node
// scheduler use this list to find a number nodes from this list to schedule models,
// when models are assigned to nodes or terminated from nodes, pool.sortedNodes are not sorted again,
// this list need to resort, but do not want to resort every time since cow is expensive,
// scheduler control when a resort is needed
func (cps *nodeGroup) sortNodes() {
	nodes := []*SchedulableNode{}
	for _, v := range cps.nodes {
		nodes = append(nodes, v)
	}
	sortedNodes := &SortedNodes{
		nodes:    nodes,
		lessFunc: BuildNodeSortingFunc(cps.sortingMethod),
	}
	sort.Sort(sortedNodes)
	cps.nodes = sortedNodes.nodes
}

func (cps *nodeGroup) scheduleModel(model *types.Model) error {
	// lock to avoid overcommit, only one model can can be scheduled at one time
	cps.mu.Lock()
	defer cps.mu.Unlock()
	return cps.scheduler.scheduleModel(model, cps.nodes)
}

// bind model to node via nodeAgentClient, add substract model requested resource from node resource list,
// it substract resource earlier to avoid duplicate use, if any error, unbindNode should release it
func (ms *ModelScheduler) bindNode(snode *SchedulableNode, model *types.Model) error {
	modelName := model.Name
	nodeId := snode.NodeIdentifier
	ms.log.Info("Bind model to node", "namespace", model.Namespace, "name", model.Name, "node", nodeId, "available resource", snode.GetAllocatableResources())

	resourceList := util.GetModelResourceList(model)
	snode.AdmitOccupiedResourceList(resourceList)
	// set model status

	// call nodeagent to start model
	err := ms.nodeManager.LoadModel(nodeId, model)
	if err != nil {
		ms.log.Error(err, "Failed to bind model, reschedule", "node", nodeId, "model", modelName)
		return err
	}
	snode.LastUsed = time.Now()

	return nil
}

// remove model from node resource list
func (ms *ModelScheduler) unbindNode(node *SchedulableNode, model *types.Model) {
	resourceList := util.GetModelResourceList(model)
	node.ReleaseOccupiedResourceList(resourceList)
	// reset model status
}

func (ms *ModelScheduler) scheduleModel(model *types.Model, candidateNodes []*SchedulableNode) error {
	st := time.Now().UnixMicro()
	defer func() {
		et := time.Now().UnixMicro()
		ms.log.Info("Done schedule model", "model", model.Uid, "name", model.Name, "took-micro", et-st)
	}()

	availableNodes := []*SchedulableNode{}
	conditions := CalculateScheduleConditions(ms.scheduleConditionBuilders, model)
	for _, node := range candidateNodes {
		allocatedResources := node.GetAllocatableResources()
		goodNode := true
		for _, cond := range conditions {
			fmt.Println("wenjian check ", "condition", cond)
			goodNode = goodNode && cond.Apply(node, &allocatedResources)
			if !goodNode {
				break
			}
		}
		if goodNode {
			availableNodes = append(availableNodes, node)
		}
	}

	if len(availableNodes) == 0 {
		return InsufficientResourceError
	} else {
		// sort candidates to use first one,
		sortedNodes := &SortedNodes{
			nodes:    availableNodes,
			lessFunc: BuildNodeSortingFunc(NodeSortingMethodLessLastUse),
		}
		sort.Sort(sortedNodes)

		var bindError error
		for _, node := range sortedNodes.nodes {
			bindError = ms.bindNode(node, model)
			if bindError != nil {
				ms.unbindNode(node, model)
				continue
			}
			break
		}
		if bindError != nil {
			return ModelBindToNodeError
		}
	}

	return nil
}

func (ms *ModelScheduler) updateNodePool(nodeEvent *types.NodeEvent) *SchedulableNode {
	ms.log.Info("Received node event, updating node pool", "event", nodeEvent)
	v1node := nodeEvent.Node
	eventType := nodeEvent.EventType
	nodeIdentifier := v1node.NodeIdentifer
	if eventType == types.Delete {
		ms.log.Info("deleting a Schedulable node", "node", nodeIdentifier)
		ms.nodePool.DeleteNode(nodeIdentifier)
		return nil
	} else {
		if snode := ms.nodePool.GetNode(nodeIdentifier); snode != nil {
			snode.LastSeen = time.Now()
			if !util.IsNodeRunning(v1node) {
				ms.log.Info("deleting a Schedulable node", "node", nodeIdentifier)
				ms.nodePool.DeleteNode(nodeIdentifier)
			}
			return snode
		} else {
			// only add ready node into scheduleable node list
			if util.IsNodeRunning(v1node) {
				snode := &SchedulableNode{
					mu:                      sync.Mutex{},
					NodeIdentifier:          nodeIdentifier,
					Node:                    v1node,
					LastSeen:                time.Now(),
					LastUsed:                time.Now(),
					ResourceList:            GetNodeAllocatableResourceList(v1node),
					PreOccupiedResourceList: v1.ResourceList{},
				}
				ms.log.Info("adding a Schedulable node", "node", nodeIdentifier, "resource", snode.ResourceList)
				ms.nodePool.AddNode(nodeIdentifier, snode)
			}
			return snode
		}
	}
}

func (ms *ModelScheduler) updateOccupiedResourceList(snode *SchedulableNode, model *types.Model, eventType types.ResourceEventType) {
	switch eventType {
	case types.Delete:
		resourceList := util.GetModelResourceList(model)
		snode.ReleaseOccupiedResourceList(resourceList)
	case types.Add:
		resourceList := util.GetModelResourceList(model)
		snode.AdmitOccupiedResourceList(resourceList)
	}
}

func (ms *ModelScheduler) printScheduleSummary() {
	activeNum, retryNum := ms.scheduleQueue.Length()
	ms.log.Info("Scheduler summary", "active queue length", activeNum, "backoff queue length", retryNum, "available nodes", ms.nodePool.size(), "node groups", len(ms.nodeGroups))
	// ps.nodePool.printSummary()
}

func (ms *ModelScheduler) initializeNodeGroups() {
	// get initial nodes
	nodes := ms.nodeManager.List()
	for _, n := range nodes {
		ms.updateNodePool(&types.NodeEvent{
			EventType: types.Add,
			Node:      n,
		})
	}

	ms.partitionNodeGroups()
}

func (ms *ModelScheduler) partitionNodeGroups() {
	// TODO shuffle
	ms.log.Info("partitionNodeGroups")
	numOfNodesPerScheduler := ms.policy.NumOfEvaluatedNodes
	nodeGroups := []*nodeGroup{}
	allNodes := ms.nodePool.GetNodes()
	numOfSchedulers := int(math.Ceil(float64(len(allNodes)) / float64(numOfNodesPerScheduler)))
	for i := 0; i < numOfSchedulers; i++ {
		nodes := allNodes[i*numOfNodesPerScheduler : int(math.Min(float64((i+1)*numOfNodesPerScheduler), float64(len(allNodes))))]
		cs := &nodeGroup{
			name:          fmt.Sprintf("node_group_%d", i),
			mu:            sync.Mutex{},
			nodes:         nodes,
			scheduler:     ms,
			sortingMethod: ms.policy.NodeSortingMethod,
		}
		nodeGroups = append(nodeGroups, cs)
	}
	ms.nodeGroups = nodeGroups
}

// scheduleLoop poll model from scheduler queue and loop node groups to find a available node for model
// scheduleLoop poll multiple models according num of node groups and assign a model to a node group and schedule model in parallel using goroutines.
// and if a model can not find node in a node group, go to next node group until all groups are tried
// if model is not scheduled, put it back to queue for backoff retry
func (ms *ModelScheduler) scheduleLoop() {
	for {
		if ms.stop {
			break
		} else {
			nodeGroups := ms.nodeGroups
			if len(nodeGroups) == 0 {
				time.Sleep(200 * time.Millisecond)
				continue
			}

			// poll at most num of scheduler of models, schdule them in parallel
			models := []*types.Model{}
			numOfScheduler := len(nodeGroups)
			for i := 0; i < numOfScheduler; i++ {
				model := ms.scheduleQueue.NextModel()
				models = append(models, model)
				if !ms.scheduleQueue.HasMore() {
					break
				}
			}
			wg := sync.WaitGroup{}
			for _, model := range models {
				rand.Seed(time.Now().UnixNano())
				i := rand.Intn(len(nodeGroups))
				wg.Add(1)
				go func(model *types.Model, index int) {
					var schedErr error
					for i := 0; i < numOfScheduler; i++ {
						nodeGroup := nodeGroups[(index+i)%numOfScheduler]
						schedErr = nodeGroup.scheduleModel(model)
						if schedErr == nil {
							break
						}
					}
					if schedErr != nil {
						ms.log.Error(schedErr, "Can not find node for model, come back later", "model", util.Name(model), "required resource", util.GetModelResourceList(model))
						ms.scheduleQueue.BackoffModel(model, ms.policy.BackoffDuration)
					}
					wg.Done()
				}(model, i)
			}
		}
	}
}

func (ms *ModelScheduler) handleNodeUpdates() {
	ticker := time.NewTicker(DefaultBackoffRetryDuration)
	defer func() {
		ms.stop = true
		close(ms.nodeInfoChannel)
		ticker.Stop()
	}()

	// listen model and node updates
	for {
		select {
		case <-ms.ctx.Done():
			ms.stop = true
			return
		case update := <-ms.nodeInfoChannel:
			oldSize := ms.nodePool.size()
			ms.updateNodePool(update)
			newSize := ms.nodePool.size()
			if oldSize != newSize {
				// reinitialization is expensive, but it's ok for now since it only happen when node pool size changed
				ms.partitionNodeGroups()
			}
		case <-ticker.C:
			ms.printScheduleSummary()
			// move item from backoff queue to active queue if item exceed cool down time
			if ms.scheduleQueue.backoffRetryQueue.queue.Len() > 0 {
				ms.scheduleQueue.ReviveBackoffItem()
			}
		}
	}
}

// 1/ listen to node monitor, maintain node resource statis
// 2/ poll from schedule queue and find a node for model
func (ms *ModelScheduler) Run(ctx context.Context) {
	ms.log.Info("Starting modelctl model scheduler")
	ms.ctx = ctx
	ms.nodeManager.Suscribe("ModelScheduler", ms.nodeInfoChannel)
	ms.initializeNodeGroups()

	// receive model and node update to update scheduleable node resource and condition
	go ms.handleNodeUpdates()

	// schedule loop
	go ms.scheduleLoop()

	for {
		select {
		case <-ctx.Done():
			ms.log.Info("Model scheduler exit")
			ms.stop = true
			break
		}
	}
}

func NewScheduler(nodeManager modelctl.NodeManagerInterface) *ModelScheduler {
	return &ModelScheduler{
		log:             ctrl.Log.WithName("ModelCtl").WithName("ModelScheduler"),
		stop:            false,
		nodeInfoChannel: make(chan *types.NodeEvent, 100),
		nodeManager:     nodeManager,
		scheduleQueue:   NewScheduleQueue(),
		nodePool: &SchedulableNodePool{
			mu:    sync.RWMutex{},
			nodes: map[string]*SchedulableNode{},
		},
		scheduleConditionBuilders: []ConditionBuildFunc{
			ModelGPUCondition,
			ModelCPUCondition,
			ModelMemoryCondition,
		},
		policy: &SchedulePolicy{
			NumOfEvaluatedNodes: 100,
			BackoffDuration:     DefaultBackoffRetryDuration,
			NodeSortingMethod:   NodeSortingMethodMoreMemory,
		},
		nodeGroups: []*nodeGroup{},
	}
}
