package utils

import (
	"fmt"
	"log"
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

	log.Printf("타이머 시작: %v", tm.StartTime.Format("2006-01-02 15:04:05"))
}

// Stop은 타이머를 중지하고 경과 시간을 저장합니다
func (tm *TimerManager) Stop() {
	tm.Mutex.Lock()
	defer tm.Mutex.Unlock()

	if tm.Running {
		tm.ElapsedTime += time.Since(tm.StartTime)
		tm.Running = false

		log.Printf("타이머 중지: 총 경과 시간 %v", tm.ElapsedTime)
	}
}

// Reset은 타이머를 초기화합니다
func (tm *TimerManager) Reset() {
	tm.Mutex.Lock()
	defer tm.Mutex.Unlock()

	tm.ElapsedTime = 0
	tm.Running = false

	log.Println("타이머 재설정")
}

// IsRunning은 타이머가 실행 중인지 확인합니다
func (tm *TimerManager) IsRunning() bool {
	tm.Mutex.Lock()
	defer tm.Mutex.Unlock()
	return tm.Running
}

// GetStartTime은 타이머 시작 시간을 반환합니다
func (tm *TimerManager) GetStartTime() time.Time {
	tm.Mutex.Lock()
	defer tm.Mutex.Unlock()
	return tm.StartTime
}

// GetElapsedTime은 현재까지의 경과 시간을 반환합니다
func (tm *TimerManager) GetElapsedTime() time.Duration {
	tm.Mutex.Lock()
	defer tm.Mutex.Unlock()

	if tm.Running {
		return tm.ElapsedTime + time.Since(tm.StartTime)
	}
	return tm.ElapsedTime
}

// GetElapsedTimeString은 경과 시간을 문자열로 반환합니다
func (tm *TimerManager) GetElapsedTimeString() string {
	elapsed := tm.GetElapsedTime()
	hours := int(elapsed.Hours())
	minutes := int(elapsed.Minutes()) % 60
	seconds := int(elapsed.Seconds()) % 60

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}
