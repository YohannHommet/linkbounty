package handlers

import (
	"sync"
	"sync/atomic"

	"linkbounty/internal/database"
)

type JobState struct {
	Status       atomic.Pointer[database.JobStatus]
	PagesCrawled atomic.Int64
	ErrorMsg     atomic.Pointer[string]
}

func newJobState() *JobState {
	s := &JobState{}
	status := database.StatusRunning
	s.Status.Store(&status)
	empty := ""
	s.ErrorMsg.Store(&empty)
	return s
}

func (s *JobState) SetStatus(st database.JobStatus) { s.Status.Store(&st) }
func (s *JobState) GetStatus() database.JobStatus   { return *s.Status.Load() }
func (s *JobState) SetErrorMsg(msg string)          { s.ErrorMsg.Store(&msg) }
func (s *JobState) GetErrorMsg() string             { return *s.ErrorMsg.Load() }

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
