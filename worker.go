package grpool

import (
	"time"
)

var (
	closeChan chan struct{}
)

func Init() {
	closeChan = make(chan struct{})
	close(closeChan)
}

type worker struct {
	pool    *Pool
	created time.Time
	sleep   time.Time
	jobs    chan Job
	done    chan struct{}
}

func (w *worker) run() {
	for {
		select {
		case <-w.done:
			return
		case job := <-w.jobs:
			job()
			w.sleep = time.Now()
			w.pool.idleSignal <- w
		}
	}
}

func (w *worker) release() {
	w.done = closeChan
}
