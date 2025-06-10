#!/bin/bash

echo "=== UI 빌드 스크립트 ==="

# webui 폴더로 이동
cd webui

# 의존성 설치 확인
if [ ! -d "node_modules" ]; then
    echo "의존성 설치 중..."
    npm install
fi

# React 앱 빌드
echo "React 앱 빌드 중..."
npm run build

if [ $? -eq 0 ]; then
    echo "✅ UI 빌드 완료!"
    echo "📂 빌드 결과: ui/web/"

    # Go 애플리케이션 빌드
    cd ..
    echo "Go 애플리케이션 빌드 중..."

    # build 폴더 생성
    if [ ! -d "build" ]; then
        mkdir -p build
        echo "build 폴더를 생성했습니다."
    fi

    if command -v go &> /dev/null; then
        # 버전 정보 설정
        version="1.0.0"
        buildDate=$(date +"%Y-%m-%d")
        outputFile="build/main.exe"

        # Go 의존성 다운로드
        echo "Go 의존성 다운로드 중..."
        go mod download

        if [ $? -eq 0 ]; then
            # 빌드 명령어 실행 (Windows GUI 모드)
            echo "매크로 도우미 빌드 중..."
            ldflags="-s -w -X example.com/m/config.Version=$version -X example.com/m/config.BuildDate=$buildDate"

            # Windows용 빌드 (GUI 모드)
            GOOS=windows GOARCH=amd64 go build -ldflags "$ldflags -H=windowsgui" -o $outputFile

            if [ $? -eq 0 ]; then
                echo "✅ 전체 빌드 완료!"
                echo "버전: $version ($buildDate)"
                echo "파일 위치: $outputFile"
                echo "🚀 실행: ./$outputFile"
            else
                echo "❌ Go 빌드 실패"
                exit 1
            fi
        else
            echo "❌ Go 의존성 다운로드 실패"
            exit 1
        fi
    else
        echo "❌ Go가 설치되어 있지 않습니다."
        exit 1
    fi
else
    echo "❌ UI 빌드 실패"
    exit 1
fi
