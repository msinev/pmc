package main

import (
	"container/list"
	"sync"
)

var taskQueue *list.List
var taskLock sync.Mutex

type RunningTask struct {
	Status string
}

var tasksRunning map[string]*RunningTask
var taskRunLock sync.Mutex

func initTaskQueue() {
	taskQueue = list.New()
	tasksRunning = make(map[string]*RunningTask, 100)
}

func initFileTypes() {
	FileTypes = map[string]int{
		"log":    1,
		"image":  2,
		"render": 2,
	}

}

func TaskQueueInsert(string2 string) {
	taskLock.Lock()
	defer taskLock.Unlock()
	taskQueue.PushBack(&string2)
}

func TaskQueueRetrive() *string {
	taskLock.Lock()
	defer taskLock.Unlock()
	fe := taskQueue.Front()
	if fe == nil {
		return nil
	}
	val := fe.Value.(*string)
	taskQueue.Remove(fe)
	return val
}
