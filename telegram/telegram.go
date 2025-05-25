package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// TelegramBot은 텔레그램 봇 설정을 관리합니다
type TelegramBot struct {
	Token  string
	ChatID string
}

// Message는 텔레그램 메시지 구조체입니다
type Message struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

// NewTelegramBot은 새로운 텔레그램 봇 인스턴스를 생성합니다
func NewTelegramBot(token, chatID string) *TelegramBot {
	return &TelegramBot{
		Token:  token,
		ChatID: chatID,
	}
}

// SendMessage는 텔레그램으로 메시지를 전송합니다
func (tb *TelegramBot) SendMessage(text string) error {
	if tb.Token == "" || tb.ChatID == "" {
		return fmt.Errorf("텔레그램 설정이 완료되지 않았습니다")
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", tb.Token)

	message := Message{
		ChatID:    tb.ChatID,
		Text:      text,
		ParseMode: "HTML",
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("메시지 인코딩 실패: %v", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("메시지 전송 실패: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("텔레그램 API 오류: %d", resp.StatusCode)
	}

	return nil
}

// SendStartNotification은 작업 시작 알림을 전송합니다
func (tb *TelegramBot) SendStartNotification(modeName string, duration time.Duration) error {
	now := time.Now()
	endTime := now.Add(duration)

	// 한국 시간으로 변환 (KST)
	loc, _ := time.LoadLocation("Asia/Seoul")
	startTimeKST := now.In(loc)
	endTimeKST := endTime.In(loc)

	message := fmt.Sprintf(`🚀 <b>매크로 시작 알림</b> 🚀

━━━━━━━━━━━━━━━━━━━━━━━━━━━
🎮 <b>모드:</b> %s
⏱️ <b>예상 실행 시간:</b> %s
🕐 <b>시작 시간:</b> %s
🕕 <b>종료 예상 시간:</b> %s
▶️ <b>상태:</b> <code>실행 시작</code>
━━━━━━━━━━━━━━━━━━━━━━━━━━━

💪 도우미가 열심히 작업을 시작했습니다!
📱 완료되면 자동으로 알림을 보내드릴게요.

<i>⏰ %s 기준</i>`,
		modeName,
		formatDuration(duration),
		startTimeKST.Format("2006년 01월 02일 15:04:05"),
		endTimeKST.Format("2006년 01월 02일 15:04:05"),
		startTimeKST.Format("2006-01-02 15:04:05"))

	return tb.SendMessage(message)
}

// SendCompletionNotification은 작업 완료 알림을 전송합니다
func (tb *TelegramBot) SendCompletionNotification(modeName string, duration time.Duration) error {
	now := time.Now()
	startTime := now.Add(-duration)

	// 한국 시간으로 변환 (KST)
	loc, _ := time.LoadLocation("Asia/Seoul")
	startTimeKST := startTime.In(loc)
	endTimeKST := now.In(loc)

	message := fmt.Sprintf(`🎉 <b>매크로 완료 알림</b> 🎉

━━━━━━━━━━━━━━━━━━━━━━━━━━━
🎮 <b>모드:</b> %s
⏱️ <b>총 실행 시간:</b> %s
🕐 <b>시작 시간:</b> %s
🕕 <b>완료 시간:</b> %s
✅ <b>상태:</b> <code>정상 완료</code>
━━━━━━━━━━━━━━━━━━━━━━━━━━━

🎊 축하합니다! 설정된 시간동안 성공적으로 작업을 완료했습니다!
💎 이제 게임에서 확인해보세요!

<i>⏰ %s 기준</i>`,
		modeName,
		formatDuration(duration),
		startTimeKST.Format("2006년 01월 02일 15:04:05"),
		endTimeKST.Format("2006년 01월 02일 15:04:05"),
		endTimeKST.Format("2006-01-02 15:04:05"))

	return tb.SendMessage(message)
}

// SendErrorNotification은 오류 알림을 전송합니다
func (tb *TelegramBot) SendErrorNotification(modeName string, errorMsg string) error {
	now := time.Now()
	loc, _ := time.LoadLocation("Asia/Seoul")
	nowKST := now.In(loc)

	message := fmt.Sprintf(`❌ <b>매크로 오류 알림</b> ❌

━━━━━━━━━━━━━━━━━━━━━━━━━━━
🎮 <b>모드:</b> %s
⚠️ <b>오류 내용:</b> %s
🛑 <b>상태:</b> <code>오류로 인한 중단</code>
🕐 <b>발생 시간:</b> %s
━━━━━━━━━━━━━━━━━━━━━━━━━━━

🔧 문제를 확인하고 다시 시도해주세요.
📋 자세한 로그는 프로그램의 로그 탭에서 확인할 수 있습니다.

<i>⏰ %s 기준</i>`,
		modeName,
		errorMsg,
		nowKST.Format("2006년 01월 02일 15:04:05"),
		nowKST.Format("2006-01-02 15:04:05"))

	return tb.SendMessage(message)
}

// formatDuration은 시간을 이쁘게 포맷팅합니다
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%d시간 %d분 %d초", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%d분 %d초", minutes, seconds)
	} else {
		return fmt.Sprintf("%d초", seconds)
	}
}

// TestConnection은 텔레그램 봇 연결을 테스트합니다
func (tb *TelegramBot) TestConnection() error {
	now := time.Now()
	loc, _ := time.LoadLocation("Asia/Seoul")
	nowKST := now.In(loc)

	message := fmt.Sprintf(`🤖 <b>도우미 봇 연결 테스트</b> 🤖

━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ <b>상태:</b> <code>연결 성공</code>
📡 <b>테스트 시간:</b> %s
🔔 <b>알림 설정:</b> <code>정상 작동</code>
━━━━━━━━━━━━━━━━━━━━━━━━━━━

🎉 텔레그램 알림이 성공적으로 설정되었습니다!

이제 다음과 같은 알림을 받으실 수 있습니다:
🚀 매크로 시작 알림
🎉 매크로 완료 알림
❌ 오류 발생 알림

<i>⏰ %s 기준</i>`,
		nowKST.Format("2006년 01월 02일 15:04:05"),
		nowKST.Format("2006-01-02 15:04:05"))

	return tb.SendMessage(message)
}
