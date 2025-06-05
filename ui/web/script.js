// 동적으로 할 일 목록을 메인 화면에 추가하지 않도록 수정된 script.js

// 타이머 관련 변수 및 함수를 완전히 클라이언트 중심으로 재구성
const ModeNone = 0;
const ModeDaeyaEnter = 1;
const ModeDaeyaParty = 2;
const ModeKanchenEnter = 3;
const ModeKanchenParty = 4;

const TimeOption1Hour = 0;
const TimeOption2Hour = 1;
const TimeOption3Hour = 2;
const TimeOption4Hour = 3;

// DOM 요소 참조
const timerDisplay = document.getElementById('timer-display');
const statusIndicator = document.getElementById('status-indicator');
const statusText = document.getElementById('status-text');
const miniLog = document.getElementById('mini-log');
const startBtn = document.getElementById('start-btn');
const stopBtn = document.getElementById('stop-btn');
const resetBtn = document.getElementById('reset-btn');
const exitBtn = document.getElementById('exit-btn');
const navButtons = document.querySelectorAll('.nav-button');
const modeOptions = document.querySelectorAll('input[name="mode"]');
const timeOptions = document.querySelectorAll('input[name="time"]');
const darkModeToggle = document.getElementById('dark-mode-toggle');
const soundToggle = document.getElementById('sound-toggle');
const startupToggle = document.getElementById('startup-toggle');
const appVersion = document.getElementById('app-version');
const buildDate = document.getElementById('build-date');

// 로그 관련 DOM 요소
const logsContainer = document.getElementById('logs-container');
const refreshLogsBtn = document.getElementById('refresh-logs-btn');
const clearLogsBtn = document.getElementById('clear-logs-btn');
const autoRefreshToggle = document.getElementById('auto-refresh-toggle');
const showDebugToggle = document.getElementById('show-debug-toggle');
const logFilterInput = document.getElementById('log-filter-input');

// 텔레그램 관련 DOM 요소
const telegramToggle = document.getElementById('telegram-toggle');
const telegramConfig = document.getElementById('telegram-config');
const botTokenInput = document.getElementById('bot-token');
const chatIdInput = document.getElementById('chat-id');
const saveTelegramBtn = document.getElementById('save-telegram-btn');
const testTelegramBtn = document.getElementById('test-telegram-btn');

// 퀘스트 관련 DOM 요소
const addQuestBtn = document.getElementById('addQuestBtn');
const questList = document.getElementById('questList');
const addQuestModal = document.getElementById('addQuestModal');
const addQuestForm = document.getElementById('addQuestForm');
const questTitle = document.getElementById('questTitle');
const questCategory = document.getElementById('questCategory');
const questPriority = document.getElementById('questPriority');
const questDifficulty = document.getElementById('questDifficulty');

// 통계 관련 DOM 요소
const totalQuests = document.getElementById('total-quests');
const completedQuests = document.getElementById('completed-quests');
const activeQuests = document.getElementById('active-quests');
const completionRate = document.getElementById('completion-rate');
const experiencePoints = document.getElementById('experience-points');

// 상태 변수
let isRunning = false;            // 매크로 실행 중 여부
let currentMode = ModeDaeyaEnter; // 현재 선택된 모드
let currentTimeOption = TimeOption3Hour; // 현재 선택된 시간 옵션
let darkMode = true;              // 다크 모드 활성화 여부
let soundEnabled = true;          // 소리 알림 활성화 여부
let autoStartup = false;          // 시작 시 자동 실행 여부
let telegramEnabled = false;      // 텔레그램 알림 활성화 여부
let currentContentSection = 'main'; // 현재 표시 중인 섹션
let countdownInterval = null;      // 카운트다운 인터벌 ID
let countdownTime = 3 * 60 * 60;   // 카운트다운 시간 (초)
let statusCheckInterval = null;     // 상태 확인 인터벌 ID
let timerPaused = false;           // 타이머 일시 정지 여부
let serverTimerStarted = false;    // 서버 타이머 시작 여부

// 로그 관련 변수
let logAutoRefresh = true;
let showDebugLogs = false;
let logFilterText = '';
let logRefreshInterval = null;
let lastLogLength = 0;

// 퀘스트 관련 변수
let quests = [];
let currentQuestFilter = 'all';
let questIdCounter = 1;

// 초기화
document.addEventListener('DOMContentLoaded', () => {
    // 테마 설정
    setTheme(darkMode);

    // 네비게이션 이벤트 리스너
    setupNavigation();

    // 초기 선택 설정
    setupInitialSelections();

    // 버튼 이벤트 리스너
    setupButtonListeners();

    // 설정 이벤트 리스너
    setupSettingsListeners();

    // 로그 관련 리스너 설정
    setupLogListeners();

    // 텔레그램 관련 리스너 설정
    setupTelegramListeners();

    // 퀘스트 관련 리스너 설정
    setupQuestListeners();

    // 저장된 설정 로드 - 추가된 부분
    loadSavedSettings();

    // 초기 타이머 표시 설정
    updateCountdownDisplay(getHoursFromOption(currentTimeOption) * 60 * 60);

    // 초기 로그 메시지
    addLogMessage('프로그램이 시작되었습니다.');

    // 상태 확인 폴링 시작
    setupStatusPolling();

    // 로그 자동 새로고침 설정
    setupLogAutoRefresh();

    // 초기 로그 불러오기
    if (logsContainer && currentContentSection === 'logs') {
        refreshLogs();
    }

    // 퀘스트 초기화
    initializeQuests();
});

