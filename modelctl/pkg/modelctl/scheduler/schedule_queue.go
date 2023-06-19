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
	"sync"
	"time"

	"container/heap"
)

type ScheduleItem struct {
	model       *types.Model
	requestTime time.Time
}

func ModelRequestTimeless(pi, pj interface{}) bool {
	return pi.(*ScheduleItem).requestTime.UnixMilli() < pj.(*ScheduleItem).requestTime.UnixMilli()
}

func ModelId(model interface{}) string {
	return string(model.(*ScheduleItem).model.Uid)
}

// A schedulePriorityQueue implements heap.Interface and holds Items.
type schedulePriorityQueue struct {
	mu    sync.RWMutex
	queue *collection.PriorityQueue
}

func (pq *schedulePriorityQueue) AddModel(model *types.Model, duration time.Duration) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	item := &ScheduleItem{
		model:       model,
		requestTime: time.Now().Add(duration),
	}
	heap.Push(pq.queue, item)
}

// RemoveModel remove model from qeueue with, and return removed model spec
func (pq *schedulePriorityQueue) RemoveModel(model *types.Model) *types.Model {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	if index, item := pq.queue.Get(string(model.Uid)); item != nil {
		heap.Remove(pq.queue, index)
		return item.(*ScheduleItem).model
	}
	return nil
}

func (pq *schedulePriorityQueue) GetModel(name string) *types.Model {
	pq.mu.RLock()
	if _, item := pq.queue.Get(name); item != nil {
		pq.mu.RUnlock()
		return item.(*ScheduleItem).model
	} else {
		pq.mu.RUnlock()
		return nil
	}
}

func (pq *schedulePriorityQueue) PeakModel() *types.Model {
	pq.mu.RLock()
	ob := pq.queue.Peak()
	if ob == nil {
		pq.mu.RUnlock()
		return nil
	}
	pq.mu.RUnlock()
	return ob.(*ScheduleItem).model
}

func (pq *schedulePriorityQueue) NextModel() *types.Model {
	item := pq.nextItem()
	if item == nil {
		return nil
	}
	return item.model
}

func (pq *schedulePriorityQueue) nextItem() *ScheduleItem {
	pq.mu.Lock()
	ob := pq.queue.Peak()
	if ob == nil {
		pq.mu.Unlock()
		return nil
	}
	ob = heap.Pop(pq.queue)
	if ob == nil {
		pq.mu.Unlock()
		return nil
	}
	pq.mu.Unlock()
	return ob.(*ScheduleItem)
}

type ModelScheduleQueue struct {
	stop              bool
	c                 *sync.Cond
	activeQueue       schedulePriorityQueue
	backoffRetryQueue schedulePriorityQueue
}

// Stop clear queue, signal
func (q *ModelScheduleQueue) Stop() {
	q.stop = true
	q.c.L.Lock()
	q.c.Broadcast()
	q.c.L.Unlock()
}

// Length implements ModelScheduleQueue
func (q *ModelScheduleQueue) Length() (int, int) {
	return q.activeQueue.queue.Len(), q.backoffRetryQueue.queue.Len()
}

func (ps *ModelScheduleQueue) ReviveBackoffItem() {
	models := []*types.Model{}
	timeNow := time.Now()
	for {
		item := ps.backoffRetryQueue.nextItem()
		if item == nil {
			break
		}
		if item.requestTime.Before(timeNow) {
			models = append(models, item.model)
		} else {
			// put it back
			ps.backoffRetryQueue.AddModel(item.model, item.requestTime.Sub(timeNow))
			break
		}
	}

	for _, v := range models {
		ps.AddModel(v, 0)
	}
}

func (q *ModelScheduleQueue) HasMore() bool {
	return q.activeQueue.PeakModel() != nil
}

// NextModel remove a model from scheduleQueue's active queue, it block until it get a model or schedule queue shutdown
func (q *ModelScheduleQueue) NextModel() (model *types.Model) {
	q.c.L.Lock()
	for {
		if q.stop {
			break
		}
		model = q.activeQueue.NextModel()
		if model == nil {
			q.c.Wait()
		} else {
			break
		}
	}
	q.c.L.Unlock()
	return model
}

// BackoffModel move a model from active queue into retry queue with a backoff duration
func (q *ModelScheduleQueue) BackoffModel(model *types.Model, backoffDuration time.Duration) {
	q.activeQueue.RemoveModel(model)
	q.backoffRetryQueue.AddModel(model, backoffDuration)
}

// AddModel add model into active queue, if there is already a copy with same name, remove old copy and add new copy,
func (q *ModelScheduleQueue) AddModel(model *types.Model, backoff time.Duration) {
	// remove old version and add new version to active queue
	if oldcopy := q.backoffRetryQueue.RemoveModel(model); oldcopy != nil {
		q.activeQueue.AddModel(model, backoff)
	} else {
		// remove old version and add new version to active queue
		q.activeQueue.RemoveModel(model)
		q.activeQueue.AddModel(model, backoff)
	}
	q.c.L.Lock()
	q.c.Broadcast()
	q.c.L.Unlock()
}

// RemoveModel remove model from active and retry queue
func (q *ModelScheduleQueue) RemoveModel(model *types.Model) *types.Model {
	if oldcopy := q.activeQueue.RemoveModel(model); oldcopy != nil {
		return oldcopy
	}

	if oldcopy := q.backoffRetryQueue.RemoveModel(model); oldcopy != nil {
		return oldcopy
	}

	return nil
}

func newSchedulePriorityQueue() *schedulePriorityQueue {
	return &schedulePriorityQueue{
		mu:    sync.RWMutex{},
		queue: collection.NewPriorityQueue(ModelRequestTimeless, ModelId),
	}
}

func NewScheduleQueue() *ModelScheduleQueue {
	return &ModelScheduleQueue{
		stop:              false,
		c:                 sync.NewCond(&sync.Mutex{}),
		activeQueue:       *newSchedulePriorityQueue(),
		backoffRetryQueue: *newSchedulePriorityQueue(),
	}
}
