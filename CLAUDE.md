# 프로젝트: 바람의나라 도우미

## 개요
- Go 언어 기반 게임 자동화 도우미 (바람의나라)
- WebView UI + HTTP API 구조
- OCR 기반 맵/좌표 감지, 키보드/마우스 자동화, 텔레그램 알림

## 개발 브랜치
- `claude/setup-git-RzLnU`

## 현재 진행 중인 작업: 대야전투 자동화 개선

### 요구사항 (사용자 확인 완료)
1. **맵별 동작 분기 (OCR 기반)**
   - "대야산전투기슭" (입구맵): `o → enter → enter → esc` (사냥터 입장 시퀀스)
   - "대야전투" (전투맵): 스킬 사용 + 좌표 이동

2. **대야전투 맵에서 스킬 랜덤 사용**
   - 현재 고정 순서: d, x, 5
   - 변경: 스킬 목록에서 랜덤 순서로 사용

3. **좌표 이동 (메인 캐릭만)**
   - 목표 좌표: (29, 32)
   - 허용 오차: ±1
   - 2~3캐릭 그룹 입장하지만, 메인 캐릭 창만 활성화되어 있으므로 메인만 이동
   - Ctrl+D 기반 좌표 이동 시스템 활용 (item_scanner.go에 이미 구현됨)

### 구현 방향 (아직 미착수)
- `automation/daeya_battle.go` 신규 파일 생성 예정
- OCR로 현재 맵 감지 → "대야산전투기슭"이면 입장, "대야전투"이면 스킬+이동
- item_scanner.go의 moveToOrigin, pressCtrlD 등 좌표 이동 로직 재활용
- main.go에서 ModeDaeyaEnter 시 새 로직 호출

### 기존 코드 구조 참고
- `automation/automation.go`: KeySequence 정의, DaeyaEnter/DaeyaParty 등 시퀀스 실행
- `automation/keyboard.go`: KeyboardManager (RunKeySequence 무한 루프)
- `automation/mouse.go`: MouseAutomation, GameUICoords, StartHunting
- `automation/rotation.go`: RotationManager (자동 사냥 캐릭터 순환)
- `automation/item_scanner.go`: ItemScanner (OCR 맵 감지, Ctrl+D 좌표 이동, 스킬 키 입력)
- `automation/ocr.go`: OCRManager (RecognizeText, ReadCoordinates)
- `main.go`: Application 구조체, HTTP API, 모드별 자동화 시작/중지

### superpowers 플러그인
- 설치 완료 (v5.0.6)
- 다음 세션에서 기획/브레인스토밍에 활용 예정