function loadSavedSettings() {
    fetch('/api/settings/load')
        .then(response => response.json())
        .then(settings => {
            // 다크모드 설정 적용
            if (settings.dark_mode !== undefined) {
                darkMode = settings.dark_mode;
                if (darkModeToggle) {
                    darkModeToggle.checked = darkMode;
                }
                setTheme(darkMode);
            }

            // 소리 알림 설정 적용
            if (settings.sound_enabled !== undefined) {
                soundEnabled = settings.sound_enabled;
                if (soundToggle) {
                    soundToggle.checked = soundEnabled;
                }
            }

            // 자동 시작 설정 적용
            if (settings.auto_startup !== undefined) {
                autoStartup = settings.auto_startup;
                if (startupToggle) {
                    startupToggle.checked = autoStartup;
                }
            }

            // 텔레그램 설정 적용
            if (settings.telegram_enabled !== undefined) {
                telegramEnabled = settings.telegram_enabled;
                if (telegramToggle) {
                    telegramToggle.checked = telegramEnabled;
                    if (telegramEnabled && telegramConfig) {
                        telegramConfig.style.display = 'block';
                        if (testTelegramBtn) {
                            testTelegramBtn.disabled = false;
                        }
                    }
                }
            }

            addLogMessage('저장된 설정을 불러왔습니다.');
        })
        .catch(() => {
            // 오류 발생 시 기본값 사용
            addLogMessage('기본 설정을 사용합니다.');
        });
}

// 상태 확인 폴링 설정
function setupStatusPolling() {
    // 처음 한 번 즉시 상태 확인
    checkApiStatus();

    // 일정 간격으로 상태 확인
    statusCheckInterval = setInterval(checkApiStatus, 2000); // 2초 간격
}

// API를 사용하여 상태 확인
function checkApiStatus() {
    fetch('/api/status')
        .then(response => response.json())
        .then(data => {
            // 서버 상태가 변경됐을 때만 처리
            if (data.running !== isRunning) {
                // 서버가 중지되었는데 타이머가 일시정지 상태가 아니면 완전 중지 처리
                if (!data.running && !timerPaused) {
                    isRunning = false;
                    statusText.textContent = '준비됨';
                    statusIndicator.classList.remove('running');
                    startBtn.classList.remove('active');
                    stopBtn.classList.remove('active');
                    resetCountdown(); // 이 경우에만 타이머 리셋
                }
                // 서버가 시작되었지만 클라이언트는 실행 중이 아니면 시작 처리
                else if (data.running && !isRunning && !timerPaused) {
                    isRunning = true;
                    serverTimerStarted = true;
                    statusText.textContent = '실행 중';
                    statusIndicator.classList.add('running');
                    startBtn.classList.add('active');
                    stopBtn.classList.remove('active');

                    // 타이머 시작 (일시정지 상태가 아닐 때만)
                    if (!countdownInterval && !timerPaused) {
                        startCountdown(getHoursFromOption(currentTimeOption) * 60 * 60);
                    }
                }
            }
        })
        .catch(() => {
            // 오류 발생시 무시
        });
}

// 네비게이션 기능 설정 - 수정됨
function setupNavigation() {
    navButtons.forEach(button => {
        button.addEventListener('click', () => {
            const section = button.dataset.section;
            changeContentSection(section);

            // 로그 섹션으로 이동할 때 로그 새로고침
            if (section === 'logs' && logsContainer) {
                refreshLogs();
            }

            // 퀘스트 섹션으로 이동할 때 퀘스트 렌더링
            if (section === 'todo') {
                renderQuests();
                updateQuestStats();
            }
        });
    });
}

// 컨텐츠 섹션 변경
function changeContentSection(section) {
    currentContentSection = section;

    // 활성 네비게이션 버튼 변경
    navButtons.forEach(btn => {
        if (btn.dataset.section === section) {
            btn.classList.add('active');
        } else {
            btn.classList.remove('active');
        }
    });

    // 활성 컨텐츠 섹션 변경
    document.querySelectorAll('.content-section').forEach(sec => {
        if (sec.id === `${section}-section`) {
            sec.classList.add('active');
        } else {
            sec.classList.remove('active');
        }
    });
}

// 초기 선택 설정
function setupInitialSelections() {
    // 초기 모드 선택
    modeOptions[0].checked = true;

    // 초기 시간 옵션 선택
    timeOptions[2].checked = true;

    // 모드 이벤트 리스너
    modeOptions.forEach(option => {
        option.addEventListener('change', (e) => {
            currentMode = parseInt(e.target.value);
            setModeApi(currentMode);

            // 모드 이름 가져오기
            let modeName = getModeName(currentMode);
            addLogMessage(`${modeName} 모드 선택됨`);
        });
    });

    // 시간 옵션 이벤트 리스너
    timeOptions.forEach(option => {
        option.addEventListener('change', (e) => {
            currentTimeOption = parseInt(e.target.value);
            setTimeOptionApi(currentTimeOption);

            // 타이머가 실행 중이 아니라면 새 시간으로 표시 업데이트
            if (!isRunning && !timerPaused) {
                let hours = getHoursFromOption(currentTimeOption);
                countdownTime = hours * 60 * 60;
                updateCountdownDisplay(countdownTime);
            }

            addLogMessage(`${formatTimeOption(currentTimeOption)} 실행 설정됨`);
        });
    });
}

