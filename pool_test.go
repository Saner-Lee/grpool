package grpool

import (
	"strings"
	"testing"
	"time"
)

var (
	p = &Pool{
		Max:     2,
		MaxIdle: 2,
		Idle:    time.Second * 60,
		Live:    time.Second * 120,
		Wait:    false,
	}
)

func TestCnt(t *testing.T) {
	defer p.Close()

	var s string

	f := func() {
		time.Sleep(time.Second)
		s = s + "hello"
	}

	p.Submit(f)
	p.Submit(f)

	err := p.Submit(f)
	if strings.Index(err.Error(), "no worker") != -1 {
		return
	}
	t.Fatal("num is fault")
}

func TestWaitResult(t *testing.T) {
	defer func() {
		p.Wait = false
		p.Close()
	}()
	p.Wait = true

	var s string

	f := func() {
		time.Sleep(time.Second)
		s = s + "hello"
	}

	p.Submit(f)
	p.Submit(f)
	p.Submit(f)

	time.Sleep(time.Second * 2)
	if s == "hellohellohello" {
		return
	}
	t.Fatal("wait submit rst is fault")
}

func TestNoWaitResult(t *testing.T) {
	defer func() {
		p.Close()
	}()

	var s string

	f := func() {
		time.Sleep(time.Second)
		s = s + "hello"
	}

	p.Submit(f)
	p.Submit(f)
	p.Submit(f)

	time.Sleep(time.Second * 2)
	if s == "hellohello" {
		return
	}
	t.Fatal("nowait submit rst is fault")
}
