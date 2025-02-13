package sync

import (
	"context"
	"golang.org/x/sync/semaphore"
	"rhyus-golang/common"
	"sync"
)

type task struct {
	ctx    context.Context
	cancel context.CancelFunc
}

type tasks struct {
	ctx     context.Context
	cancel  context.CancelFunc
	name    string
	startFn StartTaskFn
	taskNum int
	tasks   []*task
}

func newTasks(ctx context.Context, name string, startFn StartTaskFn, taskNum int) *tasks {
	ctx, cancel := context.WithCancel(ctx)
	return &tasks{
		ctx:     ctx,
		cancel:  cancel,
		name:    name,
		startFn: startFn,
		taskNum: taskNum,
		tasks:   make([]*task, 0, taskNum),
	}
}

func (t *tasks) startTasks() {
	go func() {
		sem := semaphore.NewWeighted(int64(t.taskNum))
		for {
			select {
			case <-t.ctx.Done():
				return
			default:
				t.startTask(sem)
			}
		}
	}()
}
func (t *tasks) startTask(sem *semaphore.Weighted) {
	if err := sem.Acquire(t.ctx, 1); err != nil {
		common.Log.Error("Failed to acquire semaphore: %v", err)
		return
	}

	ctx, cancel := context.WithCancel(t.ctx)
	t.tasks = append(t.tasks, &task{
		ctx:    ctx,
		cancel: cancel,
	})
	go func(ctx context.Context) {
		defer func() {
			if err := recover(); err != nil {
				common.Log.Error("Recovered from panic in %s goroutine: %v\n", t.name, err)
			}
		}()
		common.Log.Debug("startTasks task %s\n", t.name)
		err := t.startFn(ctx)
		if err != nil {
			common.Log.Error("Failed to startTasks task: %v", err)
		}
		sem.Release(1)
	}(ctx)
}

type StartTaskFn func(ctx context.Context) (err error)

type Pool struct {
	ctx    context.Context
	cancel context.CancelFunc
	tasks  map[string]*tasks
	m      sync.RWMutex
}

func NewPool(ctx context.Context) *Pool {
	ctx, cancel := context.WithCancel(ctx)
	tasks := make(map[string]*tasks)
	return &Pool{ctx: ctx, cancel: cancel, tasks: tasks, m: sync.RWMutex{}}
}

func (p *Pool) SubmitTasks(name string, workNum int, startFn StartTaskFn) {
	p.m.Lock()
	defer p.m.Unlock()
	if _, ok := p.tasks[name]; ok {
		common.Log.Debug("task %s already exists", name)
		return
	}
	p.tasks[name] = newTasks(p.ctx, name, startFn, workNum)
	p.tasks[name].startTasks()
}

func (p *Pool) SubmitTask(name string, startFn StartTaskFn) {
	p.SubmitTasks(name, 1, startFn)
}