// 버튼 이벤트 리스너 설정
function setupButtonListeners() {
    // 시작 버튼
    startBtn.addEventListener('click', () => {
        if (!isRunning) {
            try {
                // 시작 버튼 시각적 피드백
                startBtn.classList.add('active');

                // 일시정지 상태였다면 기존 타이머 값으로 재개, 아니면 새로 시작
                const wasTimerPaused = timerPaused;
                timerPaused = false;

                // 중지 버튼 활성화 상태 해제
                stopBtn.classList.remove('active');

                // 서버에 매크로 시작 요청
                startOperation(wasTimerPaused);

                // 상태 업데이트
                isRunning = true;
                statusText.textContent = '실행 중';
                statusIndicator.classList.add('running');

            } catch (error) {
                addLogMessage("오류 발생: 시작 작업을 실행할 수 없습니다.");
                startBtn.classList.remove('active');
            }
        } else {
            addLogMessage("이미 작업이 실행 중입니다...");
        }
    });

    // 중지 버튼
    stopBtn.addEventListener('click', () => {
        if (isRunning) {
            try {
                // 중지 버튼 시각적 피드백
                stopBtn.classList.add('active');

                // 서버에 매크로 중지 요청
                stopOperation();

                // 상태 업데이트 (타이머는 일시정지)
                isRunning = false;
                timerPaused = true;
                statusText.textContent = '준비됨';
                statusIndicator.classList.remove('running');
                startBtn.classList.remove('active');

                // 타이머 일시정지 (카운트다운 인터벌 중지, 값은 유지)
                if (countdownInterval) {
                    clearInterval(countdownInterval);
                    countdownInterval = null;
                    timerDisplay.classList.remove('running');
                }

                addLogMessage("작업이 일시 중지되었습니다.");

            } catch (error) {
                addLogMessage("오류 발생: 중지 작업을 실행할 수 없습니다.");
                stopBtn.classList.remove('active');
            }
        } else {
            addLogMessage("실행 중인 작업이 없습니다.");
        }
    });

    // 재설정 버튼
    resetBtn.addEventListener('click', () => {
        if (!isRunning) {
            try {
                resetBtn.classList.add('active');
                resetSettingsApi();

                // 재설정 시 일시정지 상태 해제 및 중지 버튼 비활성화
                timerPaused = false;
                stopBtn.classList.remove('active');

                // 타이머 초기화
                const hours = getHoursFromOption(currentTimeOption);
                countdownTime = hours * 60 * 60;
                updateCountdownDisplay(countdownTime);

                // 잠시 후 버튼 활성화 해제
                setTimeout(() => {
                    resetBtn.classList.remove('active');
                }, 1000);

                addLogMessage("모든 설정이 초기화되었습니다.");

            } catch (error) {
                addLogMessage("오류 발생: 재설정 작업을 실행할 수 없습니다.");
                resetBtn.classList.remove('active');
            }
        } else {
            addLogMessage('작업 중에는 재설정할 수 없습니다.');
        }
    });

    // 종료 버튼
    exitBtn.addEventListener('click', () => {
        try {
            addLogMessage('프로그램을 종료합니다...');
            exitBtn.classList.add('active');

            // 종료 전 짧은 지연
            setTimeout(() => {
                exitApplicationApi();
            }, 500);
        } catch (error) {
            addLogMessage("오류 발생: 종료 작업을 실행할 수 없습니다.");
            exitBtn.classList.remove('active');
        }
    });
}

// 설정 관련 함수들
function setupSettingsListeners() {
    // 다크모드 토글
    if (darkModeToggle) {
        darkModeToggle.addEventListener('change', () => {
            darkMode = darkModeToggle.checked;
            setTheme(darkMode);

            // 서버에 설정 저장
            saveSetting('dark_mode', darkMode ? 1 : 0);
            addLogMessage(`다크 모드: ${darkMode ? '켜짐' : '꺼짐'}`);
        });
    }

    // 소리 알림 토글
    if (soundToggle) {
        soundToggle.addEventListener('change', () => {
            soundEnabled = soundToggle.checked;

            // 서버에 설정 저장
            saveSetting('sound_enabled', soundEnabled ? 1 : 0);
            addLogMessage(`소리 알림: ${soundEnabled ? '켜짐' : '꺼짐'}`);
        });
    }

    // 자동 시작 토글
    if (startupToggle) {
        startupToggle.addEventListener('change', () => {
            autoStartup = startupToggle.checked;

            // 서버에 설정 저장
            saveSetting('auto_startup', autoStartup ? 1 : 0);
            addLogMessage(`시작 시 자동 실행: ${autoStartup ? '켜짐' : '꺼짐'}`);
        });
    }
}

function saveSetting(type, value) {
    fetch('/api/settings', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
        },
        body: `type=${type}&value=${value}`
    }).catch(() => {
        // 오류 발생 시 무시
    });
}

// 텔레그램 활성화 상태 API 전송 수정
function setTelegramEnabledApi(enabled) {
    saveSetting('telegram_enabled', enabled ? 1 : 0);
}

// 텔레그램 관련 이벤트 리스너 설정
function setupTelegramListeners() {
    // 요소가 없으면 건너뛰기
    if (!telegramToggle || !telegramConfig || !botTokenInput || !chatIdInput ||
        !saveTelegramBtn || !testTelegramBtn) {
        return;
    }

    // 텔레그램 토글
    telegramToggle.addEventListener('change', () => {
        telegramEnabled = telegramToggle.checked;

        if (telegramEnabled) {
            telegramConfig.style.display = 'block';
            addLogMessage('텔레그램 알림: 켜짐');
        } else {
            telegramConfig.style.display = 'none';
            addLogMessage('텔레그램 알림: 꺼짐');

            // 서버에 비활성화 전송
            setTelegramEnabledApi(false);
        }
    });

    // 텔레그램 설정 저장
    saveTelegramBtn.addEventListener('click', () => {
        saveTelegramSettings();
    });

    // 텔레그램 테스트
    testTelegramBtn.addEventListener('click', () => {
        testTelegramConnection();
    });

    // 초기 텔레그램 설정 로드
    loadTelegramSettings();
}

