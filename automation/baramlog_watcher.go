package automation

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// BaramlogWatcher baramlog.com 채팅 로그를 주기적으로 조회해
// 지정한 시련 파티 모집 글이 올라오면 텔레그램으로 알린다.
// 사이트 헬스체크도 겸해서, 연속 실패 시 장애/복구 알림을 보낸다.
//
// 매칭 규칙: 실제 채팅은 "시련왕궁격수구해요", "나타시련 10판", "시련 스투파가실분"
// 처럼 붙여쓰기/어순이 제각각이라 정확 문구 매칭으로는 대부분 놓친다.
// → 공백 제거한 텍스트가 "시련"을 포함하고 + 장소 키워드를 포함하면 매칭.
type BaramlogWatcher struct {
	mu       sync.Mutex
	pollMu   sync.Mutex // poll 직렬화 (수동 sync와 주기 폴링 동시 실행 방지)
	running  bool
	stopChan chan struct{}
	syncChan chan struct{} // 수동 즉시 동기화 신호

	url      string
	interval time.Duration
	client   *http.Client

	lastID int  // 마지막으로 처리한 메시지 id
	primed bool // 첫 조회 완료(과거 메시지 전체 알림 방지)

	// 헬스체크 상태 (HTTP 접속)
	failCount    int
	downAlerted  bool
	lastDownSent time.Time
	lastErr      string

	// 데이터 정체 감지 — 사이트가 200을 주면서도 새 글이 안 올라오는 경우.
	// 접속 실패와는 다른 고장이라 따로 본다. 감시는 계속 돌린다(중지하지 않음).
	newestMsgAt  time.Time // 최신 메시지의 작성 시각
	staleAlerted bool      // 정체 알림을 이미 보냈는지

	// 통계
	totalHits  int
	lastHitAt  time.Time
	lastPollAt time.Time
	recentHits []BaramlogHit // 최근 발견 (최신 우선, 최대 baramlogMaxRecent건)

	sendTelegram func(string) error
	logFunc      func(string)
}

// baramlogTarget 감시 대상 (label = 알림 표기, keys = 장소 키워드)
type baramlogTarget struct {
	label string
	keys  []string
}

// 구체적인 것부터 검사 (나타라자가 나타보다 먼저)
var baramlogTargets = []baramlogTarget{
	{label: "나타라자 시련", keys: []string{"나타라자"}},
	{label: "나타 시련", keys: []string{"나타"}},
	{label: "왕궁 시련", keys: []string{"왕궁"}},
	{label: "스투파 시련", keys: []string{"스투파"}},
}

// BaramlogMessage API 응답의 메시지 1건
type BaramlogMessage struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	Message   string  `json:"message"`
	CreatedAt string  `json:"created_at"`
	TS        float64 `json:"ts"`
}

type baramlogResponse struct {
	Messages []BaramlogMessage `json:"messages"`
}

// baramlogStaleThreshold 이 시간 동안 새 글이 없으면 사이트 고장으로 본다.
// (HTTP는 200이지만 데이터가 갱신되지 않는 상태 — 실제로 11시간 정체한 적 있음)
const baramlogStaleThreshold = 30 * time.Minute

// baramlogMaxRecent 화면에 보관할 최근 발견 건수
const baramlogMaxRecent = 50

// BaramlogHit 발견된 모집 글 1건 (화면 표시용)
type BaramlogHit struct {
	ID        int    `json:"id"`
	Label     string `json:"label"`
	Name      string `json:"name"`
	Message   string `json:"message"`
	CreatedAt string `json:"createdAt"` // 게임 채팅 시각
	FoundAt   string `json:"foundAt"`   // 감지 시각
}

// BaramlogStatus UI 표시용 상태
type BaramlogStatus struct {
	Running    bool   `json:"running"`
	Healthy    bool   `json:"healthy"`
	LastID     int    `json:"lastId"`
	TotalHits  int    `json:"totalHits"`
	LastHitAt  string `json:"lastHitAt"`
	LastPollAt string `json:"lastPollAt"`
	FailCount  int    `json:"failCount"`
	LastError  string `json:"lastError"`
	IntervalS  int           `json:"intervalSeconds"`
	Syncing    bool          `json:"syncing"`
	Stale        bool   `json:"stale"`            // 데이터 정체 여부
	DataAgeMins  int    `json:"dataAgeMinutes"`   // 최신 글이 몇 분 전 것인지
	NewestMsgAt  string `json:"newestMessageAt"`  // 최신 글 작성 시각
	Targets    []string      `json:"targets"`    // 감시 키워드 라벨
	RecentHits []BaramlogHit `json:"recentHits"` // 최근 발견 (최신 우선)
}

