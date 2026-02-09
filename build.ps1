if (-not (Test-Path "build")) { 
    New-Item -Path "build" -ItemType Directory 
    Write-Host "build 폴더를 생성했습니다." -ForegroundColor Green
}

# Go 의존성 다운로드
Write-Host "Go 의존성 다운로드 중..." -ForegroundColor Cyan
go mod download
if ($LASTEXITCODE -ne 0) {
    Write-Host "의존성 다운로드 실패" -ForegroundColor Red
    exit
}

# 버전 정보 설정
$version = "1.0.0"
$buildDate = Get-Date -Format "yyyy-MM-dd"
$outputFile = "build\main.exe"

# 빌드 명령어 실행 (CMD 창 안 나오게 -H=windowsgui 추가)
Write-Host "매크로 도우미 빌드 중..." -ForegroundColor Cyan
$ldflags = "-s -w -X ""example.com/m/config.Version=$version"" -X ""example.com/m/config.BuildDate=$buildDate"" -H=windowsgui"
go build -ldflags $ldflags -o $outputFile

# 빌드 결과 확인
if ($LASTEXITCODE -eq 0) {
    Write-Host "빌드 성공!" -ForegroundColor Green
    Write-Host "버전: $version ($buildDate)"
    Write-Host "파일 위치: $outputFile"
    
    $run = Read-Host "프로그램을 실행하시겠습니까? (y/n)"
    if ($run -eq "y") {
        Start-Process $outputFile
    }
} else {
    Write-Host "빌드 실패" -ForegroundColor Red
}