// 텔레그램 설정 저장
function saveTelegramSettings() {
    const token = botTokenInput.value.trim();
    const chatId = chatIdInput.value.trim();

    if (!token || !chatId) {
        showNotification('봇 토큰과 채팅 ID를 모두 입력해주세요.', 'error');
        return;
    }

    // 버튼 비활성화
    saveTelegramBtn.disabled = true;
    saveTelegramBtn.textContent = '저장 중...';

    // 서버에 설정 전송
    fetch('/api/telegram/config', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
        },
        body: `token=${encodeURIComponent(token)}&chat_id=${encodeURIComponent(chatId)}`
    })
        .then(response => {
            if (response.ok) {
                showNotification('텔레그램 설정이 저장되었습니다! 🎉', 'success');
                addLogMessage('텔레그램 설정이 저장되었습니다.');

                // 테스트 버튼 활성화
                testTelegramBtn.disabled = false;
            } else {
                throw new Error('설정 저장 실패');
            }
        })
        .catch(error => {
            showNotification('설정 저장에 실패했습니다. 다시 시도해주세요.', 'error');
            addLogMessage('텔레그램 설정 저장 실패');
        })
        .finally(() => {
            // 버튼 복원
            saveTelegramBtn.disabled = false;
            saveTelegramBtn.innerHTML = `
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M19 21l-7-5-7 5V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2z"></path>
            </svg>
            저장
        `;
        });
}

// 텔레그램 연결 테스트
function testTelegramConnection() {
    // 버튼 비활성화
    testTelegramBtn.disabled = true;
    testTelegramBtn.textContent = '테스트 중...';

    fetch('/api/telegram/test', {
        method: 'POST'
    })
        .then(response => {
            if (response.ok) {
                showNotification('테스트 메시지가 전송되었습니다! 📱', 'success');
                addLogMessage('텔레그램 테스트 메시지 전송 완료');
            } else {
                throw new Error('테스트 실패');
            }
        })
        .catch(error => {
            showNotification('테스트에 실패했습니다. 설정을 확인해주세요.', 'error');
            addLogMessage('텔레그램 테스트 실패');
        })
        .finally(() => {
            // 버튼 복원
            testTelegramBtn.disabled = false;
            testTelegramBtn.innerHTML = `
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M22 2L11 13"></path>
                <path d="M22 2L15 22L11 13L2 9L22 2Z"></path>
            </svg>
            테스트
        `;
        });
}

// 텔레그램 설정 로드
function loadTelegramSettings() {
    fetch('/api/telegram/config')
        .then(response => response.json())
        .then(data => {
            if (data.enabled) {
                telegramToggle.checked = true;
                telegramEnabled = true;
                telegramConfig.style.display = 'block';
                testTelegramBtn.disabled = false;
            }
        })
        .catch(() => {
            // 오류 무시
        });
}

// 텔레그램 활성화 상태 API 전송
function setTelegramEnabledApi(enabled) {
    fetch('/api/settings', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
        },
        body: `type=telegram_enabled&value=${enabled ? 1 : 0}`
    }).catch(() => { });
}

// 알림 메시지 표시
function showNotification(message, type = 'info') {
    // 기존 알림 제거
    const existingNotification = document.querySelector('.notification');
    if (existingNotification) {
        existingNotification.remove();
    }

    // 새 알림 생성
    const notification = document.createElement('div');
    notification.className = `notification ${type}`;
    notification.textContent = message;

    // DOM에 추가
    document.body.appendChild(notification);

    // 애니메이션으로 표시
    setTimeout(() => {
        notification.classList.add('show');
    }, 100);

    // 3초 후 자동 제거
    setTimeout(() => {
        notification.classList.remove('show');
        setTimeout(() => {
            notification.remove();
        }, 300);
    }, 3000);
}

// 로그 관련 이벤트 리스너 설정
function setupLogListeners() {
    // 요소가 없으면 건너뛰기
    if (!refreshLogsBtn || !clearLogsBtn || !autoRefreshToggle || !showDebugToggle || !logFilterInput) {
        return;
    }

    // 로그 새로고침 버튼
    refreshLogsBtn.addEventListener('click', () => {
        refreshLogs();
    });

    // 로그 지우기 버튼
    clearLogsBtn.addEventListener('click', () => {
        clearLogs();
    });

    // 자동 새로고침 토글
    autoRefreshToggle.addEventListener('change', () => {
        logAutoRefresh = autoRefreshToggle.checked;
        if (logAutoRefresh) {
            setupLogAutoRefresh();
        } else {
            clearInterval(logRefreshInterval);
        }
    });

    // 디버그 로그 표시 토글
    showDebugToggle.addEventListener('change', () => {
        showDebugLogs = showDebugToggle.checked;
        refreshLogs();
    });

    // 로그 필터
    logFilterInput.addEventListener('input', () => {
        logFilterText = logFilterInput.value.toLowerCase();
        refreshLogs();
    });
}

// 자동 로그 새로고침 설정
function setupLogAutoRefresh() {
    if (logRefreshInterval) {
        clearInterval(logRefreshInterval);
    }

    if (logAutoRefresh) {
        logRefreshInterval = setInterval(() => {
            if (currentContentSection === 'logs') {
                refreshLogs();
            }
        }, 10000); // 10초마다 새로고침
    }
}

// 로그 새로고침 및 표시 함수들
function refreshLogs() {
    // logsContainer 요소가 없으면 무시
    if (!logsContainer) {
        return;
    }

    fetch('/api/logs')
        .then(response => response.json())
        .then(data => {
            displayLogs(data.logs);
        })
        .catch(() => {
            // 오류 발생 시 기본 메시지 표시
            logsContainer.innerHTML = '<p class="log-placeholder">로그를 불러올 수 없습니다.</p>';
        });
}

