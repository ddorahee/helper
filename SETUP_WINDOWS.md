# 윈도우 셋업 가이드

## 사전 준비

1. **Git 설치**: https://git-scm.com/download/win
2. **Go 설치** (1.24.2 이상): https://go.dev/dl/
3. **GCC 설치** (robotgo, webview 빌드에 필요):
   - [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) 또는 [MSYS2](https://www.msys2.org/) 설치
   - 설치 후 `gcc --version`으로 확인

## 저장소 클론

PowerShell 또는 명령 프롬프트에서:

```powershell
git clone https://github.com/ddorahee/helper.git
cd helper
```

이미 클론한 경우 최신 코드 풀:

```powershell
git pull origin main
```

## 의존성 설치

```powershell
go mod download
```

## 빌드

PowerShell에서 빌드 스크립트 실행:

```powershell
.\build.ps1
```

빌드 성공 시 `build\main.exe` 파일이 생성됩니다.

## 문제 해결

| 증상 | 해결 방법 |
|------|-----------|
| `gcc` not found | TDM-GCC 또는 MSYS2 설치 후 PATH에 추가 |
| `go` not found | Go 설치 후 터미널 재시작 |
| webview 빌드 오류 | WebView2 런타임 설치: https://developer.microsoft.com/en-us/microsoft-edge/webview2/ |
| 실행 정책 오류 | `Set-ExecutionPolicy -Scope CurrentUser RemoteSigned` 실행 |
