package utils

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// TimerManager는 타이머 기능을 관리합니다 (서버 기반으로 개선)
type TimerManager struct {
	Running        bool
	StartTime      time.Time
	PauseTime      time.Time
	ElapsedTime    time.Duration
	TotalDuration  time.Duration // 총 예정 시간
	RemainingTime  time.Duration // 남은 시간
	Paused         bool
	Mutex          sync.Mutex
	TickerStopChan chan bool
	AutoStopChan   chan bool
	ticker         *time.Ticker
	onTimeUpdate   func(remaining time.Duration) // 시간 업데이트 콜백
	onTimeComplete func()                        // 시간 완료 콜백
}

// NewTimerManager는 새로운 타이머 관리자를 생성합니다
func NewTimerManager() *TimerManager {
	return &TimerManager{
		Running:        false,
		Paused:         false,
		ElapsedTime:    0,
		RemainingTime:  0,
		TotalDuration:  0,
		Mutex:          sync.Mutex{},
		TickerStopChan: make(chan bool, 1),
		AutoStopChan:   make(chan bool, 1),
	}
}

// SetDuration은 타이머의 총 시간을 설정합니다
func (tm *TimerManager) SetDuration(duration time.Duration) {
	tm.Mutex.Lock()
	defer tm.Mutex.Unlock()

	tm.TotalDuration = duration
	tm.RemainingTime = duration

	log.Printf("타이머 시간 설정: %v", duration)
}

// SetTimeUpdateCallback은 시간 업데이트 콜백을 설정합니다
func (tm *TimerManager) SetTimeUpdateCallback(callback func(remaining time.Duration)) {
	tm.Mutex.Lock()
	defer tm.Mutex.Unlock()
	tm.onTimeUpdate = callback
}

// SetTimeCompleteCallback은 시간 완료 콜백을 설정합니다
func (tm *TimerManager) SetTimeCompleteCallback(callback func()) {
	tm.Mutex.Lock()
	defer tm.Mutex.Unlock()
	tm.onTimeComplete = callback
}

// Start는 타이머를 시작합니다 (서버 기반)
func (tm *TimerManager) Start() {
	tm.Mutex.Lock()
	defer tm.Mutex.Unlock()

	if tm.Running {
		log.Printf("타이머가 이미 실행 중입니다")
		return
	}

	now := time.Now()
	tm.StartTime = now
	tm.Running = true
	tm.Paused = false

	// 일시 정지된 상태에서 재시작하는 경우
	if tm.ElapsedTime > 0 {
		log.Printf("타이머 재시작: 경과 시간 %v", tm.ElapsedTime)
	} else {
		// 새로 시작하는 경우
		tm.ElapsedTime = 0
		tm.RemainingTime = tm.TotalDuration
		log.Printf("타이머 새로 시작: 총 시간 %v", tm.TotalDuration)
	}

	// 백그라운드에서 실행되는 서버 기반 타이머 시작
	go tm.runServerTimer()
}

// Stop은 타이머를 중지하고 경과 시간을 저장합니다
func (tm *TimerManager) Stop() {
	tm.Mutex.Lock()
	defer tm.Mutex.Unlock()

	if !tm.Running {
		return
	}

	// 현재까지 경과한 시간 계산
	if !tm.Paused {
		tm.ElapsedTime += time.Since(tm.StartTime)
		tm.RemainingTime = tm.TotalDuration - tm.ElapsedTime
		if tm.RemainingTime < 0 {
			tm.RemainingTime = 0
		}
	}

	tm.Running = false
	tm.Paused = true
	tm.PauseTime = time.Now()

	// 타이머 정지 신호 전송
	select {
	case tm.TickerStopChan <- true:
	default:
	}

	log.Printf("타이머 일시정지: 경과 시간 %v, 남은 시간 %v", tm.ElapsedTime, tm.RemainingTime)
}

// Reset은 타이머를 초기화합니다
func (tm *TimerManager) Reset() {
	tm.Mutex.Lock()
	defer tm.Mutex.Unlock()

	tm.ElapsedTime = 0
	tm.RemainingTime = tm.TotalDuration
	tm.Running = false
	tm.Paused = false

	// 정지 신호 전송
	select {
	case tm.TickerStopChan <- true:
	default:
	}

	log.Printf("타이머 리셋: 총 시간 %v로 초기화", tm.TotalDuration)
}

