#!/bin/bash
# ==============================
# 통합 빌드 스크립트
# 충돌 문제 해결을 위한 파일 정리 추가
# ==============================

# 현재 플랫폼 감지
PLATFORM=""
if [ "$(uname)" == "Darwin" ]; then
  PLATFORM="macos"
elif [ "$(uname)" == "Linux" ]; then
  PLATFORM="linux"
elif [ -n "$WINDIR" ]; then
  PLATFORM="windows"
else
  echo "지원되지 않는 플랫폼입니다."
  exit 1
fi

echo "=== 매크로 도우미 빌드 준비 ==="
echo "플랫폼: $PLATFORM"

# 폴더 구조 확인
echo "[1/7] 폴더 구조 확인 중..."
mkdir -p ui/web/sounds

# 환경 준비
echo "[2/7] 충돌 파일 정리 중..."

# 통합 파일 구조로 리팩토링
if [ -f "automation/daeya.go" ]; then
  echo "automation/daeya.go 파일을 백업하고 제거합니다."
  cp automation/daeya.go automation/daeya.go.bak
  rm automation/daeya.go
fi

if [ -f "automation/kanchan.go" ]; then
  echo "automation/kanchan.go 파일을 백업하고 제거합니다."
  cp automation/kanchan.go automation/kanchan.go.bak
  rm automation/kanchan.go
fi

echo "[3/7] 의존성 확인 및 다운로드 중..."
go mod download
if [ $? -ne 0 ]; then
  echo "의존성 다운로드 실패"
  exit 1
fi

# 빌드 디렉토리 생성
mkdir -p build

# 빌드 유형 선택
echo ""
echo "빌드 타입을 선택하세요:"
echo "1) 개발 빌드 (디버그 정보 포함)"
echo "2) 프로덕션 빌드 (최적화)"
read -p "선택 (기본: 2): " BUILD_TYPE

if [ "$BUILD_TYPE" != "1" ]; then
  BUILD_TYPE="2"
  BUILD_MODE="프로덕션"
  BUILD_TAGS=""
  BUILD_LDFLAGS="-s -w"
else
  BUILD_MODE="개발"
  BUILD_TAGS="-tags dev"
  BUILD_LDFLAGS=""
fi

echo "[4/7] $BUILD_MODE 모드로 빌드 준비 중..."

# 앱 버전
VERSION="1.0.0"
BUILD_DATE=$(date +"%Y-%m-%d")

# 플랫폼별 빌드 설정
APP_NAME="main"
OUTPUT_FILE=""

if [ "$PLATFORM" == "windows" ]; then
  OUTPUT_FILE="build/${APP_NAME}.exe"
  # Windows 리소스 파일 생성 (아이콘 등 포함)
  echo "[5/7] Windows 리소스 준비 중..."

  # 아이콘 파일 확인 및 처리
  ICON_FILE="resources/app.ico"
  if [ -f "$ICON_FILE" ]; then
    echo "아이콘 파일을 찾았습니다: $ICON_FILE"

    # 임시 .syso 파일 생성 (rsrc 도구가 있는 경우)
    if command -v rsrc &> /dev/null; then
      echo "rsrc 도구를 사용하여 Windows 리소스 생성..."
      rsrc -arch=amd64 -manifest resources/app.manifest -ico=$ICON_FILE -o rsrc.syso
    else
      echo "rsrc 도구를 찾을 수 없습니다. 아이콘 없이 빌드합니다."
      echo "아이콘을 포함하려면 다음 명령을 사용하여 rsrc를 설치하세요:"
      echo "go install github.com/akavel/rsrc@latest"
    fi
  else
    echo "아이콘 파일을 찾을 수 없습니다. 기본 아이콘으로 빌드합니다."
  fi
else
  OUTPUT_FILE="build/${APP_NAME}"

  # macOS 아이콘 처리 (향후 개발)
  if [ "$PLATFORM" == "macos" ]; then
    echo "[5/7] macOS 리소스 준비 중..."
    # 아이콘 파일 확인 및 처리 (향후 추가)
    echo "macOS 앱 번들 기능은 향후 추가될 예정입니다."
  else
    echo "[5/7] 리소스 준비 중..."
    echo "플랫폼별 리소스 처리가 필요하지 않습니다."
  fi
fi

# 빌드 실행
echo "[6/7] 빌드 중..."
echo "빌드 중: $OUTPUT_FILE"

LDFLAGS="$BUILD_LDFLAGS -X 'example.com/m/config.Version=$VERSION' -X 'example.com/m/config.BuildDate=$BUILD_DATE' -H=windowsgui"

if go build $BUILD_TAGS -ldflags "$LDFLAGS" -o "$OUTPUT_FILE"; then
  echo "[7/7] 빌드 완료!"

  # 파일 권한 설정 (macOS/Linux)
  if [ "$PLATFORM" != "windows" ]; then
    chmod +x "$OUTPUT_FILE"
  fi

  # 빌드 정보 출력
  echo ""
  echo "=== 빌드 정보 ==="
  echo "애플리케이션: 매크로 도우미"
  echo "버전: $VERSION ($BUILD_DATE)"
  echo "모드: $BUILD_MODE"
  echo "출력 파일: $OUTPUT_FILE"

  # 실행 방법 안내
  echo ""
  echo "=== 실행 방법 ==="
  if [ "$PLATFORM" == "windows" ]; then
    echo "build\\${APP_NAME}.exe를 더블클릭하거나 명령 프롬프트에서 실행하세요."
  else
    echo "터미널에서 다음 명령을 실행하세요: ./build/${APP_NAME}"
  fi

  echo ""
  echo "=== 문제 해결 ==="
  echo "이 스크립트는 충돌하는 파일을 자동으로 처리했습니다."
  echo "원본 daeya.go 및 kanchan.go 파일은 .bak 확장자로 백업되었습니다."
else
  echo "빌드 실패"
  exit 1
fi

echo ""
echo "지금 애플리케이션을 실행하시겠습니까? (y/n)"
read -p "> " RUN_APP

if [ "$RUN_APP" == "y" ] || [ "$RUN_APP" == "Y" ]; then
  echo "애플리케이션 실행 중..."
  if [ "$PLATFORM" == "windows" ]; then
    "$OUTPUT_FILE"
  else
    "$OUTPUT_FILE"
  fi
fi

exit 0