// NewBaramlogWatcher 생성. sendTelegram은 nil 가능(알림 없이 감시만).
func NewBaramlogWatcher(sendTelegram func(string) error) *BaramlogWatcher {
	return &BaramlogWatcher{
		url:      "https://baramlog.com/api/messages",
		interval: 10 * time.Second,
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
		sendTelegram: sendTelegram,
	}
}

func (bw *BaramlogWatcher) SetLogFunc(f func(string)) { bw.logFunc = f }

// SetTelegramSender 텔레그램 설정이 런타임에 바뀔 때 갱신
func (bw *BaramlogWatcher) SetTelegramSender(f func(string) error) {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	bw.sendTelegram = f
}

func (bw *BaramlogWatcher) log(format string, a ...interface{}) {
	m := fmt.Sprintf(format, a...)
	log.Printf("[바람로그] %s", m)
	if bw.logFunc != nil {
		bw.logFunc(m)
	}
}

// IsRunning 동작 여부
func (bw *BaramlogWatcher) IsRunning() bool {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	return bw.running
}

// GetStatus 현재 상태 반환
func (bw *BaramlogWatcher) GetStatus() BaramlogStatus {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	st := BaramlogStatus{
		Running:   bw.running,
		Healthy:   bw.failCount == 0,
		LastID:    bw.lastID,
		TotalHits: bw.totalHits,
		FailCount: bw.failCount,
		LastError: bw.lastErr,
	}
	st.IntervalS = int(bw.interval.Seconds())
	for _, t := range baramlogTargets {
		st.Targets = append(st.Targets, t.label)
	}
	st.RecentHits = append([]BaramlogHit(nil), bw.recentHits...)
	if !bw.newestMsgAt.IsZero() {
		age := time.Since(bw.newestMsgAt)
		st.DataAgeMins = int(age.Minutes())
		st.Stale = age > baramlogStaleThreshold
		st.NewestMsgAt = bw.newestMsgAt.Format("2006-01-02 15:04:05")
	}
	if !bw.lastHitAt.IsZero() {
		st.LastHitAt = bw.lastHitAt.Format("2006-01-02 15:04:05")
	}
	if !bw.lastPollAt.IsZero() {
		st.LastPollAt = bw.lastPollAt.Format("2006-01-02 15:04:05")
	}
	return st
}

// Start 감시 시작 (앱 실행 중 상시 동작).
// 시작 시점을 기준점으로 다시 잡는다 — 중지된 동안 쌓인 옛 모집 글이
// 한꺼번에 알림으로 터지는 것을 막기 위함(모집 글은 시간이 지나면 무의미).
// 따라서 "감시 시작 이후 올라온 새 글"만 알림 대상이다.
func (bw *BaramlogWatcher) Start() {
	bw.mu.Lock()
	if bw.running {
		bw.mu.Unlock()
		return
	}
	bw.running = true
	bw.primed = false // 첫 조회에서 기준점 재설정
	bw.stopChan = make(chan struct{})
	bw.syncChan = make(chan struct{}, 1)
	stop := bw.stopChan
	sync := bw.syncChan
	bw.mu.Unlock()

	go bw.run(stop, sync)
	bw.log("시련 모집 감시 시작 (%.0f초 주기) — 왕궁/나타라자/나타/스투파", bw.interval.Seconds())
}

// Stop 감시 중지
func (bw *BaramlogWatcher) Stop() {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	if !bw.running {
		return
	}
	bw.running = false
	close(bw.stopChan)
	bw.log("시련 모집 감시 중지")
}

func (bw *BaramlogWatcher) run(stop chan struct{}, sync chan struct{}) {
	defer func() {
		if r := recover(); r != nil {
			bw.log("패닉 복구: %v", r)
		}
		bw.mu.Lock()
		bw.running = false
		bw.mu.Unlock()
	}()

	bw.mu.Lock()
	iv0 := bw.interval
	bw.mu.Unlock()
	ticker := time.NewTicker(iv0)
	defer ticker.Stop()

	bw.poll() // 시작 직후 1회 (기준점 설정)

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			bw.poll()
		case <-sync:
			// 수동 새로고침(SyncNow)이 조회를 마쳤거나 주기가 바뀐 경우 —
			// 여기서는 다음 자동 주기만 처음부터 다시 센다 (중복 조회 방지)
			bw.mu.Lock()
			iv := bw.interval
			bw.mu.Unlock()
			ticker.Reset(iv)
		}
	}
}

