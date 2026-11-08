package automation

import (
	"fmt"
	"html"
	"log"
	"strings"
	"sync"
	"time"
)

// BaramlogWatcher 고른 게임 창의 채팅창을 주기적으로 읽어
// 지정한 시련 파티 모집 글이 올라오면 텔레그램으로 알린다.
// (예전엔 baramlog.com 을 조회했는데 사이트가 멈춰서 화면 읽기로 바꿨다. 이름은 UI·API 호환 때문에 유지)
// 창을 연속으로 못 읽으면 장애/복구 알림을 보낸다.
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

	interval time.Duration
	nick     string                                  // 채팅을 읽을 캐릭터(창)
	readChat func(nick string) ([]string, error)      // 그 창의 채팅 줄 (위→아래)
	seen     map[string]time.Time                    // 공백 뺀 줄 → 마지막으로 본 시각
	primed   bool                                    // 첫 조회 완료(이미 떠 있던 줄 알림 방지)
	seq      int

	// 헬스체크 상태 (창 읽기)
	failCount    int
	downAlerted  bool
	lastDownSent time.Time
	lastErr      string

	// 통계
	totalHits  int
	lastHitAt  time.Time
	lastPollAt time.Time
	lastLines  []string      // 마지막으로 읽은 채팅 (화면 확인용)
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

// BaramlogMessage 채팅 한 줄 ("이름: 내용")
type BaramlogMessage struct {
	ID        int
	Name      string
	Message   string
	CreatedAt time.Time // 처음 읽은 시각
}

// chatSeenTTL 같은 줄을 이 시간 안에 다시 보면 새 글로 치지 않는다.
// 화면에 남아 있는 동안은 계속 갱신되므로, 한 번 떴던 글은 사라지고 이만큼 지나야 다시 알린다.
const chatSeenTTL = 5 * time.Minute

// baramlogMaxRecent 화면에 보관할 최근 발견 건수
const baramlogMaxRecent = 50

// BaramlogHit 발견된 모집 글 1건 (화면 표시용)
type BaramlogHit struct {
	ID        int    `json:"id"`
	Label     string `json:"label"`
	Name      string `json:"name"`
	Message   string `json:"message"`
	CreatedAt string `json:"createdAt"` // 채팅에서 읽은 시각
	FoundAt   string `json:"foundAt"`   // 감지 시각
}

// BaramlogStatus UI 표시용 상태
type BaramlogStatus struct {
	Running    bool          `json:"running"`
	Healthy    bool          `json:"healthy"`
	Nick       string        `json:"nick"` // 채팅을 읽는 캐릭터
	TotalHits  int           `json:"totalHits"`
	LastHitAt  string        `json:"lastHitAt"`
	LastPollAt string        `json:"lastPollAt"`
	FailCount  int           `json:"failCount"`
	LastError  string        `json:"lastError"`
	IntervalS  int           `json:"intervalSeconds"`
	Targets    []string      `json:"targets"`    // 감시 키워드 라벨
	LastLines  []string      `json:"lastLines"`  // 마지막으로 읽은 채팅
	RecentHits []BaramlogHit `json:"recentHits"` // 최근 발견 (최신 우선)
}

