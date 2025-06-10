// keycode_debug.go - 정확한 키 코드 확인
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	hook "github.com/robotn/gohook"
)

func main() {
	fmt.Println("=== 키 코드 정확한 디버깅 ===")
	fmt.Println("다음 키들을 순서대로 눌러주세요:")
	fmt.Println("1. C 키")
	fmt.Println("2. Delete 키")
	fmt.Println("3. End 키")
	fmt.Println("4. 1 키")
	fmt.Println("5. A 키")
	fmt.Println()
	fmt.Println("각 키를 누른 후 잠시 기다려주세요.")
	fmt.Println("Ctrl+C로 종료")
	fmt.Println("=================================")

	// 시그널 핸들러
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 키 훅 시작
	evChan := hook.Start()
	defer hook.End()

	go func() {
		keyPressCount := 0
		for ev := range evChan {
			if ev.Kind == hook.KeyDown {
				keyPressCount++

				fmt.Printf("\n=== 키 입력 #%d ===\n", keyPressCount)
				fmt.Printf("키 코드: %d (0x%X)\n", ev.Keycode, ev.Keycode)
				fmt.Printf("키 문자: %c (ASCII: %d)\n", ev.Keychar, ev.Keychar)
				fmt.Printf("원시 키 코드: %d\n", ev.Rawcode)

				// 예상되는 키 코드 매핑
				switch ev.Keycode {
				case 67, 99:
					fmt.Printf("🔍 이것은 C 키입니다!\n")
				case 46:
					fmt.Printf("🗑️  이것은 Delete 키입니다 (표준)!\n")
				case 61011:
					fmt.Printf("🗑️  이것은 Delete 키입니다 (한국어 기계식)!\n")
				case 35:
					fmt.Printf("🏁 이것은 End 키입니다!\n")
				case 49:
					fmt.Printf("1️⃣  이것은 1 키입니다!\n")
				case 65, 97:
					fmt.Printf("🅰️  이것은 A 키입니다!\n")
				default:
					fmt.Printf("❓ 알 수 없는 키입니다\n")
				}

				// 특별한 경우들
				if ev.Keycode == 61011 {
					fmt.Printf("⚠️  WARNING: 이 키 코드는 매우 특이합니다!\n")
					fmt.Printf("   이것이 정말 C 키라면 키보드에 문제가 있을 수 있습니다.\n")
				}

				fmt.Printf("시간: %s\n", time.Now().Format("15:04:05.000"))
				fmt.Printf("========================\n")
			}
		}
	}()

	// 사용자에게 안내
	fmt.Println("키 모니터링 시작됨...")
	fmt.Println("지금 C 키를 한 번 눌러보세요!")

	// 시그널 대기
	<-sigChan
	fmt.Println("\n프로그램 종료...")
}
