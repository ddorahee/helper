package utils

import (
	"sync"
	"time"
)

// TimerManager는 타이머 기능을 관리합니다
type TimerManager struct {
	Running     bool
	StartTime   time.Time
	ElapsedTime time.Duration
	Mutex       sync.Mutex
}

// NewTimerManager는 새로운 타이머 관리자를 생성합니다
func NewTimerManager() *TimerManager {
	return &TimerManager{
		Running:     false,
		ElapsedTime: 0,
		Mutex:       sync.Mutex{},
	}
}

// Start는 타이머를 시작합니다
func (tm *TimerManager) Start() {
	tm.Mutex.Lock()
	defer tm.Mutex.Unlock()
	tm.StartTime = time.Now()
	tm.Running = true
}

// Stop은 타이머를 중지하고 경과 시간을 저장합니다
func (tm *TimerManager) Stop() {
	tm.Mutex.Lock()
	defer tm.Mutex.Unlock()
	if tm.Running {
		tm.ElapsedTime += time.Since(tm.StartTime)
		tm.Running = false
	}
}

// Reset은 타이머를 초기화합니다
func (tm *TimerManager) Reset() {
	tm.Mutex.Lock()
	defer tm.Mutex.Unlock()
	tm.ElapsedTime = 0
	tm.Running = false
}

// IsRunning은 타이머가 실행 중인지 확인합니다
func (tm *TimerManager) IsRunning() bool {
	tm.Mutex.Lock()
	defer tm.Mutex.Unlock()
	return tm.Running
}