// runServerTimer는 서버에서 실행되는 정확한 타이머입니다
func (tm *TimerManager) runServerTimer() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("타이머 패닉 복구: %v", r)
		}
	}()

	log.Printf("서버 기반 타이머 시작")

	// 1초마다 업데이트하는 정확한 타이머
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	tm.ticker = ticker
	startTime := time.Now()

	for {
		select {
		case <-tm.TickerStopChan:
			log.Printf("서버 타이머 정지 신호 받음")
			return
		case <-ticker.C:
			tm.Mutex.Lock()

			// 실행 중이 아니면 건너뛰기
			if !tm.Running || tm.Paused {
				tm.Mutex.Unlock()
				continue
			}

			// 정확한 경과 시간 계산
			currentElapsed := tm.ElapsedTime + time.Since(startTime)
			tm.RemainingTime = tm.TotalDuration - currentElapsed

			// 시간이 완료된 경우
			if tm.RemainingTime <= 0 {
				tm.RemainingTime = 0
				tm.Running = false
				tm.ElapsedTime = tm.TotalDuration

				log.Printf("타이머 완료: 총 %v 실행됨", tm.TotalDuration)

				// 완료 콜백 실행
				if tm.onTimeComplete != nil {
					go tm.onTimeComplete()
				}

				tm.Mutex.Unlock()
				return
			}

			// 시간 업데이트 콜백 실행
			if tm.onTimeUpdate != nil {
				remaining := tm.RemainingTime
				tm.Mutex.Unlock()

				// 콜백을 별도 고루틴에서 실행하여 타이머 블로킹 방지
				go tm.onTimeUpdate(remaining)
			} else {
				tm.Mutex.Unlock()
			}
		}
	}
}

// IsRunning은 타이머가 실행 중인지 확인합니다
func (tm *TimerManager) IsRunning() bool {
	tm.Mutex.Lock()
	defer tm.Mutex.Unlock()
	return tm.Running && !tm.Paused
}

// IsPaused는 타이머가 일시정지 상태인지 확인합니다
func (tm *TimerManager) IsPaused() bool {
	tm.Mutex.Lock()
	defer tm.Mutex.Unlock()
	return tm.Paused
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

	if tm.Running && !tm.Paused {
		// 실행 중인 경우 실시간 경과 시간 계산
		return tm.ElapsedTime + time.Since(tm.StartTime)
	}
	return tm.ElapsedTime
}

// GetRemainingTime은 남은 시간을 반환합니다 (정확한 서버 시간 기반)
func (tm *TimerManager) GetRemainingTime() time.Duration {
	tm.Mutex.Lock()
	defer tm.Mutex.Unlock()

	if tm.Running && !tm.Paused {
		// 실행 중인 경우 정확한 남은 시간 계산
		elapsed := tm.ElapsedTime + time.Since(tm.StartTime)
		remaining := tm.TotalDuration - elapsed
		if remaining < 0 {
			remaining = 0
		}
		return remaining
	}

	return tm.RemainingTime
}

// GetRemainingTimeSeconds는 남은 시간을 초 단위로 반환합니다
func (tm *TimerManager) GetRemainingTimeSeconds() int {
	remaining := tm.GetRemainingTime()
	return int(remaining.Seconds())
}

// GetElapsedTimeString은 경과 시간을 문자열로 반환합니다
func (tm *TimerManager) GetElapsedTimeString() string {
	elapsed := tm.GetElapsedTime()
	hours := int(elapsed.Hours())
	minutes := int(elapsed.Minutes()) % 60
	seconds := int(elapsed.Seconds()) % 60

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

// GetRemainingTimeString은 남은 시간을 문자열로 반환합니다
func (tm *TimerManager) GetRemainingTimeString() string {
	remaining := tm.GetRemainingTime()
	hours := int(remaining.Hours())
	minutes := int(remaining.Minutes()) % 60
	seconds := int(remaining.Seconds()) % 60

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

// ForceComplete는 타이머를 강제로 완료시킵니다
func (tm *TimerManager) ForceComplete() {
	tm.Mutex.Lock()
	defer tm.Mutex.Unlock()

	if tm.Running {
		tm.Running = false
		tm.ElapsedTime = tm.TotalDuration
		tm.RemainingTime = 0

		// 정지 신호 전송
		select {
		case tm.TickerStopChan <- true:
		default:
		}

		// 완료 콜백 실행
		if tm.onTimeComplete != nil {
			go tm.onTimeComplete()
		}

		log.Printf("타이머 강제 완료")
	}
}

// GetProgress는 진행률을 0-1 사이의 값으로 반환합니다
func (tm *TimerManager) GetProgress() float64 {
	tm.Mutex.Lock()
	defer tm.Mutex.Unlock()

	if tm.TotalDuration == 0 {
		return 0
	}

	elapsed := tm.GetElapsedTime()
	progress := float64(elapsed) / float64(tm.TotalDuration)

	if progress > 1.0 {
		progress = 1.0
	}

	return progress
}