// NewBaramlogWatcher 생성. sendTelegram은 nil 가능(알림 없이 감시만).
// readChat 은 캐릭터 이름으로 창을 찾아 채팅 줄을 돌려준다.
func NewBaramlogWatcher(sendTelegram func(string) error, readChat func(nick string) ([]string, error)) *BaramlogWatcher {
	return &BaramlogWatcher{
		interval:     2 * time.Second, // 채팅창은 7줄뿐이라 붐빌 때 금방 밀려난다
		readChat:     readChat,
		seen:         map[string]time.Time{},
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

// SetNick 채팅을 읽을 캐릭터를 바꾼다. 바뀌면 기준점을 다시 잡는다(그 창에 떠 있던 줄로 알림 방지).
func (bw *BaramlogWatcher) SetNick(nick string) {
	nick = strings.TrimSpace(nick)
	bw.mu.Lock()
	changed := bw.nick != nick
	bw.nick = nick
	if changed {
		bw.primed = false
	}
	bw.mu.Unlock()
	if changed {
		bw.log("채팅 읽을 창: '%s'", nick)
	}
}

func (bw *BaramlogWatcher) log(format string, a ...interface{}) {
	m := fmt.Sprintf(format, a...)
	log.Printf("[시련알림] %s", m)
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
		Nick:      bw.nick,
		TotalHits: bw.totalHits,
		FailCount: bw.failCount,
		LastError: bw.lastErr,
		LastLines: append([]string(nil), bw.lastLines...),
	}
	st.IntervalS = int(bw.interval.Seconds())
	for _, t := range baramlogTargets {
		st.Targets = append(st.Targets, t.label)
	}
	st.RecentHits = append([]BaramlogHit(nil), bw.recentHits...)
	if !bw.lastHitAt.IsZero() {
		st.LastHitAt = bw.lastHitAt.Format("2006-01-02 15:04:05")
	}
	if !bw.lastPollAt.IsZero() {
		st.LastPollAt = bw.lastPollAt.Format("2006-01-02 15:04:05")
	}
	return st
}

// Start 감시 시작. 시작 시점을 기준점으로 다시 잡는다 — 이미 채팅창에 떠 있던 글은 알리지 않고
// "감시 시작 이후 새로 올라온 줄"만 알린다.
func (bw *BaramlogWatcher) Start() error {
	bw.mu.Lock()
	if bw.running {
		bw.mu.Unlock()
		return nil
	}
	if bw.nick == "" {
		bw.mu.Unlock()
		return fmt.Errorf("채팅을 읽을 창(캐릭터)을 먼저 고르세요")
	}
	bw.running = true
	bw.primed = false // 첫 조회에서 기준점 재설정
	bw.stopChan = make(chan struct{})
	bw.syncChan = make(chan struct{}, 1)
	stop := bw.stopChan
	sync := bw.syncChan
	nick := bw.nick
	bw.mu.Unlock()

	go bw.run(stop, sync)
	bw.log("시련 모집 감시 시작 ('%s' 채팅, %.0f초 주기) — 왕궁/나타라자/나타/스투파", nick, bw.interval.Seconds())
	return nil
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

// SetInterval 조회 주기 변경 (1~600초). 실행 중이면 즉시 반영된다.
func (bw *BaramlogWatcher) SetInterval(seconds int) error {
	if seconds < 1 || seconds > 600 {
		return fmt.Errorf("주기는 1~600초 사이여야 합니다 (입력: %d초)", seconds)
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
	nick := bw.nick
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
		state += " (채팅 읽기 실패)"
	}

	text := fmt.Sprintf(
		"🔔 <b>시련 모집 알림 테스트</b>\n\n"+
			"이 메시지가 보이면 알림 설정이 정상입니다.\n\n"+
			"감시 상태: %s\n"+
			"읽는 창: %s\n"+
			"감시 키워드: %s\n"+
			"확인 주기: %.0f초\n"+
			"마지막 확인: %s\n"+
			"누적 발견: %d건\n"+
			"보낸 시각: %s",
		state, html.EscapeString(nick), strings.Join(targets, ", "), bw.interval.Seconds(),
		lastPollStr, totalHits, time.Now().Format("2006-01-02 15:04:05"))

	if err := send(text); err != nil {
		bw.log("테스트 알림 전송 실패: %v", err)
		return err
	}
	bw.log("테스트 알림 전송 완료")
	return nil
}

// SyncNow 주기와 무관하게 즉시 1회 읽기 (UI 새로고침 버튼용).
// 폴링 중이면 끝날 때까지 기다렸다가 수행하므로, 반환 후 GetStatus는 최신이다.
func (bw *BaramlogWatcher) SyncNow() {
	bw.poll() // 감시 중지 상태여도 수동 읽기는 허용 (창 확인용)
	bw.mu.Lock()
	ch := bw.syncChan
	running := bw.running
	bw.mu.Unlock()
	// 읽기는 위에서 끝났고, run 루프에는 "다음 주기 리셋" 신호만 보낸다
	if running && ch != nil {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// poll 1회 읽기 + 매칭 + 알림 + 헬스체크 갱신
func (bw *BaramlogWatcher) poll() {
	bw.pollMu.Lock()
	defer bw.pollMu.Unlock()

	bw.mu.Lock()
	nick := bw.nick
	bw.mu.Unlock()
	if nick == "" {
		bw.handleFailure(fmt.Errorf("채팅을 읽을 창을 고르지 않았습니다"))
		return
	}
	lines, err := bw.readChat(nick)
	if err != nil {
		bw.handleFailure(err)
		return
	}
	bw.handleSuccess()

	now := time.Now()
	bw.mu.Lock()
	bw.lastPollAt = now
	bw.lastLines = lines
	primed := bw.primed
	bw.primed = true
	var hits []BaramlogMessage
	var labels []string
	for _, l := range lines {
		key := strings.ReplaceAll(l, " ", "")
		last, dup := bw.seen[key]
		bw.seen[key] = now // 화면에 남아 있는 동안 계속 갱신
		// 첫 조회는 기준점만 잡고, 최근에 본 줄은 같은 글로 본다
		if !primed || dup && now.Sub(last) < chatSeenTTL {
			continue
		}
		if label, ok := matchBaramlogTarget(l); ok {
			name, msg, found := strings.Cut(l, ":")
			if !found {
				name, msg = "", l
			}
			bw.seq++
			hits = append(hits, BaramlogMessage{ID: bw.seq, Name: strings.TrimSpace(name), Message: strings.TrimSpace(msg), CreatedAt: now})
			labels = append(labels, label)
		}
	}
	for k, t := range bw.seen {
		if now.Sub(t) >= chatSeenTTL {
			delete(bw.seen, k)
		}
	}
	if !primed {
		bw.mu.Unlock()
		bw.log("기준점 설정 완료 (채팅 %d줄) — 이후 새 줄만 알림", len(lines))
		return
	}
	if len(hits) > 0 {
		bw.totalHits += len(hits)
		bw.lastHitAt = now
		// 최신이 앞에 오도록 역순으로 앞에 붙인다
		for i := len(hits) - 1; i >= 0; i-- {
			m := hits[i]
			bw.recentHits = append([]BaramlogHit{{
				ID:        m.ID,
				Label:     labels[i],
				Name:      m.Name,
				Message:   m.Message,
				CreatedAt: m.CreatedAt.Format("2006-01-02 15:04:05"),
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

// handleFailure 연속 실패 누적 → 15회 연속이면 장애 알림, 이후 30분마다 재알림
func (bw *BaramlogWatcher) handleFailure(err error) {
	bw.mu.Lock()
	bw.failCount++
	bw.lastErr = err.Error()
	count := bw.failCount
	alerted := bw.downAlerted
	lastSent := bw.lastDownSent
	send := bw.sendTelegram
	running := bw.running
	bw.mu.Unlock()

	if count == 1 || count%30 == 0 {
		bw.log("채팅 읽기 실패 (%d회 연속): %v", count, err) // 2초 주기라 매번 찍으면 로그가 넘친다
	}

	if !running || count < 15 {
		return // 잠깐 최소화·로딩 같은 일시적 실패는 무시 (2초 주기 기준 약 30초)
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
			"⚠️ <b>시련 모집 알림 — 채팅 읽기 실패</b>\n\n"+
				"연속 실패: %d회\n"+
				"오류: %s\n"+
				"시각: %s\n\n"+
				"게임 창이 꺼졌거나 최소화됐는지 확인하세요. 시련 모집 알림이 멈춘 상태입니다.",
			count, html.EscapeString(err.Error()), time.Now().Format("15:04:05"))
		if e := send(text); e != nil {
			log.Printf("[시련알림] 장애 알림 전송 실패: %v", e)
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
		bw.log("채팅 읽기 복구 (직전 %d회 연속 실패)", failed)
		if send != nil {
			text := fmt.Sprintf(
				"✅ <b>시련 모집 알림 — 채팅 읽기 복구</b>\n\n"+
					"시각: %s\n\n"+
					"시련 모집 알림을 다시 감시합니다.",
				time.Now().Format("15:04:05"))
			if e := send(text); e != nil {
				log.Printf("[시련알림] 복구 알림 전송 실패: %v", e)
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
		sb.WriteString(fmt.Sprintf("[%s] %s\n<b>%s</b>: %s\n\n",
			labels[i], m.CreatedAt.Format("15:04:05"),
			html.EscapeString(m.Name), html.EscapeString(m.Message)))
	}
	if err := send(strings.TrimRight(sb.String(), "\n")); err != nil {
		log.Printf("[시련알림] 알림 전송 실패: %v", err)
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
