// 타이머 관련 변수 및 함수를 완전히 클라이언트 중심으로 재구성
// BUG FIX: setInterval 대신 Date.now() 기반 실제 시각 추적으로 최소화 시 타이머 동작 보장

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
const timerStartEl = document.getElementById('timer-start');
const timerEndEl = document.getElementById('timer-end');
const timerProgressFill = document.getElementById('timer-progress-fill');
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

// 상태 변수
let isRunning = false;
let currentMode = ModeDaeyaEnter;
let currentTimeOption = TimeOption3Hour;
let darkMode = true;
let soundEnabled = true;
let autoStartup = false;
let currentContentSection = 'main';
let countdownInterval = null;
let countdownTime = 3 * 60 * 60;
let statusCheckInterval = null;
let timerPaused = false;
let serverTimerStarted = false;

// BUG FIX: 실제 시각 기반 타이머 변수
let countdownEndTime = null;    // 카운트다운 종료 예정 시각 (Date.now 기반)
let countdownPausedRemaining = null; // 일시정지 시 남은 시간 (ms)

// 로그 관련 변수
let logAutoRefresh = true;
let showDebugLogs = false;
let logFilterText = '';
let logRefreshInterval = null;
let lastLogLength = 0;

// 초기화
document.addEventListener('DOMContentLoaded', () => {
    setTheme(darkMode);
    setupNavigation();
    setupInitialSelections();
    setupButtonListeners();
    setupSettingsListeners();
    setupLogListeners();
    updateCountdownDisplay(getHoursFromOption(currentTimeOption) * 60 * 60);
    addLogMessage('프로그램이 시작되었습니다.');
    setupStatusPolling();
    setupLogAutoRefresh();

    if (logsContainer && currentContentSection === 'logs') {
        refreshLogs();
    }
});

// 상태 확인 폴링 설정
function setupStatusPolling() {
    checkApiStatus();
    statusCheckInterval = setInterval(checkApiStatus, 2000);
}

// API를 사용하여 상태 확인
function checkApiStatus() {
    fetch('/api/status')
        .then(response => response.json())
        .then(data => {
            if (data.running !== isRunning) {
                if (!data.running && !timerPaused) {
                    isRunning = false;
                    statusText.textContent = '준비됨';
                    statusIndicator.classList.remove('running');
                    startBtn.classList.remove('active');
                    stopBtn.classList.remove('active');
                    resetCountdown();
                } else if (data.running && !isRunning && !timerPaused) {
                    isRunning = true;
                    serverTimerStarted = true;
                    statusText.textContent = '실행 중';
                    statusIndicator.classList.add('running');
                    startBtn.classList.add('active');
                    stopBtn.classList.remove('active');

                    if (!countdownInterval && !timerPaused) {
                        startCountdown(getHoursFromOption(currentTimeOption) * 60 * 60);
                    }
                }
            }
        })
        .catch(() => {});
}

// 네비게이션 기능 설정
function setupNavigation() {
    navButtons.forEach(button => {
        button.addEventListener('click', () => {
            const section = button.dataset.section;
            changeContentSection(section);

            if (section === 'logs' && logsContainer) {
                refreshLogs();
            }
        });
    });
}

// 컨텐츠 섹션 변경
function changeContentSection(section) {
    currentContentSection = section;

    navButtons.forEach(btn => {
        if (btn.dataset.section === section) {
            btn.classList.add('active');
        } else {
            btn.classList.remove('active');
        }
    });

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
    modeOptions[0].checked = true;
    timeOptions[2].checked = true;

    modeOptions.forEach(option => {
        option.addEventListener('change', (e) => {
            currentMode = parseInt(e.target.value);
            setModeApi(currentMode);
            addLogMessage(`${getModeName(currentMode)} 모드 선택됨`);

            // 칸첸 모드일 때만 아이템 습득 카드 표시
            const pickupCard = document.getElementById('item-pickup-card');
            if (pickupCard) {
                const isKanchen = (currentMode === ModeKanchenEnter || currentMode === ModeKanchenParty);
                pickupCard.style.display = isKanchen ? '' : 'none';
            }
        });
    });

    timeOptions.forEach(option => {
        option.addEventListener('change', (e) => {
            currentTimeOption = parseInt(e.target.value);
            setTimeOptionApi(currentTimeOption);

            if (!isRunning && !timerPaused) {
                let hours = getHoursFromOption(currentTimeOption);
                countdownTime = hours * 60 * 60;
                updateCountdownDisplay(countdownTime);
            }

            addLogMessage(`${getHoursFromOption(currentTimeOption)}시간 실행 설정됨`);
        });
    });
}

