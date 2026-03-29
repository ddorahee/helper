# 프로젝트: 바람의나라 도우미

## 개요
- Go 언어 기반 게임 자동화 도우미 (바람의나라)
- WebView UI + HTTP API 구조
- OCR 기반 맵/좌표 감지, 키보드/마우스 자동화, 텔레그램 알림
- 빌드 출력: `chrome.exe`

## 개발 브랜치
- `claude/read-execute-context-xqrby`

## 완료된 작업

### 대야전투 자동화 (구현 완료)
- **`automation/daeya_battle.go`** 신규 생성
- OCR 맵 감지 → 맵별 분기:
  - "대야산전투기슭" (입구맵): `o → enter → enter → esc` (2~4초 랜덤 딜레이)
  - "대야전투" (전투맵): 스킬 `d, x, 5` 랜덤 순서 사용 + Ctrl+D 좌표 이동 (29,32) ±1
  - 알 수 없는 맵: 스킬만 사용
- main.go에서 `ModeDaeyaEnter` 시 `DaeyaBattle.Start(hwnd)` 호출 (게임 창 없으면 기존 키 시퀀스 폴백)
- 중지 시 `DaeyaBattle.Stop()` 호출 (API 중지 + 전체 중지 2곳)

### 채굴 대기 시간 변경
- `automation/mouse.go`: 채굴 확인 후 대기 30초 → **15초**로 변경

### 빌드 설정 변경
- `build.ps1`, `build.sh`: 출력 파일명 `main.exe` → **`chrome.exe`**로 변경

## 코드 구조
- `automation/automation.go`: KeySequence 정의, DaeyaEnter/DaeyaParty 등 시퀀스 실행
- `automation/keyboard.go`: KeyboardManager (RunKeySequence 무한 루프)
- `automation/mouse.go`: MouseAutomation, GameUICoords, StartHunting (채굴 15초 대기)
- `automation/rotation.go`: RotationManager (자동 사냥 캐릭터 순환)
- `automation/item_scanner.go`: ItemScanner (OCR 맵 감지, Ctrl+D 좌표 이동, 스킬 키 입력)
- `automation/ocr.go`: OCRManager (RecognizeText, ReadCoordinates, 상주 PowerShell OCR)
- `automation/daeya_battle.go`: DaeyaBattle (OCR 맵 감지, 입구/전투맵 분기, 랜덤 스킬, 좌표 이동)
- `main.go`: Application 구조체, HTTP API, 모드별 자동화 시작/중지

## 빌드 참고
- Windows 전용 (WebView2 + robotgo CGO)
- Linux 크로스 컴파일 시 MinGW + EventToken.h 수동 생성 필요
- Windows에서: `.\build.ps1` → `build\chrome.exe` 생성
