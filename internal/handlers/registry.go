package handlers

import (
	"sync"
	"sync/atomic"
)

type JobStatus string

const (
	StatusRunning JobStatus = "running"
	StatusDone    JobStatus = "done"
	StatusError   JobStatus = "error"
)

type JobState struct {
	Status       atomic.Value // holds JobStatus string
	PagesCrawled atomic.Int64
	ErrorMsg     atomic.Value // holds string
}

func newJobState() *JobState {
	s := &JobState{}
	s.Status.Store(StatusRunning)
	s.ErrorMsg.Store("")
	return s
}

type JobRegistry struct {
	m sync.Map // map[string]*JobState
}

func (r *JobRegistry) Register(id string) *JobState {
	s := newJobState()
	r.m.Store(id, s)
	return s
}

func (r *JobRegistry) Get(id string) (*JobState, bool) {
	v, ok := r.m.Load(id)
	if !ok {
		return nil, false
	}
	return v.(*JobState), true
}

func (r *JobRegistry) Delete(id string) {
	r.m.Delete(id)
}