function displayLogs(logs) {
    if (!logsContainer) {
        return;
    }

    if (!logs || logs.length === 0) {
        logsContainer.innerHTML = '<p class="log-placeholder">로그가 없습니다.</p>';
        return;
    }

    logsContainer.innerHTML = '';

    logs.forEach(log => {
        // 필터링 적용
        if (logFilterText && !log.toLowerCase().includes(logFilterText)) {
            return;
        }

        // 디버그 로그 필터링
        if (!showDebugLogs && isDebugLog(log)) {
            return;
        }

        // 로그 항목 생성
        const logEntry = document.createElement('pre');
        logEntry.className = 'log-entry ' + getLogLevel(log);
        logEntry.textContent = log;

        // 로그 항목 추가
        logsContainer.appendChild(logEntry);
    });

    // 자동 스크롤
    logsContainer.scrollTop = logsContainer.scrollHeight;

    // 로그 길이 저장
    lastLogLength = logs.length;
}

function clearLogs() {
    fetch('/api/logs/clear', { method: 'POST' })
        .then(response => {
            if (response.ok) {
                logsContainer.innerHTML = '<p class="log-placeholder">로그가 지워졌습니다.</p>';
                lastLogLength = 0;
            }
        })
        .catch(() => {
            // 오류 발생 시 메시지 표시
            addLogMessage("로그 파일을 지울 수 없습니다.");
        });
}

// 로그 레벨 판단
function getLogLevel(log) {
    const lowerLog = log.toLowerCase();
    if (lowerLog.includes('error') || lowerLog.includes('오류') || lowerLog.includes('실패')) {
        return 'error';
    } else if (lowerLog.includes('warn') || lowerLog.includes('경고')) {
        return 'warning';
    } else if (isDebugLog(log)) {
        return 'debug';
    }
    return 'info';
}

// 디버그 로그 판단
function isDebugLog(log) {
    const lowerLog = log.toLowerCase();
    return lowerLog.includes('debug') || lowerLog.includes('초기화') ||
        lowerLog.includes('설정') || lowerLog.includes('디버그');
}

// 테마 설정
function setTheme(isDark) {
    if (isDark) {
        document.documentElement.removeAttribute('data-theme');
    } else {
        document.documentElement.setAttribute('data-theme', 'light');
    }
}

// 로그 메시지 추가
function addLogMessage(message) {
    // 미니 로그 업데이트
    if (miniLog) {
        miniLog.textContent = message;
    }

    // 서버에 로그 전송 (내부 저장용)
    try {
        fetch('/api/log', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ message: message })
        }).catch(() => { });
    } catch (e) {
        // 오류는 무시
    }
}

// 모드 이름 가져오기
function getModeName(mode) {
    switch (mode) {
        case ModeDaeyaEnter:
            return '대야 (입장)';
        case ModeDaeyaParty:
            return '대야 (파티)';
        case ModeKanchenEnter:
            return '칸첸 (입장)';
        case ModeKanchenParty:
            return '칸첸 (파티)';
        default:
            return '알 수 없음';
    }
}

// API 모드 이름 가져오기
function getApiModeName(mode) {
    switch (mode) {
        case ModeDaeyaEnter:
            return 'daeya-entrance';
        case ModeDaeyaParty:
            return 'daeya-party';
        case ModeKanchenEnter:
            return 'kanchen-entrance';
        case ModeKanchenParty:
            return 'kanchen-party';
        default:
            return '';
    }
}

// 시간 옵션별 시간 가져오기
function getHoursFromOption(option) {
    switch (option) {
        case TimeOption1Hour:
            return 1 + (10 / 60); // 1시간 10분
        case TimeOption2Hour:
            return 2 + (10 / 60); // 2시간 10분
        case TimeOption3Hour:
            return 3 + (10 / 60); // 3시간 10분
        case TimeOption4Hour:
            return 4 + (10 / 60); // 4시간 10분
        default:
            return 3 + (10 / 60); // 기본값: 3시간 10분
    }
}

function formatTimeOption(option) {
    switch (option) {
        case TimeOption1Hour:
            return '1시간 10분';
        case TimeOption2Hour:
            return '2시간 10분';
        case TimeOption3Hour:
            return '3시간 10분';
        case TimeOption4Hour:
            return '4시간 10분';
        default:
            return '3시간 10분';
    }
}