// SetInterval 조회 주기 변경 (5~600초). 실행 중이면 즉시 반영된다.
func (bw *BaramlogWatcher) SetInterval(seconds int) error {
	if seconds < 5 || seconds > 600 {
		return fmt.Errorf("주기는 5~600초 사이여야 합니다 (입력: %d초)", seconds)
	}
	bw.mu.Lock()
	bw.interval = time.Duration(seconds) * time.Second
	ch := bw.syncChan
	running := bw.running
	bw.mu.Unlock()

	// run 루프에 리셋 신호 (sync 케이스가 ticker.Reset을 수행)
	if running && ch != nil {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	bw.log("조회 주기 변경: %d초", seconds)
	return nil
}

// SendTestMessage 알림 경로 점검용 테스트 메시지 전송.
// 실제 발견/장애 알림과 완전히 같은 경로(sendTelegram)를 타므로,
// 이게 도착하면 실제 알림도 도착한다.
func (bw *BaramlogWatcher) SendTestMessage() error {
	bw.mu.Lock()
	send := bw.sendTelegram
	running := bw.running
	healthy := bw.failCount == 0
	lastPoll := bw.lastPollAt
	totalHits := bw.totalHits
	bw.mu.Unlock()

	if send == nil {
		return fmt.Errorf("텔레그램이 설정되지 않았습니다")
	}

	var targets []string
	for _, t := range baramlogTargets {
		targets = append(targets, t.label)
	}
	lastPollStr := "아직 없음"
	if !lastPoll.IsZero() {
		lastPollStr = lastPoll.Format("2006-01-02 15:04:05")
	}
	state := "감시중"
	if !running {
		state = "중지됨"
	}
	if !healthy {
		state += " (사이트 접속 실패)"
	}

	text := fmt.Sprintf(
		"🔔 <b>시련 모집 알림 테스트</b>\n\n"+
			"이 메시지가 보이면 알림 설정이 정상입니다.\n\n"+
			"감시 상태: %s\n"+
			"감시 키워드: %s\n"+
			"확인 주기: %.0f초\n"+
			"마지막 동기화: %s\n"+
			"누적 발견: %d건\n"+
			"보낸 시각: %s",
		state, strings.Join(targets, ", "), bw.interval.Seconds(),
		lastPollStr, totalHits, time.Now().Format("2006-01-02 15:04:05"))

	if err := send(text); err != nil {
		bw.log("테스트 알림 전송 실패: %v", err)
		return err
	}
	bw.log("테스트 알림 전송 완료")
	return nil
}

// SyncNow 주기와 무관하게 즉시 1회 동기화 (UI 새로고침 버튼용).
// 폴링 중이면 끝날 때까지 기다렸다가 수행하므로, 반환 후 GetStatus는 최신이다.
func (bw *BaramlogWatcher) SyncNow() {
	if !bw.IsRunning() {
		// 감시 중지 상태여도 수동 조회는 허용 (헬스체크 확인용)
		bw.poll()
		return
	}
	bw.poll()
	// 조회는 위에서 끝났고, run 루프에는 "다음 주기 리셋" 신호만 보낸다
	bw.mu.Lock()
	ch := bw.syncChan
	bw.mu.Unlock()
	if ch != nil {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// poll 1회 조회 + 매칭 + 알림 + 헬스체크 갱신
func (bw *BaramlogWatcher) poll() {
	bw.pollMu.Lock()
	defer bw.pollMu.Unlock()

	msgs, err := bw.fetch()
	if err != nil {
		bw.handleFailure(err)
		return
	}
	bw.handleSuccess()

	bw.mu.Lock()
	bw.lastPollAt = time.Now()
	prevID := bw.lastID
	primed := bw.primed
	bw.mu.Unlock()

	maxID := prevID
	var hits []BaramlogMessage
	var labels []string
	var newestAt time.Time

	for _, m := range msgs {
		if m.ID > maxID {
			maxID = m.ID
		}
		// 최신 글의 작성 시각 추적 (데이터 정체 판정용)
		if t, err := time.Parse(time.RFC3339, m.CreatedAt); err == nil && t.After(newestAt) {
			newestAt = t
		}
		// 첫 조회는 기준점만 잡고 과거 메시지로 알림하지 않는다
		if !primed || m.ID <= prevID {
			continue
		}
		if label, ok := matchBaramlogTarget(m.Name + " " + m.Message); ok {
			hits = append(hits, m)
			labels = append(labels, label)
		}
	}

	// 데이터 정체 점검 (기준점 설정 여부와 무관하게 매번)
	bw.checkStale(newestAt)

	bw.mu.Lock()
	bw.lastID = maxID
	if !bw.primed {
		bw.primed = true
		bw.mu.Unlock()
		bw.log("기준점 설정 완료 (최신 id=%d) — 이후 새 글만 알림", maxID)
		return
	}
	if len(hits) > 0 {
		bw.totalHits += len(hits)
		now := time.Now()
		bw.lastHitAt = now
		// 최신이 앞에 오도록 역순으로 앞에 붙인다
		for i := len(hits) - 1; i >= 0; i-- {
			m := hits[i]
			when := m.CreatedAt
			if t, err := time.Parse(time.RFC3339, m.CreatedAt); err == nil {
				when = t.Format("2006-01-02 15:04:05")
			}
			bw.recentHits = append([]BaramlogHit{{
				ID:        m.ID,
				Label:     labels[i],
				Name:      m.Name,
				Message:   m.Message,
				CreatedAt: when,
				FoundAt:   now.Format("2006-01-02 15:04:05"),
			}}, bw.recentHits...)
		}
		if len(bw.recentHits) > baramlogMaxRecent {
			bw.recentHits = bw.recentHits[:baramlogMaxRecent]
		}
	}
	bw.mu.Unlock()

	if len(hits) > 0 {
		bw.notifyHits(hits, labels)
	}
}

// checkStale 사이트가 200을 주면서도 새 글이 안 올라오는 상태를 감지한다.
// 접속 실패와 다른 고장이라 별도로 알리고, 감시는 중지하지 않는다.
// 진입 시 1회, 복구 시 1회 알린다.
func (bw *BaramlogWatcher) checkStale(newestAt time.Time) {
	if newestAt.IsZero() {
		return
	}
	age := time.Since(newestAt)
	stale := age > baramlogStaleThreshold

	bw.mu.Lock()
	bw.newestMsgAt = newestAt
	alerted := bw.staleAlerted
	send := bw.sendTelegram
	if stale && !alerted {
		bw.staleAlerted = true
	} else if !stale && alerted {
		bw.staleAlerted = false
	}
	bw.mu.Unlock()

	switch {
	case stale && !alerted:
		bw.log("데이터 정체 감지 — 최신 글이 %.0f분 전(%s). 감시는 계속합니다.",
			age.Minutes(), newestAt.Format("01-02 15:04:05"))
		if send != nil {
			text := fmt.Sprintf(
				"⚠️ <b>바람로그 데이터 정체</b>\n\n"+
					"사이트는 응답하지만 새 글이 올라오지 않습니다.\n"+
					"(사이트 쪽 문제로 보입니다)\n\n"+
					"최신 글: %s\n"+
					"정체 시간: %.0f분\n"+
					"확인 시각: %s\n\n"+
					"감시는 계속 돌고 있으며, 갱신이 재개되면 다시 알려드립니다.",
				newestAt.Format("2006-01-02 15:04:05"), age.Minutes(),
				time.Now().Format("15:04:05"))
			if e := send(text); e != nil {
				log.Printf("[바람로그] 정체 알림 전송 실패: %v", e)
			}
		}
	case !stale && alerted:
		bw.log("데이터 갱신 재개 — 최신 글 %s", newestAt.Format("01-02 15:04:05"))
		if send != nil {
			text := fmt.Sprintf(
				"✅ <b>바람로그 갱신 재개</b>\n\n"+
					"새 글이 다시 올라오고 있습니다.\n\n"+
					"최신 글: %s\n"+
					"확인 시각: %s",
				newestAt.Format("2006-01-02 15:04:05"), time.Now().Format("15:04:05"))
			if e := send(text); e != nil {
				log.Printf("[바람로그] 갱신 재개 알림 전송 실패: %v", e)
			}
		}
	}
}

// fetch API 조회 (gzip은 Go가 자동 처리)
func (bw *BaramlogWatcher) fetch() ([]BaramlogMessage, error) {
	req, err := http.NewRequest(http.MethodGet, bw.url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "baram-helper/1.0")

	resp, err := bw.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("요청 실패: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var out baramlogResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("JSON 파싱 실패: %v", err)
	}
	if len(out.Messages) == 0 {
		return nil, fmt.Errorf("메시지가 비어 있음")
	}
	return out.Messages, nil
}

// handleFailure 연속 실패 누적 → 3회(약 30초) 연속이면 장애 알림, 이후 30분마다 재알림
func (bw *BaramlogWatcher) handleFailure(err error) {
	bw.mu.Lock()
	bw.failCount++
	bw.lastErr = err.Error()
	count := bw.failCount
	alerted := bw.downAlerted
	lastSent := bw.lastDownSent
	send := bw.sendTelegram
	bw.mu.Unlock()

	bw.log("조회 실패 (%d회 연속): %v", count, err)

	if count < 3 {
		return // 일시적 네트워크 오류는 무시
	}
	if alerted && time.Since(lastSent) < 30*time.Minute {
		return // 재알림 쿨다운
	}

	bw.mu.Lock()
	bw.downAlerted = true
	bw.lastDownSent = time.Now()
	bw.mu.Unlock()

	if send != nil {
		text := fmt.Sprintf(
			"⚠️ <b>바람로그 접속 실패</b>\n\n"+
				"사이트: baramlog.com\n"+
				"연속 실패: %d회\n"+
				"오류: %s\n"+
				"시각: %s\n\n"+
				"시련 모집 알림이 중단된 상태입니다.",
			count, html.EscapeString(err.Error()), time.Now().Format("15:04:05"))
		if e := send(text); e != nil {
			log.Printf("[바람로그] 장애 알림 전송 실패: %v", e)
		}
	}
}

// handleSuccess 성공 시 실패 카운터 리셋 + 장애 알림 후 복구면 복구 알림
func (bw *BaramlogWatcher) handleSuccess() {
	bw.mu.Lock()
	wasDown := bw.downAlerted
	failed := bw.failCount
	bw.failCount = 0
	bw.lastErr = ""
	bw.downAlerted = false
	send := bw.sendTelegram
	bw.mu.Unlock()

	if wasDown {
		bw.log("사이트 정상 복구 (직전 %d회 연속 실패)", failed)
		if send != nil {
			text := fmt.Sprintf(
				"✅ <b>바람로그 복구</b>\n\n"+
					"사이트: baramlog.com\n"+
					"시각: %s\n\n"+
					"시련 모집 알림을 다시 감시합니다.",
				time.Now().Format("15:04:05"))
			if e := send(text); e != nil {
				log.Printf("[바람로그] 복구 알림 전송 실패: %v", e)
			}
		}
	}
}

// notifyHits 매칭된 글들을 한 번에 텔레그램으로
func (bw *BaramlogWatcher) notifyHits(hits []BaramlogMessage, labels []string) {
	for i, m := range hits {
		bw.log("발견 [%s] %s: %s", labels[i], m.Name, m.Message)
	}

	bw.mu.Lock()
	send := bw.sendTelegram
	bw.mu.Unlock()
	if send == nil {
		return
	}

	const maxShow = 10
	var sb strings.Builder
	sb.WriteString("🔔 <b>시련 파티 모집</b>\n\n")
	for i, m := range hits {
		if i >= maxShow {
			sb.WriteString(fmt.Sprintf("\n… 외 %d건 더", len(hits)-maxShow))
			break
		}
		when := m.CreatedAt
		if t, err := time.Parse(time.RFC3339, m.CreatedAt); err == nil {
			when = t.Format("15:04:05")
		}
		sb.WriteString(fmt.Sprintf("[%s] %s\n<b>%s</b>: %s\n\n",
			labels[i], when,
			html.EscapeString(m.Name), html.EscapeString(m.Message)))
	}
	if err := send(strings.TrimRight(sb.String(), "\n")); err != nil {
		log.Printf("[바람로그] 알림 전송 실패: %v", err)
	}
}

// matchBaramlogTarget 공백 제거 후 "시련" + 장소 키워드 동시 포함 여부
func matchBaramlogTarget(text string) (string, bool) {
	norm := strings.ReplaceAll(text, " ", "")
	if !strings.Contains(norm, "시련") {
		return "", false
	}
	for _, t := range baramlogTargets {
		for _, k := range t.keys {
			if strings.Contains(norm, k) {
				return t.label, true
			}
		}
	}
	return "", false
}