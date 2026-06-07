package automation

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// RotationScheduler 자동사냥 예약 시작 관리자
// KST 기준 오늘 HH:MM 시각에 콜백 1회 실행 (1회성 예약)
type RotationScheduler struct {
	mu         sync.Mutex
	target     time.Time
	stopChan   chan struct{}
	running    bool
	startFn    func() error // 시작 콜백
	logFn      func(string)
}

// NewRotationScheduler 새로운 예약 매니저 생성
func NewRotationScheduler(startFn func() error, logFn func(string)) *RotationScheduler {
	return &RotationScheduler{
		startFn: startFn,
		logFn:   logFn,
	}
}

func (s *RotationScheduler) log(msg string) {
	log.Printf("[예약] %s", msg)
	if s.logFn != nil {
		s.logFn(msg)
	}
}

// kstLocation KST 시간대
func kstLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		// 폴백: UTC+9
		return time.FixedZone("KST", 9*3600)
	}
	return loc
}

// ScheduleAt KST 기준 오늘 HH:MM 시각으로 예약 (이미 지났으면 에러)
func (s *RotationScheduler) ScheduleAt(timeStr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		// 기존 예약 취소 후 새로 설정
		close(s.stopChan)
		s.running = false
	}

	loc := kstLocation()
	now := time.Now().In(loc)

	timeStr = strings.TrimSpace(timeStr)
	var hour, min int
	if _, err := fmt.Sscanf(timeStr, "%d:%d", &hour, &min); err != nil {
		return fmt.Errorf("시간 형식 오류 ('%s'): %v", timeStr, err)
	}
	if hour < 0 || hour > 23 || min < 0 || min > 59 {
		return fmt.Errorf("유효하지 않은 시간: '%s'", timeStr)
	}

	target := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, loc)
	s.log(fmt.Sprintf("ScheduleAt 호출: 입력='%s' → target=%s, 현재 KST=%s",
		timeStr, target.Format("2006-01-02 15:04:05 MST"), now.Format("2006-01-02 15:04:05 MST")))

	if !target.After(now) {
		return fmt.Errorf("이미 지난 시각입니다 (입력=%s, 현재 KST=%s)",
			timeStr, now.Format("15:04:05"))
	}

	dur := time.Until(target)

	s.target = target
	s.stopChan = make(chan struct{})
	s.running = true

	s.log(fmt.Sprintf("예약 설정 완료: target=%s (남은시간 %s)",
		target.Format("15:04"), dur.Round(time.Second)))

	stopChan := s.stopChan
	go s.run(stopChan)
	return nil
}

func (s *RotationScheduler) run(stopChan chan struct{}) {
	target := s.target
	s.log(fmt.Sprintf("goroutine 시작: target=%s, 초기 남은시간=%s",
		target.Format("15:04:05 MST"), time.Until(target).Round(time.Second)))
	tickCount := 0
	for {
		remaining := time.Until(target)
		if remaining <= 0 {
			s.log(fmt.Sprintf("예약 시각 도달 (target=%s, now=%s, remaining=%v) → 자동사냥 시작",
				target.Format("15:04:05"), time.Now().In(kstLocation()).Format("15:04:05"), remaining))
			if s.startFn != nil {
				if err := s.startFn(); err != nil {
					s.log(fmt.Sprintf("자동사냥 시작 실패: %v", err))
				}
			}
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
			return
		}

		// 매 30초마다 진단 로그
		tickCount++
		if tickCount%30 == 1 {
			s.log(fmt.Sprintf("대기 중: 남은시간=%s, target=%s",
				remaining.Round(time.Second), target.Format("15:04:05")))
		}

		// 짧은 인터벌로 대기 (취소 응답성)
		wait := remaining
		if wait > time.Second {
			wait = time.Second
		}
		select {
		case <-stopChan:
			s.log("예약 취소됨")
			return
		case <-time.After(wait):
		}
	}
}

// Cancel 예약 취소
func (s *RotationScheduler) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		close(s.stopChan)
		s.running = false
	}
}

// Status 현재 예약 상태 반환
func (s *RotationScheduler) Status() (running bool, target time.Time, remaining time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return false, time.Time{}, 0
	}
	return true, s.target, time.Until(s.target)
}