// 카운트다운 표시 업데이트
function updateCountdownDisplay(seconds) {
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const secs = Math.floor(seconds % 60);

    timerDisplay.textContent = `${hours.toString().padStart(2, '0')}:${minutes.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
}

// 카운트다운 시작 - 클라이언트에서 타이머 관리
function startCountdown(seconds) {
    // 이전 카운트다운이 있으면 중지
    if (countdownInterval) {
        clearInterval(countdownInterval);
    }

    // 일시정지 상태가 아니면 새로운 타이머 값 설정
    if (!timerPaused) {
        countdownTime = seconds;
    }

    // 타이머 표시 업데이트
    updateCountdownDisplay(countdownTime);

    // 타이머 스타일 업데이트
    timerDisplay.classList.add('running');

    // 카운트다운 시작
    countdownInterval = setInterval(() => {
        countdownTime--;

        if (countdownTime <= 0) {
            // 시간이 다 되면 자동 종료
            clearInterval(countdownInterval);
            countdownInterval = null;
            stopOperation();
            addLogMessage("설정한 시간이 경과하여 자동으로 종료되었습니다.");
        } else {
            // 표시 업데이트
            updateCountdownDisplay(countdownTime);
        }
    }, 1000);
}

// 카운트다운 중지
function stopCountdown() {
    if (countdownInterval) {
        clearInterval(countdownInterval);
        countdownInterval = null;
    }

    // 타이머 스타일 업데이트
    timerDisplay.classList.remove('running');
}

// 카운트다운 리셋
function resetCountdown() {
    // 타이머 중지
    if (countdownInterval) {
        clearInterval(countdownInterval);
        countdownInterval = null;
    }

    // 타이머 값 초기화
    const hours = getHoursFromOption(currentTimeOption);
    countdownTime = hours * 60 * 60;
    updateCountdownDisplay(countdownTime);

    // 타이머 스타일 업데이트
    timerDisplay.classList.remove('running');

    // 일시정지 상태 해제
    timerPaused = false;
}

// 이벤트 수신 함수
window.dispatchAppEvent = function (event) {
    const { type, payload } = event;

    switch (type) {
        case 'updateTimer':
            // 웹앱에서 자체적으로 타이머를 관리하므로 무시
            break;
        case 'logMessage':
            addLogMessage(payload.message);
            break;
        case 'operationStatus':
            // 서버에서 상태 변경 이벤트 받음 - 타이머는 건드리지 않음
            if (payload.running !== isRunning) {
                if (payload.running) {
                    // 서버에서 시작 신호가 왔지만 클라이언트는 중지됐다면
                    if (!isRunning && !timerPaused) {
                        isRunning = true;
                        serverTimerStarted = true;
                        statusText.textContent = '실행 중';
                        statusIndicator.classList.add('running');
                        startBtn.classList.add('active');

                        // 타이머 시작 (일시정지 상태가 아니면)
                        if (!countdownInterval && !timerPaused) {
                            startCountdown(getHoursFromOption(currentTimeOption) * 60 * 60);
                        }
                    }
                } else {
                    // 서버에서 중지 신호가 왔고 일시정지 상태가 아니라면
                    if (isRunning && !timerPaused) {
                        isRunning = false;
                        statusText.textContent = '준비됨';
                        statusIndicator.classList.remove('running');
                        startBtn.classList.remove('active');
                        stopBtn.classList.remove('active');

                        // 타이머 리셋 (일시정지가 아닐 때만)
                        resetCountdown();
                    }
                }
            }
            break;
        case 'resetMode':
            resetModeSelection(payload.mode);
            break;
        case 'resetTimeOption':
            resetTimeOptionSelection(payload.option);
            break;
        case 'resetTimer':
            // 타이머 값 초기화
            resetCountdown();
            break;
        case 'appVersion':
            updateAppVersion(payload.version, payload.buildDate);
            break;
    }
};

// 모드 선택 초기화
function resetModeSelection(mode) {
    currentMode = mode;

    modeOptions.forEach(option => {
        option.checked = parseInt(option.value) === mode;
    });
}

// 시간 옵션 초기화
function resetTimeOptionSelection(option) {
    currentTimeOption = option;

    timeOptions.forEach(opt => {
        opt.checked = parseInt(opt.value) === option;
    });

    // 타이머 표시 업데이트 (실행 중이 아닐 때만)
    if (!isRunning && !timerPaused) {
        const hours = getHoursFromOption(option);
        countdownTime = hours * 60 * 60;
        updateCountdownDisplay(countdownTime);
    }
}

// 앱 버전 정보 업데이트
function updateAppVersion(version, date) {
    if (appVersion) appVersion.textContent = version;
    if (buildDate) buildDate.textContent = date;
}

// API 호출 관련 함수

// 작업 시작 함수
function startOperation(wasTimerPaused) {
    // 모드 선택 확인
    if (currentMode === ModeNone) {
        addLogMessage("오류: 모드를 선택해야 합니다.");
        startBtn.classList.remove('active');
        isRunning = false;
        return;
    }

    // API 모드 변환
    const apiMode = getApiModeName(currentMode);
    if (!apiMode) {
        addLogMessage("오류: 유효하지 않은 모드입니다.");
        startBtn.classList.remove('active');
        isRunning = false;
        return;
    }

    // 실행 시간 설정 확인
    const hours = getHoursFromOption(currentTimeOption);

    // 서버에 시작 요청 - 재시작 여부를 파라미터로 전달
    const requestBody = `mode=${apiMode}&auto_stop=${hours}${wasTimerPaused ? '&resume=true' : ''}`;

    fetch('/api/start', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
        },
        body: requestBody
    })
        .then(response => {
            if (response.ok) {
                // 서버 시작 성공
                serverTimerStarted = true;

                // 클라이언트 타이머 시작
                if (!countdownInterval) {
                    // 일시정지 상태였다면 그 값 유지, 아니면 새로 시작
                    if (wasTimerPaused) {
                        startCountdown(countdownTime);
                    } else {
                        startCountdown(hours * 60 * 60);
                    }
                }

                // 로그 메시지 - 재시작 여부에 따라 다른 메시지
                if (wasTimerPaused) {
                    addLogMessage(`${getModeName(currentMode)} 모드 작업을 재개합니다...`);
                } else {
                    addLogMessage(`${getModeName(currentMode)} 모드로 작업을 시작합니다... (${formatTimeOption(currentTimeOption)})`);
                }
            } else {
                throw new Error('작업 시작 실패');
            }
        })
        .catch(error => {
            addLogMessage("오류: 작업을 시작할 수 없습니다.");
            startBtn.classList.remove('active');

            // 상태 복원
            isRunning = false;
            statusText.textContent = '준비됨';
            statusIndicator.classList.remove('running');

            // 일시정지 상태였다면 복원
            if (wasTimerPaused) {
                timerPaused = true;
                stopBtn.classList.add('active');
            }
        });
}

// 시간 옵션 변경시 로그 메시지도 수정
timeOptions.forEach(option => {
    option.addEventListener('change', (e) => {
        currentTimeOption = parseInt(e.target.value);
        setTimeOptionApi(currentTimeOption);

        // 타이머가 실행 중이 아니라면 새 시간으로 표시 업데이트
        if (!isRunning && !timerPaused) {
            let hours = getHoursFromOption(currentTimeOption);
            countdownTime = hours * 60 * 60;
            updateCountdownDisplay(countdownTime);
        }

        addLogMessage(`${formatTimeOption(currentTimeOption)} 실행 설정됨`);
    });
});

// 작업 중지 함수
function stopOperation() {
    fetch('/api/stop', {
        method: 'POST'
    })
        .then(response => {
            if (!response.ok) {
                throw new Error('작업 중지 실패');
            }
        })
        .catch(error => {
            addLogMessage("오류: 작업을 중지할 수 없습니다.");
        });
}

// 설정 재설정 함수
function resetSettingsApi() {
    // 타이머 재설정 (값 초기화)
    resetCountdown();

    // 모드 초기화 - 대야 입장으로 설정
    resetModeSelection(ModeDaeyaEnter);

    // 시간 설정 초기화 - 3시간으로 설정
    resetTimeOptionSelection(TimeOption3Hour);

    // 일시정지 상태 해제
    timerPaused = false;

    // 중지 버튼 활성화 상태 해제
    stopBtn.classList.remove('active');

    // API를 사용한 설정 저장
    fetch('/api/reset', {
        method: 'POST'
    }).catch(() => { });
}

// 종료 함수
function exitApplicationApi() {
    // API 호출하여 애플리케이션 종료 요청
    fetch('/api/exit', {
        method: 'POST'
    }).catch(() => {
        exitBtn.classList.remove('active');
    });
}

// 모드 설정 API
function setModeApi(mode) {
    const apiMode = getApiModeName(mode);

    // API 호출하여 모드 설정 저장
    fetch('/api/settings', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
        },
        body: `type=mode&value=${apiMode}`
    }).catch(() => { });
}

// 시간 옵션 설정 API
function setTimeOptionApi(option) {
    const hours = getHoursFromOption(option);

    // API 호출하여 시간 설정 저장
    fetch('/api/settings', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
        },
        body: `type=time&value=${hours}`
    }).catch(() => { });
}

// 자동 시작 설정 API
function setAutoStartupApi(enabled) {
    // API 호출하여 자동 시작 설정 저장
    fetch('/api/settings', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
        },
        body: `type=auto_startup&value=${enabled ? 1 : 0}`
    }).catch(() => { });
}

// ====== 퀘스트 관련 기능 ======

// 퀘스트 관련 상수
const categoryIcons = {
    game: "🎮",
    daily: "📅",
    shopping: "🛒",
    special: "⭐"
};

const categoryNames = {
    game: "게임 퀘스트",
    daily: "일상 업무",
    shopping: "쇼핑 목록",
    special: "특별 임무"
};

const priorityLabels = {
    low: "낮음",
    medium: "보통",
    high: "높음"
};

// 퀘스트 관련 이벤트 리스너 설정
function setupQuestListeners() {
    // 요소가 없으면 건너뛰기
    if (!addQuestBtn || !addQuestModal || !addQuestForm) {
        return;
    }

    // 새 퀘스트 추가 버튼
    addQuestBtn.addEventListener('click', () => {
        openQuestModal();
    });

    // 퀘스트 폼 제출
    addQuestForm.addEventListener('submit', (e) => {
        e.preventDefault();
        addQuest();
    });

    // 모달 외부 클릭 시 닫기
    addQuestModal.addEventListener('click', (e) => {
        if (e.target === addQuestModal) {
            closeQuestModal();
        }
    });

    // 카테고리 필터링
    document.querySelectorAll('.category-card').forEach(card => {
        card.addEventListener('click', function () {
            // 모든 카테고리 카드에서 active 클래스 제거
            document.querySelectorAll('.category-card').forEach(c => c.classList.remove('active'));
            // 클릭된 카드에 active 클래스 추가
            this.classList.add('active');

            const category = this.dataset.category;
            currentQuestFilter = category;
            filterQuests(category);
        });
    });
}

// 퀘스트 초기화
function initializeQuests() {
    // 기본 퀘스트들 추가
    if (quests.length === 0) {
        quests = [
            {
                id: questIdCounter++,
                title: "매크로 자동화 스크립트 개선",
                category: "game",
                priority: "high",
                difficulty: 3,
                completed: false
            },
            {
                id: questIdCounter++,
                title: "장비 강화 재료 정리",
                category: "game",
                priority: "medium",
                difficulty: 2,
                completed: true
            },
            {
                id: questIdCounter++,
                title: "길드 활동 참여",
                category: "daily",
                priority: "low",
                difficulty: 1,
                completed: false
            },
            {
                id: questIdCounter++,
                title: "생활 용품 쇼핑",
                category: "shopping",
                priority: "medium",
                difficulty: 1,
                completed: false
            }
        ];

        saveQuestsToStorage();
    }

    renderQuests();
    updateCategoryStats();
    updateQuestStats();
}

// 퀘스트 모달 열기
function openQuestModal() {
    if (addQuestModal) {
        addQuestModal.classList.add('show');
    }
}

// 퀘스트 모달 닫기
function closeQuestModal() {
    if (addQuestModal) {
        addQuestModal.classList.remove('show');
        if (addQuestForm) {
            addQuestForm.reset();
        }
    }
}

// 새 퀘스트 추가
function addQuest() {
    const title = questTitle.value.trim();
    const category = questCategory.value;
    const priority = questPriority.value;
    const difficulty = parseInt(questDifficulty.value);

    if (!title) {
        showNotification('퀘스트 이름을 입력해주세요.', 'error');
        return;
    }

    const newQuest = {
        id: questIdCounter++,
        title: title,
        category: category,
        priority: priority,
        difficulty: difficulty,
        completed: false
    };

    quests.push(newQuest);
    saveQuestsToStorage();
    renderQuests();
    updateCategoryStats();
    updateQuestStats();
    closeQuestModal();

    addLogMessage(`새 퀘스트가 추가되었습니다: ${title}`);
    showNotification('새 퀘스트가 추가되었습니다! ⚔️', 'success');
}

// 퀘스트 완료 상태 토글
function toggleQuest(questId) {
    const quest = quests.find(q => q.id === questId);
    if (quest) {
        quest.completed = !quest.completed;
        saveQuestsToStorage();
        renderQuests();
        updateCategoryStats();
        updateQuestStats();

        const statusText = quest.completed ? '완료됨' : '진행 중';
        addLogMessage(`퀘스트 상태 변경: "${quest.title}" - ${statusText}`);

        if (quest.completed) {
            showNotification('퀘스트 완료! 🎉', 'success');
        }
    }
}

// 퀘스트 삭제
function deleteQuest(questId) {
    const questIndex = quests.findIndex(q => q.id === questId);
    if (questIndex === -1) return;

    const questTitle = quests[questIndex].title;

    if (confirm(`"${questTitle}" 퀘스트를 삭제하시겠습니까?`)) {
        quests.splice(questIndex, 1);
        saveQuestsToStorage();
        renderQuests();
        updateCategoryStats();
        updateQuestStats();

        addLogMessage(`퀘스트가 삭제되었습니다: ${questTitle}`);
        showNotification('퀘스트가 삭제되었습니다.', 'info');
    }
}

// 퀘스트 렌더링
function renderQuests() {
    if (!questList) return;

    questList.innerHTML = '';

    // 현재 필터에 맞는 퀘스트들 가져오기
    let filteredQuests = quests;
    if (currentQuestFilter !== 'all') {
        filteredQuests = quests.filter(quest => quest.category === currentQuestFilter);
    }

    if (filteredQuests.length === 0) {
        questList.innerHTML = `
            <div class="empty-state">
                <p>표시할 퀘스트가 없습니다</p>
            </div>
        `;
        return;
    }

    filteredQuests.forEach(quest => {
        const questElement = createQuestElement(quest);
        questList.appendChild(questElement);
    });
}

// 퀘스트 HTML 요소 생성
function createQuestElement(quest) {
    const questItem = document.createElement('div');
    questItem.className = `task-item ${quest.completed ? 'completed' : ''}`;
    questItem.dataset.id = quest.id;

    const difficultyStars = Array.from({ length: 5 }, (_, i) =>
        `<div class="difficulty-star ${i < quest.difficulty ? 'filled' : ''}"></div>`
    ).join('');

    questItem.innerHTML = `
        <div class="task-header">
            <div class="task-checkbox ${quest.completed ? 'checked' : ''}" onclick="toggleQuest(${quest.id})"></div>
            <div class="task-title">${quest.title}</div>
            <div class="task-priority priority-${quest.priority}">${priorityLabels[quest.priority]}</div>
        </div>
        <div class="task-meta">
            <div class="task-category">
                <span>${categoryIcons[quest.category]}</span>
                <span>${categoryNames[quest.category]}</span>
            </div>
            <div class="task-difficulty">
                ${difficultyStars}
            </div>
            <div class="task-actions">
                <button class="task-button delete" onclick="deleteQuest(${quest.id})">
                    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                        <polyline points="3 6 5 6 21 6"></polyline>
                        <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
                    </svg>
                </button>
            </div>
        </div>
    `;

    return questItem;
}

// 카테고리 통계 업데이트
function updateCategoryStats() {
    const categoryCards = document.querySelectorAll('.category-card');

    categoryCards.forEach(card => {
        const category = card.dataset.category;
        const categoryQuests = quests.filter(q => q.category === category);
        const completedQuests = categoryQuests.filter(q => q.completed);
        const progress = categoryQuests.length > 0 ? (completedQuests.length / categoryQuests.length) * 100 : 0;

        const countElement = card.querySelector('.category-count');
        const progressFill = card.querySelector('.progress-fill');
        const progressText = card.querySelector('.progress-text');

        if (countElement) countElement.textContent = categoryQuests.length;
        if (progressFill) progressFill.style.width = `${progress}%`;
        if (progressText) progressText.textContent = `${completedQuests.length}/${categoryQuests.length}`;
    });
}

// 퀘스트 통계 업데이트
function updateQuestStats() {
    const total = quests.length;
    const completed = quests.filter(q => q.completed).length;
    const active = total - completed;
    const completionRateValue = total > 0 ? Math.round((completed / total) * 100) : 0;
    const experience = completed * 125 + active * 25; // 완료된 퀘스트당 125xp, 진행중 25xp

    if (totalQuests) totalQuests.textContent = total;
    if (completedQuests) completedQuests.textContent = completed;
    if (activeQuests) activeQuests.textContent = active;
    if (completionRate) completionRate.textContent = `${completionRateValue}%`;
    if (experiencePoints) experiencePoints.textContent = experience.toLocaleString();
}

// 퀘스트 필터링
function filterQuests(category) {
    currentQuestFilter = category;
    renderQuests();
}

// 퀘스트를 로컬 스토리지에 저장
function saveQuestsToStorage() {
    try {
        localStorage.setItem('quests', JSON.stringify(quests));
        localStorage.setItem('questIdCounter', questIdCounter.toString());
    } catch (e) {
        console.error('퀘스트 저장 실패:', e);
    }
}

// 로컬 스토리지에서 퀘스트 로드
function loadQuestsFromStorage() {
    try {
        const savedQuests = localStorage.getItem('quests');
        const savedCounter = localStorage.getItem('questIdCounter');

        if (savedQuests) {
            quests = JSON.parse(savedQuests);
        }

        if (savedCounter) {
            questIdCounter = parseInt(savedCounter);
        }
    } catch (e) {
        console.error('퀘스트 로드 실패:', e);
        quests = [];
        questIdCounter = 1;
    }
}

// 전역 함수들 (HTML에서 호출되는 함수들)
window.toggleQuest = toggleQuest;
window.deleteQuest = deleteQuest;
window.closeQuestModal = closeQuestModal;