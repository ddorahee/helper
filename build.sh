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
    
    if command -v go &> /dev/null; then
        go build -o build/main.exe
        if [ $? -eq 0 ]; then
            echo "✅ 전체 빌드 완료!"
            echo "🚀 실행: ./build/main.exe"
        else
            echo "❌ Go 빌드 실패"
        fi
    else
        echo "Go가 설치되어 있지 않습니다."
    fi
else
    echo "❌ UI 빌드 실패"
    exit 1
fi