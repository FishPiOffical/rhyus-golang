package common

import (
	"sync"
	"sync/atomic"
)

type WorkerPool struct {
	taskQueue   chan func()
	workCount   int
	activeCount int64
	addWorker   chan int
	wg          sync.WaitGroup
}

func (g *GuPool) NewWorkerPool(workCount int) *WorkerPool {
	return &WorkerPool{
		taskQueue:   make(chan func(), workCount),
		workCount:   workCount,
		activeCount: 0,
		addWorker:   make(chan int, workCount>>2+1),
		wg:          sync.WaitGroup{},
	}
}

func (wp *WorkerPool) Start() {
	wp.wg.Add(wp.workCount)
	for i := 0; i < wp.workCount; i++ {
		go wp.worker()
	}
	go func() {
		defer func() {
			if err := recover(); err != nil {
				Log.Error("Recovered from panic in goroutine: %v\n", err)
			}
		}()
		for range wp.addWorker {
			wp.wg.Add(1)
			go wp.worker()
		}
	}()
}

func (wp *WorkerPool) worker() {
	defer func() {
		wp.wg.Done()
		atomic.AddInt64(&wp.activeCount, -1)
		if err := recover(); err != nil {
			Log.Error("Recovered from panic in goroutine: %v\n", err)
		}
	}()
	atomic.AddInt64(&wp.activeCount, 1)
	for task := range wp.taskQueue {
		task()
	}
}

func (wp *WorkerPool) AddTask(task func()) {
	wp.taskQueue <- task
}

func (wp *WorkerPool) AddTasks(task func(), workCount int) {
	for i := 0; i < workCount; i++ {
		wp.AddTask(task)
	}
}

func (wp *WorkerPool) Wait() {
	wp.wg.Wait()
}

func (wp *WorkerPool) Close() {
	close(wp.taskQueue)
	close(wp.addWorker)
}

type SteadyWorkerPool struct {
	tasks       []func()
	workCount   int
	activeCount int64
	addWorker   chan int
	wg          sync.WaitGroup
}

func (g *GuPool) NewSteadyWorkerPool(workCount int) *SteadyWorkerPool {
	wp := &SteadyWorkerPool{
		tasks:       make([]func(), 0, workCount),
		workCount:   workCount,
		activeCount: 0,
		addWorker:   make(chan int, workCount>>2+1),
		wg:          sync.WaitGroup{},
	}
	go func() {
		defer func() {
			if err := recover(); err != nil {
				Log.Error("Recovered from panic in goroutine: %v\n", err)
			}
		}()
		for i := range wp.addWorker {
			wp.wg.Add(1)
			go wp.worker(i)
		}
	}()
	return wp
}

func (wp *SteadyWorkerPool) AddTask(task func()) {
	i := len(wp.tasks)
	wp.tasks = append(wp.tasks, task)
	wp.wg.Add(1)
	go wp.worker(i)
}

func (wp *SteadyWorkerPool) AddTasks(task func(), workCount int) {
	for i := 0; i < workCount; i++ {
		wp.AddTask(task)
	}
}

func (wp *SteadyWorkerPool) worker(i int) {
	defer func() {
		wp.wg.Done()
		atomic.AddInt64(&wp.activeCount, -1)
		wp.addWorker <- i
		if err := recover(); err != nil {
			Log.Error("Recovered from panic in goroutine: %v\n", err)
		}
	}()
	atomic.AddInt64(&wp.activeCount, 1)
	wp.tasks[i]()
}

func (wp *SteadyWorkerPool) Wait() {
	wp.wg.Wait()
}

func (wp *SteadyWorkerPool) Close() {
	close(wp.addWorker)
}
