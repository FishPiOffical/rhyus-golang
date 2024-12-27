package common

import (
	"sync"
)

type WorkerPool struct {
	taskQueue chan func()
	workCount int
	wg        sync.WaitGroup
}

func (g *GuPool) NewWorkerPool(workCount int) *WorkerPool {
	return &WorkerPool{
		taskQueue: make(chan func(), workCount),
		workCount: workCount,
	}
}

func (wp *WorkerPool) Start() {
	for i := 0; i < wp.workCount; i++ {
		go wp.worker()
	}
}

func (wp *WorkerPool) worker() {
	defer func() {
		if err := recover(); err != nil {
			Log.Error("Recovered from panic in goroutine: %v\n", err)
		}
	}()
	for task := range wp.taskQueue {
		task()
		wp.wg.Done()
	}
}

func (wp *WorkerPool) AddTask(task func()) {
	wp.wg.Add(1)
	wp.taskQueue <- task
}

func (wp *WorkerPool) Wait() {
	wp.wg.Wait()
}

func (wp *WorkerPool) Close() {
	close(wp.taskQueue)
}
