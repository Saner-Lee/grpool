package grpool

import (
	"container/list"
	"errors"
	"sync"
	"time"
)

type Job func()

type Pool struct {
	Max     int
	MaxIdle int // 必须设置，否则可能死锁

	Idle time.Duration
	Live time.Duration

	Wait bool

	idleSignal chan *worker

	mu       sync.Mutex
	idlelist *list.List
	cnt      int
	idleCnt  int
	running  bool
}

func (p *Pool) lazyinit() {
	if p.running {
		return
	}
	p.idlelist = list.New()
	p.idleSignal = make(chan *worker, p.MaxIdle)
	p.running = true
}

// 添加任务
func (p *Pool) Submit(job Job) error {
	p.lazyinit()

	p.mu.Lock()
	defer p.mu.Unlock()
	// 删除老旧的worker
	p.kill()
EXEC:
	// 存在空闲worker, 执行job
	w, ok := p.getWorker()
	if ok != false {
		w.jobs <- job
		p.idlelist.Remove(p.idlelist.Front())
		p.idleCnt--
		return nil
	}

	// 不存在空闲worker
	// 1. 数量未达到上限
	if p.cnt < p.Max {
		now := time.Now()
		w := &worker{
			pool:    p,
			created: now,
			jobs:    make(chan Job, 1),
		}
		w.jobs <- job
		p.cnt++
		// pool写goroutine，而不是worker起，原因是这样可以在关闭池的时候做到更好的控制
		// 比如这里可以将包装匿名函数，在匿名函数中加上sync.WaitGroup，那么就Close就可以做到等到所有worker完工后才退出
		go w.run()
		return nil
	}

	// 2. 数量达到上限
	if !p.Wait {
		return errors.New("no worker")
	}
	w = <-p.idleSignal
	p.idleCnt++
	p.idlelist.PushBack(w)

	p.keepMaxIdle()

	goto EXEC
}

// 删除闲置时间过长的worker
func (p *Pool) kill() {
	var (
		cur  = p.idlelist.Front()
		next *list.Element
	)
	for cur != nil {
		next = cur.Next()

		// 释放无用连接
		now := time.Now()
		w := cur.Value.(*worker)
		if !w.sleep.Add(p.Idle).Before(now) {
			p.idlelist.Remove(cur)
			p.cnt--
		}

		cur = next
	}
}

// 保存空闲的数量维持在MaxIdle
func (p *Pool) keepMaxIdle() {
	var (
		cur  = p.idlelist.Front()
		next *list.Element
	)

	for {
		select {
		case w := <-p.idleSignal:
			p.idlelist.PushBack(w)
			p.idleCnt++
		default:
			for p.idlelist.Len() > p.MaxIdle {
				next = cur.Next()
				p.idlelist.Remove(cur)
				cur = next
			}
			return
		}
	}
}

// 删除存在时间过长的worker，返回第一个未在移除条件内worker
func (p *Pool) getWorker() (w *worker, get bool) {
	var (
		cur  = p.idlelist.Front()
		next *list.Element
	)
	for cur != nil {
		next = cur.Next()

		now := time.Now()
		w := cur.Value.(*worker)
		if !w.created.Add(p.Live).Before(now) {
			p.idlelist.Remove(cur)
			p.cnt--
		} else {
			return w, true
		}

		cur = next
	}

	return
}

// 释放池中所有的worker, 仅仅是发出退出信号，如果需要等待worker完全退出需要在发起goroutine时使用sync.WaitGroup控制
func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for e := p.idlelist.Front(); e != nil; e = e.Next() {
		w := e.Value.(*worker)
		w.release()
	}
	p.cnt = p.cnt - p.idlelist.Len()
	p.idlelist = nil

	for p.cnt != 0 {
		w := <-p.idleSignal
		w.release()
		p.cnt--
	}
	p.idleCnt = 0
	p.idleSignal = nil
	p.running = false
}