// 버튼 이벤트 리스너 설정
function setupButtonListeners() {
    // 시작 버튼
    startBtn.addEventListener('click', () => {
        if (!isRunning) {
            try {
                startBtn.classList.add('active');
                const wasTimerPaused = timerPaused;
                timerPaused = false;
                stopBtn.classList.remove('active');
                startOperation(wasTimerPaused);
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
                stopBtn.classList.add('active');
                stopOperation();
                isRunning = false;
                timerPaused = true;
                statusText.textContent = '일시정지';
                statusIndicator.classList.remove('running');
                statusIndicator.classList.add('paused');
                startBtn.classList.remove('active');

                // BUG FIX: 남은 시간 저장 (실제 시각 기반)
                if (countdownEndTime) {
                    countdownPausedRemaining = countdownEndTime - Date.now();
                    if (countdownPausedRemaining < 0) countdownPausedRemaining = 0;
                    countdownTime = Math.ceil(countdownPausedRemaining / 1000);
                }

                if (countdownInterval) {
                    clearInterval(countdownInterval);
                    countdownInterval = null;
                    timerDisplay.classList.remove('running');
                }
                countdownEndTime = null;

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
                timerPaused = false;
                stopBtn.classList.remove('active');
                statusIndicator.classList.remove('paused');

                const hours = getHoursFromOption(currentTimeOption);
                countdownTime = hours * 60 * 60;
                countdownEndTime = null;
                countdownPausedRemaining = null;
                updateCountdownDisplay(countdownTime);
                timerDisplay.classList.remove('running');
                if (timerStartEl) timerStartEl.textContent = '--:--';
                if (timerEndEl) timerEndEl.textContent = '--:--';

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
    darkModeToggle.addEventListener('change', () => {
        darkMode = darkModeToggle.checked;
        setTheme(darkMode);
        addLogMessage(`다크 모드: ${darkMode ? '켜짐' : '꺼짐'}`);
    });

    soundToggle.addEventListener('change', () => {
        soundEnabled = soundToggle.checked;
        addLogMessage(`소리 알림: ${soundEnabled ? '켜짐' : '꺼짐'}`);
    });

    startupToggle.addEventListener('change', () => {
        autoStartup = startupToggle.checked;
        addLogMessage(`시작 시 자동 실행: ${autoStartup ? '켜짐' : '꺼짐'}`);
        setAutoStartupApi(autoStartup);
    });
}

// 로그 관련 이벤트 리스너 설정
function setupLogListeners() {
    if (!refreshLogsBtn || !clearLogsBtn || !autoRefreshToggle || !showDebugToggle || !logFilterInput) {
        return;
    }

    refreshLogsBtn.addEventListener('click', () => refreshLogs());
    clearLogsBtn.addEventListener('click', () => clearLogs());

    autoRefreshToggle.addEventListener('change', () => {
        logAutoRefresh = autoRefreshToggle.checked;
        if (logAutoRefresh) {
            setupLogAutoRefresh();
        } else {
            clearInterval(logRefreshInterval);
        }
    });

    showDebugToggle.addEventListener('change', () => {
        showDebugLogs = showDebugToggle.checked;
        refreshLogs();
    });

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
        }, 10000);
    }
}

// 로그 새로고침 및 표시 함수들
function refreshLogs() {
    if (!logsContainer) return;

    fetch('/api/logs')
        .then(response => response.json())
        .then(data => displayLogs(data.logs))
        .catch(() => {
            logsContainer.innerHTML = '<p class="log-placeholder">로그를 불러올 수 없습니다.</p>';
        });
}

function displayLogs(logs) {
    if (!logsContainer) return;

    if (!logs || logs.length === 0) {
        logsContainer.innerHTML = '<p class="log-placeholder">로그가 없습니다.</p>';
        return;
    }

    logsContainer.innerHTML = '';

    logs.forEach(log => {
        if (logFilterText && !log.toLowerCase().includes(logFilterText)) return;
        if (!showDebugLogs && isDebugLog(log)) return;

        const logEntry = document.createElement('pre');
        logEntry.className = 'log-entry ' + getLogLevel(log);
        logEntry.textContent = log;
        logsContainer.appendChild(logEntry);
    });

    logsContainer.scrollTop = logsContainer.scrollHeight;
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
        .catch(() => addLogMessage("로그 파일을 지울 수 없습니다."));
}

function getLogLevel(log) {
    const lowerLog = log.toLowerCase();
    if (lowerLog.includes('error') || lowerLog.includes('오류') || lowerLog.includes('실패')) return 'error';
    if (lowerLog.includes('warn') || lowerLog.includes('경고')) return 'warning';
    if (isDebugLog(log)) return 'debug';
    return 'info';
}

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
    if (miniLog) {
        miniLog.textContent = message;
    }

    try {
        fetch('/api/log', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ message: message })
        }).catch(() => {});
    } catch (e) {}
}

// 모드 이름 가져오기
function getModeName(mode) {
    switch (mode) {
        case ModeDaeyaEnter: return '대야 (입장)';
        case ModeDaeyaParty: return '대야 (파티)';
        case ModeKanchenEnter: return '칸첸 (입장)';
        case ModeKanchenParty: return '칸첸 (파티)';
        default: return '알 수 없음';
    }
}

function getApiModeName(mode) {
    switch (mode) {
        case ModeDaeyaEnter: return 'daeya-entrance';
        case ModeDaeyaParty: return 'daeya-party';
        case ModeKanchenEnter: return 'kanchen-entrance';
        case ModeKanchenParty: return 'kanchen-party';
        default: return '';
    }
}

function getHoursFromOption(option) {
    switch (option) {
        case TimeOption1Hour: return 1;
        case TimeOption2Hour: return 2;
        case TimeOption3Hour: return 3;
        case TimeOption4Hour: return 4;
        default: return 3;
    }
}

// 카운트다운 표시 업데이트
function updateCountdownDisplay(seconds) {
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const secs = seconds % 60;

    timerDisplay.textContent =
        `${hours.toString().padStart(2, '0')}:${minutes.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;

    // 프로그레스 바 업데이트
    const totalSeconds = getHoursFromOption(currentTimeOption) * 60 * 60;
    const progress = totalSeconds > 0 ? (seconds / totalSeconds) : 1;
    if (timerProgressFill) {
        timerProgressFill.style.width = (progress * 100) + '%';
    }
}

// 시:분 포맷 헬퍼
function formatHM(date) {
    const h = date.getHours().toString().padStart(2, '0');
    const m = date.getMinutes().toString().padStart(2, '0');
    return `${h}:${m}`;
}

// BUG FIX: 카운트다운 시작 - Date.now() 기반 실제 시각 추적
function startCountdown(seconds) {
    if (countdownInterval) {
        clearInterval(countdownInterval);
    }

    // 일시정지 복귀라면 저장된 남은 시간 사용, 아니면 새로 설정
    if (timerPaused && countdownPausedRemaining !== null) {
        countdownEndTime = Date.now() + countdownPausedRemaining;
        countdownPausedRemaining = null;
    } else {
        countdownTime = seconds;
        countdownEndTime = Date.now() + (seconds * 1000);
    }

    // 시작/종료 시각 표시
    const startTime = new Date();
    const endTime = new Date(countdownEndTime);
    if (timerStartEl) timerStartEl.textContent = formatHM(startTime);
    if (timerEndEl) timerEndEl.textContent = formatHM(endTime);

    updateCountdownDisplay(countdownTime);
    timerDisplay.classList.add('running');

    // BUG FIX: 매 틱마다 실제 남은 시간을 Date.now() 기준으로 계산
    // setInterval이 throttle되어도 정확한 시간 표시
    countdownInterval = setInterval(() => {
        const now = Date.now();
        const remainingMs = countdownEndTime - now;

        if (remainingMs <= 0) {
            countdownTime = 0;
            updateCountdownDisplay(0);
            clearInterval(countdownInterval);
            countdownInterval = null;
            countdownEndTime = null;
            stopOperation();
            addLogMessage("설정한 시간이 경과하여 자동으로 종료되었습니다.");
        } else {
            countdownTime = Math.ceil(remainingMs / 1000);
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
    timerDisplay.classList.remove('running');
}

// 카운트다운 리셋
function resetCountdown() {
    if (countdownInterval) {
        clearInterval(countdownInterval);
        countdownInterval = null;
    }

    const hours = getHoursFromOption(currentTimeOption);
    countdownTime = hours * 60 * 60;
    countdownEndTime = null;
    countdownPausedRemaining = null;
    updateCountdownDisplay(countdownTime);
    timerDisplay.classList.remove('running');
    timerPaused = false;

    // 시작/종료 시각 초기화
    if (timerStartEl) timerStartEl.textContent = '--:--';
    if (timerEndEl) timerEndEl.textContent = '--:--';
}

// 이벤트 수신 함수
window.dispatchAppEvent = function(event) {
    const { type, payload } = event;

    switch (type) {
        case 'updateTimer':
            break;
        case 'logMessage':
            addLogMessage(payload.message);
            break;
        case 'operationStatus':
            if (payload.running !== isRunning) {
                if (payload.running) {
                    if (!isRunning && !timerPaused) {
                        isRunning = true;
                        serverTimerStarted = true;
                        statusText.textContent = '실행 중';
                        statusIndicator.classList.add('running');
                        startBtn.classList.add('active');

                        if (!countdownInterval && !timerPaused) {
                            startCountdown(getHoursFromOption(currentTimeOption) * 60 * 60);
                        }
                    }
                } else {
                    if (isRunning && !timerPaused) {
                        isRunning = false;
                        statusText.textContent = '준비됨';
                        statusIndicator.classList.remove('running');
                        startBtn.classList.remove('active');
                        stopBtn.classList.remove('active');
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
            resetCountdown();
            break;
        case 'appVersion':
            updateAppVersion(payload.version, payload.buildDate);
            break;
    }
};

function resetModeSelection(mode) {
    currentMode = mode;
    modeOptions.forEach(option => {
        option.checked = parseInt(option.value) === mode;
    });
}

function resetTimeOptionSelection(option) {
    currentTimeOption = option;
    timeOptions.forEach(opt => {
        opt.checked = parseInt(opt.value) === option;
    });

    if (!isRunning && !timerPaused) {
        const hours = getHoursFromOption(option);
        countdownTime = hours * 60 * 60;
        updateCountdownDisplay(countdownTime);
    }
}

function updateAppVersion(version, date) {
    if (appVersion) appVersion.textContent = version;
    if (buildDate) buildDate.textContent = date;
}

// API 호출 관련 함수

function startOperation(wasTimerPaused) {
    if (currentMode === ModeNone) {
        addLogMessage("오류: 모드를 선택해야 합니다.");
        startBtn.classList.remove('active');
        isRunning = false;
        return;
    }

    const apiMode = getApiModeName(currentMode);
    if (!apiMode) {
        addLogMessage("오류: 유효하지 않은 모드입니다.");
        startBtn.classList.remove('active');
        isRunning = false;
        return;
    }

    const hours = getHoursFromOption(currentTimeOption);

    fetch('/api/start', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: `mode=${apiMode}&auto_stop=${hours}`
    })
    .then(response => {
        if (response.ok) {
            serverTimerStarted = true;

            if (!countdownInterval) {
                if (wasTimerPaused) {
                    startCountdown(countdownTime);
                } else {
                    startCountdown(hours * 60 * 60);
                }
            }

            addLogMessage(`${getModeName(currentMode)} 모드로 작업을 시작합니다...`);
        } else {
            throw new Error('작업 시작 실패');
        }
    })
    .catch(error => {
        addLogMessage("오류: 작업을 시작할 수 없습니다.");
        startBtn.classList.remove('active');
        isRunning = false;
        statusText.textContent = '준비됨';
        statusIndicator.classList.remove('running');

        if (wasTimerPaused) {
            timerPaused = true;
            stopBtn.classList.add('active');
        }
    });
}

function stopOperation() {
    fetch('/api/stop', { method: 'POST' })
    .then(response => {
        if (!response.ok) throw new Error('작업 중지 실패');
    })
    .catch(error => addLogMessage("오류: 작업을 중지할 수 없습니다."));
}

function resetSettingsApi() {
    resetCountdown();
    resetModeSelection(ModeDaeyaEnter);
    resetTimeOptionSelection(TimeOption3Hour);
    timerPaused = false;
    stopBtn.classList.remove('active');
    statusIndicator.classList.remove('paused');

    fetch('/api/reset', { method: 'POST' }).catch(() => {});
}

function exitApplicationApi() {
    fetch('/api/exit', { method: 'POST' }).catch(() => {
        exitBtn.classList.remove('active');
    });
}

function setModeApi(mode) {
    const apiMode = getApiModeName(mode);
    fetch('/api/settings', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: `type=mode&value=${apiMode}`
    }).catch(() => {});
}

function setTimeOptionApi(option) {
    const hours = getHoursFromOption(option);
    fetch('/api/settings', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: `type=time&value=${hours}`
    }).catch(() => {});
}

function setAutoStartupApi(enabled) {
    fetch('/api/settings', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: `type=auto_startup&value=${enabled ? 1 : 0}`
    }).catch(() => {});
}

// === 아이템 자동 습득 설정 (다중 아이템) ===
(function() {
    const pickupCard = document.getElementById('item-pickup-card');
    const pickupToggle = document.getElementById('item-pickup-toggle');
    const pickupList = document.getElementById('item-pickup-list');
    const pickupAddBtn = document.getElementById('item-pickup-add-btn');
    const pickupInterval = document.getElementById('item-pickup-interval');
    const pickupIntervalDisplay = document.getElementById('item-pickup-interval-display');
    const pickupTileW = document.getElementById('item-pickup-tile-w');
    const pickupTileH = document.getElementById('item-pickup-tile-h');
    const pickupOriginX = document.getElementById('item-pickup-origin-x');
    const pickupOriginY = document.getElementById('item-pickup-origin-y');
    const pickupTargetMap = document.getElementById('item-pickup-target-map');
    const pickupWrongMap = document.getElementById('item-pickup-wrong-map');
    const pickupSkillKeys = document.getElementById('item-pickup-skill-keys');
    const pickupSaveBtn = document.getElementById('item-pickup-save-btn');
    const pickupTestBtn = document.getElementById('item-pickup-test-btn');
    const pickupTestResult = document.getElementById('item-pickup-test-result');

    // 아이템 리스트 데이터
    let itemList = [];

    // 아이템 행 렌더링
    function renderItemList() {
        if (!pickupList) return;
        pickupList.innerHTML = '';
        itemList.forEach((item, idx) => {
            const row = document.createElement('div');
            row.style.cssText = 'display:flex;gap:0.3rem;align-items:center';
            // 이름 입력
            const nameInput = document.createElement('input');
            nameInput.type = 'text';
            nameInput.value = item.name;
            nameInput.placeholder = '아이템 이름';
            nameInput.style.cssText = 'flex:1;padding:0.3rem 0.5rem;font-size:0.8rem;min-width:0;border:1px solid var(--border-color);border-radius:var(--radius-sm);background:var(--bg-color);color:var(--text-primary)';
            nameInput.addEventListener('input', () => { itemList[idx].name = nameInput.value; });
            // 색상 배지 버튼
            const colorBtn = document.createElement('button');
            colorBtn.style.cssText = 'padding:0.2rem 0.5rem;font-size:0.7rem;font-weight:600;border-radius:4px;border:1px solid;cursor:pointer;white-space:nowrap;min-width:40px;text-align:center';
            function updateColorBtn() {
                if (item.color === 'yellow') {
                    colorBtn.textContent = '노랑';
                    colorBtn.style.background = 'rgba(245,158,11,0.2)';
                    colorBtn.style.color = '#f59e0b';
                    colorBtn.style.borderColor = 'rgba(245,158,11,0.4)';
                } else {
                    colorBtn.textContent = '초록';
                    colorBtn.style.background = 'rgba(34,197,94,0.2)';
                    colorBtn.style.color = '#22c55e';
                    colorBtn.style.borderColor = 'rgba(34,197,94,0.4)';
                }
            }
            updateColorBtn();
            colorBtn.addEventListener('click', () => {
                itemList[idx].color = item.color === 'green' ? 'yellow' : 'green';
                item.color = itemList[idx].color;
                updateColorBtn();
            });
            // 삭제 버튼
            const delBtn = document.createElement('button');
            delBtn.textContent = 'X';
            delBtn.style.cssText = 'padding:0.2rem 0.4rem;font-size:0.7rem;border:1px solid var(--border-color);border-radius:4px;background:none;color:var(--text-muted);cursor:pointer';
            delBtn.addEventListener('click', () => {
                itemList.splice(idx, 1);
                renderItemList();
            });
            row.appendChild(nameInput);
            row.appendChild(colorBtn);
            row.appendChild(delBtn);
            pickupList.appendChild(row);
        });
    }

    // 아이템 추가 버튼
    if (pickupAddBtn) {
        pickupAddBtn.addEventListener('click', () => {
            itemList.push({ name: '', color: 'green' });
            renderItemList();
        });
    }

    // 슬라이더 값 실시간 표시
    if (pickupInterval) {
        pickupInterval.addEventListener('input', () => {
            if (pickupIntervalDisplay) pickupIntervalDisplay.textContent = pickupInterval.value;
        });
    }

    // 복귀좌표 입력 제한: 숫자만, 3자리 max (0~999)
    [pickupOriginX, pickupOriginY].forEach(el => {
        if (!el) return;
        el.addEventListener('input', () => {
            el.value = el.value.replace(/[^0-9]/g, '').slice(0, 3);
        });
        el.addEventListener('blur', () => {
            let v = parseInt(el.value) || 0;
            if (v > 999) v = 999;
            if (v < 0) v = 0;
            el.value = v;
        });
    });

    // 초기 로드 시 설정 불러오기
    async function loadItemPickupConfig() {
        try {
            const res = await fetch('/api/item-pickup/config');
            if (!res.ok) return;
            const data = await res.json();
            if (pickupToggle) pickupToggle.checked = data.enabled || false;
            // Items 배열 로드
            if (data.items && Array.isArray(data.items) && data.items.length > 0) {
                itemList = data.items.map(it => ({ name: it.name || '', color: it.color || 'green' }));
            } else {
                // 기본 아이템
                itemList = [
                    { name: '설산의보석', color: 'green' },
                    { name: '찬란한설산의보석', color: 'green' },
                    { name: '찬란한아그니의적영', color: 'yellow' },
                ];
            }
            renderItemList();
            if (pickupInterval && data.scanInterval) {
                pickupInterval.value = data.scanInterval;
                if (pickupIntervalDisplay) pickupIntervalDisplay.textContent = data.scanInterval;
            }
            if (pickupTileW && data.tilePixelW) pickupTileW.value = data.tilePixelW;
            if (pickupTileH && data.tilePixelH) pickupTileH.value = data.tilePixelH;
            if (pickupOriginX) pickupOriginX.value = data.originX || 0;
            if (pickupOriginY) pickupOriginY.value = data.originY || 0;
            if (pickupTargetMap) pickupTargetMap.value = data.targetMap || '';
            if (pickupWrongMap) pickupWrongMap.value = data.wrongMap || '';
            if (pickupSkillKeys) pickupSkillKeys.value = (data.skillKeys && data.skillKeys.length > 0) ? data.skillKeys.join(',') : '';
        } catch(e) {}
    }

    // UI에서 아이템 리스트 수집
    function collectItems() {
        return itemList.filter(it => it.name.trim() !== '').map(it => ({
            name: it.name.trim(),
            color: it.color || 'green'
        }));
    }

    // 설정 저장
    if (pickupSaveBtn) {
        pickupSaveBtn.addEventListener('click', async () => {
            const items = collectItems();
            if (items.length === 0) {
                addLogMessage('감지할 아이템을 최소 1개 입력하세요.');
                return;
            }
            const cfg = {
                enabled: pickupToggle ? pickupToggle.checked : false,
                items: items,
                scanInterval: pickupInterval ? parseInt(pickupInterval.value) : 1,
                tilePixelW: pickupTileW ? parseInt(pickupTileW.value) : 48,
                tilePixelH: pickupTileH ? parseInt(pickupTileH.value) : 48,
                originX: pickupOriginX ? parseInt(pickupOriginX.value) || 0 : 0,
                originY: pickupOriginY ? parseInt(pickupOriginY.value) || 0 : 0,
                targetMap: pickupTargetMap ? pickupTargetMap.value.trim() : '',
                wrongMap: pickupWrongMap ? pickupWrongMap.value.trim() : '',
                skillKeys: pickupSkillKeys ? pickupSkillKeys.value.split(',').map(k => k.trim()).filter(k => k) : [],
            };
            try {
                const res = await fetch('/api/item-pickup/config', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(cfg)
                });
                if (res.ok) {
                    addLogMessage('아이템 습득 설정이 저장되었습니다. (' + items.length + '개 아이템)');
                } else {
                    addLogMessage('아이템 습득 설정 저장 실패');
                }
            } catch(e) {
                addLogMessage('아이템 습득 설정 저장 실패: ' + e.message);
            }
        });
    }

    // 테스트 스캔
    if (pickupTestBtn) {
        pickupTestBtn.addEventListener('click', async () => {
            if (!pickupTestResult) return;
            pickupTestResult.style.display = 'block';
            pickupTestResult.textContent = '스캔 중...';
            pickupTestResult.className = 'ocr-test-result';

            try {
                const res = await fetch('/api/item-pickup/test-scan');
                const data = await res.json();
                if (data.error) {
                    pickupTestResult.textContent = '오류: ' + data.error;
                    pickupTestResult.className = 'ocr-test-result error';
                } else {
                    let msg = data.message || '결과 없음';
                    if (data.words && data.words.length > 0) {
                        msg += '\n\n인식된 단어 (' + data.words.length + '개):';
                        data.words.slice(0, 20).forEach(w => {
                            msg += '\n  "' + w.Text + '" @ (' + Math.round(w.X) + ',' + Math.round(w.Y) + ')';
                        });
                        if (data.words.length > 20) {
                            msg += '\n  ... 외 ' + (data.words.length - 20) + '개';
                        }
                    }
                    pickupTestResult.innerHTML = '';
                    pickupTestResult.className = 'ocr-test-result' + (msg.includes('발견') ? ' success' : '');
                    if (data.markedImage) {
                        const img = document.createElement('img');
                        img.src = data.markedImage;
                        img.style.cssText = 'max-width:100%;border-radius:4px;margin-bottom:0.5rem';
                        pickupTestResult.appendChild(img);
                    }
                    const pre = document.createElement('pre');
                    pre.style.cssText = 'margin:0;white-space:pre-wrap;font-size:0.85rem';
                    pre.textContent = msg;
                    pickupTestResult.appendChild(pre);
                }
            } catch(e) {
                pickupTestResult.textContent = '테스트 실패: ' + e.message;
                pickupTestResult.className = 'ocr-test-result error';
            }
        });
    }

    // 좌표 OCR 테스트
    const pickupCoordBtn = document.getElementById('item-pickup-coord-btn');
    if (pickupCoordBtn) {
        pickupCoordBtn.addEventListener('click', async () => {
            if (!pickupTestResult) return;
            pickupTestResult.style.display = 'block';
            pickupTestResult.textContent = '좌표 인식 중...';
            pickupTestResult.className = 'ocr-test-result';

            try {
                const res = await fetch('/api/item-pickup/test-coords');
                const data = await res.json();
                pickupTestResult.innerHTML = '';
                if (data.success) {
                    pickupTestResult.className = 'ocr-test-result success';
                    const msg = document.createElement('div');
                    msg.textContent = '좌표 인식 성공: X=' + data.x + ', Y=' + data.y + ' (화면: ' + data.imgWidth + 'x' + data.imgHeight + ')';
                    pickupTestResult.appendChild(msg);
                } else {
                    pickupTestResult.className = 'ocr-test-result error';
                    const msg = document.createElement('div');
                    msg.textContent = '좌표 인식 실패: ' + (data.error || '알 수 없음') + ' (화면: ' + (data.imgWidth||'?') + 'x' + (data.imgHeight||'?') + ')';
                    pickupTestResult.appendChild(msg);
                }
                if (data.ocrResults && data.ocrResults.length > 0) {
                    const ocrLabel = document.createElement('div');
                    ocrLabel.textContent = 'OCR 결과:';
                    ocrLabel.style.cssText = 'font-size:0.75rem;color:var(--text-muted);margin-top:0.3rem';
                    pickupTestResult.appendChild(ocrLabel);
                    data.ocrResults.forEach(r => {
                        const line = document.createElement('div');
                        line.textContent = r;
                        line.style.cssText = 'font-size:0.7rem;color:var(--text-muted);font-family:monospace;padding-left:0.5rem';
                        pickupTestResult.appendChild(line);
                    });
                }
                if (data.cropImage) {
                    const label = document.createElement('div');
                    label.textContent = '우하단 크롭 영역:';
                    label.style.cssText = 'font-size:0.75rem;color:var(--text-muted);margin-top:0.3rem';
                    pickupTestResult.appendChild(label);
                    const img = document.createElement('img');
                    img.src = data.cropImage;
                    img.style.cssText = 'max-width:100%;border:1px solid var(--text-muted);border-radius:4px';
                    pickupTestResult.appendChild(img);
                }
            } catch(e) {
                pickupTestResult.textContent = '좌표 테스트 실패: ' + e.message;
                pickupTestResult.className = 'ocr-test-result error';
            }
        });
    }

    loadItemPickupConfig();
})();

// 키 맵핑 탭은 keymapping.js에서 처리

// === 텔레그램 설정 ===
(function() {
    const enabledToggle = document.getElementById('telegram-enabled-toggle');
    const tokenInput = document.getElementById('telegram-token');
    const chatIdInput = document.getElementById('telegram-chat-id');
    const saveBtn = document.getElementById('telegram-save-btn');
    const testBtn = document.getElementById('telegram-test-btn');
    const statusEl = document.getElementById('telegram-status');
    const configForm = document.getElementById('telegram-config-form');

    function showStatus(msg, isError) {
        if (!statusEl) return;
        statusEl.textContent = msg;
        statusEl.className = 'telegram-status ' + (isError ? 'error' : 'success');
        setTimeout(() => { statusEl.textContent = ''; }, 5000);
    }

    // 초기 로드
    async function loadTelegramConfig() {
        try {
            const res = await fetch('/api/telegram/config');
            const data = await res.json();
            if (enabledToggle) enabledToggle.checked = data.enabled || false;
            if (chatIdInput && data.chat_id) chatIdInput.value = data.chat_id;
            if (configForm) configForm.style.display = data.enabled ? '' : 'none';
        } catch(e) {}
    }

    if (enabledToggle) {
        enabledToggle.addEventListener('change', async () => {
            const enabled = enabledToggle.checked;
            if (configForm) configForm.style.display = enabled ? '' : 'none';
            try {
                await fetch('/api/telegram/toggle', {
                    method: 'POST',
                    headers: {'Content-Type':'application/json'},
                    body: JSON.stringify({ enabled })
                });
            } catch(e) {}
        });
    }

    if (saveBtn) {
        saveBtn.addEventListener('click', async () => {
            const token = tokenInput?.value.trim() || '';
            const chatId = chatIdInput?.value.trim() || '';
            if (!token || !chatId) {
                showStatus('봇 토큰과 채팅 ID를 모두 입력하세요.', true);
                return;
            }
            try {
                const res = await fetch('/api/telegram/config', {
                    method: 'POST',
                    headers: {'Content-Type':'application/json'},
                    body: JSON.stringify({ token, chat_id: chatId, enabled: true })
                });
                if (res.ok) {
                    if (enabledToggle) enabledToggle.checked = true;
                    showStatus('텔레그램 설정이 저장되었습니다.', false);
                } else {
                    showStatus('저장 실패', true);
                }
            } catch(e) {
                showStatus('저장 실패: ' + e.message, true);
            }
        });
    }

    if (testBtn) {
        testBtn.addEventListener('click', async () => {
            try {
                const res = await fetch('/api/telegram/test', { method: 'POST' });
                if (res.ok) {
                    showStatus('테스트 메시지가 전송되었습니다!', false);
                } else {
                    const text = await res.text();
                    showStatus('테스트 실패: ' + text, true);
                }
            } catch(e) {
                showStatus('테스트 실패: ' + e.message, true);
            }
        });
    }

    loadTelegramConfig();
})();

