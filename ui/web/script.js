// 타이머 관련 변수 및 함수를 완전히 클라이언트 중심으로 재구성
// BUG FIX: setInterval 대신 Date.now() 기반 실제 시각 추적으로 최소화 시 타이머 동작 보장

const ModeNone = 0;
const ModeDaeyaEnter = 1;
const ModeDaeyaParty = 2;
const ModeKanchenEnter = 3;
const ModeKanchenParty = 4;
const ModeTrialSolo = 5; // 시련 탭에서 별도 관리
const ModeTrialGroup = 6;

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
    setupMultiEntry();
    setupBaramlog();
    setupDaeyaConfig();
    setupCollapsibles();
    setupGlyphLearn();

    if (logsContainer && currentContentSection === 'logs') {
        refreshLogs();
    }
});

// 카드 표시 갱신 — 다중창 목록에서 칸첸으로 지정된 창이 있으면
// 아이템습득 카드 + 칸첸 복귀좌표 행을 표시 (모드 라디오는 제거됨)
function updateModeCards() {
    updateMultiCenterRow();
}

function anyKanchenSelected() {
    return Array.from(document.querySelectorAll('#multi-entry-list select.multi-mode-select'))
        .some(s => s.value === 'kanchen');
}

function updateMultiCenterRow() {
    const anyKanchen = anyKanchenSelected();
    const centerRow = document.getElementById('multi-center-row');
    if (centerRow) centerRow.style.display = anyKanchen ? 'flex' : 'none';
    const pickupCard = document.getElementById('item-pickup-card');
    if (pickupCard) pickupCard.style.display = anyKanchen ? '' : 'none';
}

// 카드 접기/펴기.
// 설정 카드들이 항상 펼쳐져 있으면 화면을 잡아먹어 불편하므로 기본은 접힘이고,
// 접고 편 상태는 브라우저에 기억해 다음에도 유지한다.
// 글자 학습 — 사전에 없는 글자를 화면에서 배운다.
// 게임 폰트가 고정 비트맵이라, 한 번 배우면 그 글자는 이후 픽셀 단위로 정확히 읽힌다.
function setupGlyphLearn() {
    const toggle = document.getElementById('glyph-learn-toggle');
    const body = document.getElementById('glyph-learn-body');
    const sel = document.getElementById('glyph-learn-hwnd');
    const mapIn = document.getElementById('glyph-learn-map');
    const nickIn = document.getElementById('glyph-learn-nick');
    const saveBtn = document.getElementById('glyph-learn-save');
    const result = document.getElementById('glyph-learn-result');
    if (!toggle || !body || !sel || !saveBtn) return;

    function fillWindows() {
        const boxes = document.querySelectorAll('#multi-entry-list input[type="checkbox"]');
        const cur = sel.value;
        sel.innerHTML = '<option value="">창 선택</option>' +
            Array.from(boxes).map((b, i) => `<option value="${b.value}">창 ${i + 1}</option>`).join('');
        if (cur) sel.value = cur;
    }

    async function showStatus() {
        try {
            const res = await fetch('/api/glyph');
            const s = await res.json();
            result.textContent = `사전 ${s.total}개 글리프 / 아는 글자 ${(s.chars || '').length}자`;
        } catch (e) { /* 상태 표시는 실패해도 무시 */ }
    }

    toggle.addEventListener('click', () => {
        const open = body.style.display === 'none';
        body.style.display = open ? '' : 'none';
        if (open) { fillWindows(); showStatus(); }
    });

    saveBtn.addEventListener('click', async () => {
        const hwnd = sel.value;
        if (!hwnd) { result.textContent = '창을 먼저 선택하세요.'; return; }
        const map = (mapIn.value || '').trim();
        const nick = (nickIn.value || '').trim();
        if (!map && !nick) { result.textContent = '맵 이름이나 닉네임 중 하나는 입력하세요.'; return; }
        saveBtn.disabled = true;
        const orig = saveBtn.textContent;
        saveBtn.textContent = '학습 중…';
        try {
            const res = await fetch('/api/glyph', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ hwnd: Number(hwnd), map, nick })
            });
            const s = await res.json();
            const notes = (s.notes || []).join(' / ');
            if (s.error) {
                result.textContent = `실패: ${s.error}${notes ? ' — ' + notes : ''}`;
            } else if (s.added > 0) {
                result.textContent = `새 글자 ${s.added}개 저장 (사전 ${s.total}개) ${notes}`;
                mapIn.value = '';
                nickIn.value = '';
            } else {
                result.textContent = notes || '새로 배운 글자가 없습니다 (이미 다 아는 글자).';
            }
        } catch (e) {
            result.textContent = '실패: ' + e.message;
        }
        saveBtn.textContent = orig;
        saveBtn.disabled = false;
    });
}

function setupCollapsibles() {
    document.querySelectorAll('.collapse-btn[data-collapse]').forEach(btn => {
        const bodyId = btn.dataset.collapse;
        const body = document.getElementById(bodyId);
        if (!body) return;
        const KEY = 'collapse:' + bodyId;

        function paint(open) {
            body.style.display = open ? '' : 'none';
            btn.textContent = open ? '▾' : '▸';
            btn.title = open ? '접기' : '펴기';
        }

        let open = false; // 기본 접힘
        try { open = localStorage.getItem(KEY) === 'open'; } catch (e) {}
        paint(open);

        btn.addEventListener('click', (e) => {
            e.preventDefault();
            e.stopPropagation(); // 헤더의 다른 조작(토글 스위치 등)과 분리
            open = body.style.display === 'none';
            paint(open);
            try { localStorage.setItem(KEY, open ? 'open' : 'closed'); } catch (e) {}
        });
    });
}

// 대야 전투 키 설정 — 어떤 키를 누르는지 보여주고 바꿀 수 있게 한다.
// 서버는 기본값(d, x, 5 / 29,32 / ±1)으로 시작하므로, 저장한 값이 있으면
// 화면이 뜰 때 서버에 다시 적용한다.
function setupDaeyaConfig() {
    const card = document.getElementById('daeya-config-card');
    if (!card) return;
    const keysEl = document.getElementById('daeya-skill-keys');
    const xEl = document.getElementById('daeya-target-x');
    const yEl = document.getElementById('daeya-target-y');
    const tolEl = document.getElementById('daeya-tolerance');
    const saveBtn = document.getElementById('daeya-config-save');
    const KEY = 'daeyaBattleConfig';

    function paint(cfg) {
        keysEl.value = (cfg.skillKeys || []).join(', ');
        xEl.value = cfg.targetX;
        yEl.value = cfg.targetY;
        tolEl.value = cfg.tolerance;
    }

    function read() {
        return {
            skillKeys: keysEl.value.split(',').map(s => s.trim()).filter(Boolean),
            targetX: parseInt(xEl.value) || 0,
            targetY: parseInt(yEl.value) || 0,
            tolerance: Math.max(0, parseInt(tolEl.value) || 0)
        };
    }

    async function post(cfg) {
        const r = await fetch('/api/daeya/config', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(cfg)
        });
        if (!r.ok) throw new Error(await r.text());
        return await r.json();
    }

    (async () => {
        try {
            const r = await fetch('/api/daeya/config');
            let cfg = await r.json();
            // 저장해둔 설정이 있으면 서버에 재적용 (서버는 재시작 시 기본값)
            let saved = null;
            try { saved = JSON.parse(localStorage.getItem(KEY)); } catch (e) {}
            if (saved && saved.skillKeys && saved.skillKeys.length) {
                cfg = await post(saved);
            }
            paint(cfg);
        } catch (e) { /* 서버 미응답 시 무시 */ }
    })();

    if (saveBtn) {
        saveBtn.addEventListener('click', async () => {
            const cfg = read();
            if (cfg.skillKeys.length === 0) { alert('스킬 키를 하나 이상 입력해주세요.'); return; }
            saveBtn.disabled = true;
            try {
                const applied = await post(cfg);
                paint(applied);
                try { localStorage.setItem(KEY, JSON.stringify(applied)); } catch (e) {}
                addLogMessage(`대야 설정 저장: 스킬 [${(applied.skillKeys || []).join(', ')}] / 목표 (${applied.targetX},${applied.targetY}) ±${applied.tolerance}`);
            } catch (e) {
                addLogMessage('대야 설정 저장 실패: ' + e.message);
            }
            saveBtn.disabled = false;
        });
    }
}

// 시련 모집 알림 화면 (전용 섹션)
// 서버가 10초마다 baramlog.com을 확인하고, 여기서는 5초마다 상태만 갱신한다.
// "새로고침"은 주기와 무관하게 서버에 즉시 동기화를 요청한다(서버가 조회를 마친 뒤 응답).
function setupBaramlog() {
    const section = document.getElementById('baramlog-section');
    if (!section) return;
    const badge = document.getElementById('baramlog-badge');
    const statsBox = document.getElementById('baramlog-stats');
    const targetsBox = document.getElementById('baramlog-targets');
    const errBox = document.getElementById('baramlog-error');
    const listBox = document.getElementById('baramlog-hits-list');
    const intervalEl = document.getElementById('baramlog-interval');
    const refreshBtn = document.getElementById('baramlog-refresh');
    const toggleBtn = document.getElementById('baramlog-toggle');
    const testBtn = document.getElementById('baramlog-test');
    const intervalSel = document.getElementById('baramlog-interval-select');
    const INTERVAL_KEY = 'baramlogIntervalSeconds';
    let appliedSavedInterval = false;
    let running = false; // 기본은 중지 — 서버도 자동 시작하지 않는다

    function stat(label, value, color) {
        return `<div class="result-item" style="background:rgba(255,255,255,0.04);border-radius:8px;padding:0.6rem">
            <div style="font-size:0.7rem;color:var(--text-muted)">${label}</div>
            <div style="font-size:0.95rem;font-weight:600;margin-top:0.2rem${color ? ';color:' + color : ''}">${value}</div>
        </div>`;
    }

    function paint(st) {
        running = !!st.running;

        let label, bg, fg;
        if (!st.running) { label = '중지됨'; bg = 'rgba(148,163,184,0.2)'; fg = '#94a3b8'; }
        else if (!st.healthy) { label = '접속 실패'; bg = 'rgba(248,113,113,0.18)'; fg = '#f87171'; }
        else if (st.stale) { label = '데이터 정체'; bg = 'rgba(245,166,35,0.18)'; fg = '#f5a623'; }
        else { label = '감시중'; bg = 'rgba(52,211,153,0.18)'; fg = '#34d399'; }
        badge.textContent = label;
        badge.style.background = bg;
        badge.style.color = fg;
        if (toggleBtn) toggleBtn.textContent = st.running ? '중지' : '시작';
        if (intervalEl && st.intervalSeconds) intervalEl.textContent = st.intervalSeconds;

        // 서버의 현재 주기를 드롭다운에 반영.
        // 프로그램을 껐다 켜면 서버는 기본값(10초)으로 시작하므로,
        // 저장해둔 사용자 선택이 있으면 최초 1회 서버에 다시 적용한다.
        if (intervalSel && st.intervalSeconds) {
            let saved = null;
            try { saved = localStorage.getItem(INTERVAL_KEY); } catch (e) {}
            if (!appliedSavedInterval && saved && parseInt(saved) !== st.intervalSeconds) {
                appliedSavedInterval = true;
                post('interval', { seconds: parseInt(saved) })
                    .then(s2 => paint(s2))
                    .catch(() => {});
                return;
            }
            appliedSavedInterval = true;
            if (intervalSel.value !== String(st.intervalSeconds)) {
                intervalSel.value = String(st.intervalSeconds);
            }
        }

        if (targetsBox) {
            targetsBox.innerHTML = (st.targets || []).map(t =>
                `<span style="font-size:0.75rem;padding:0.2rem 0.5rem;border-radius:6px;background:rgba(245,166,35,0.15);color:#f5a623">${escapeHtmlMin(t)}</span>`
            ).join('');
        }

        statsBox.innerHTML =
            stat('마지막 동기화', st.lastPollAt || '-') +
            stat('발견', `${st.totalHits || 0}건`) +
            stat('최근 발견', st.lastHitAt || '-') +
            stat('사이트 상태',
                !st.healthy ? `접속 실패 ${st.failCount}회`
                    : st.stale ? `데이터 정체 ${st.dataAgeMinutes}분`
                    : '정상',
                !st.healthy ? '#f87171' : st.stale ? '#f5a623' : '#34d399') +
            stat('사이트 최신 글', st.newestMessageAt || '-', st.stale ? '#f5a623' : undefined);

        if (!st.healthy && st.lastError) {
            errBox.style.display = '';
            errBox.style.background = 'rgba(248,113,113,0.12)';
            errBox.style.color = '#f87171';
            errBox.textContent = `baramlog.com 접속 실패 ${st.failCount}회 연속 — ${st.lastError}`;
        } else if (st.stale) {
            // 사이트는 응답하지만 새 글이 안 올라오는 상태 (사이트 쪽 문제)
            errBox.style.display = '';
            errBox.style.background = 'rgba(245,166,35,0.12)';
            errBox.style.color = '#f5a623';
            errBox.textContent = `사이트는 응답하지만 ${st.dataAgeMinutes}분째 새 글이 없습니다 (최신 글 ${st.newestMessageAt}). 사이트 쪽 문제로 보이며, 감시는 계속 돌고 있습니다.`;
        } else {
            errBox.style.display = 'none';
        }

        const hits = st.recentHits || [];
        listBox.innerHTML = hits.length === 0
            ? '<p style="font-size:0.8rem;color:var(--text-muted)">아직 발견된 모집 글이 없습니다. 새 글이 올라오면 여기와 텔레그램에 표시됩니다.</p>'
            : hits.map(h => `<div style="display:flex;gap:0.6rem;align-items:baseline;padding:0.45rem 0.2rem;border-bottom:1px solid var(--border-color);flex-wrap:wrap">
                <span style="font-size:0.68rem;padding:0.1rem 0.4rem;border-radius:5px;background:rgba(245,166,35,0.18);color:#f5a623;white-space:nowrap">${escapeHtmlMin(h.label)}</span>
                <span style="font-size:0.72rem;color:var(--text-muted);white-space:nowrap">${escapeHtmlMin((h.createdAt || '').slice(5))}</span>
                <span style="font-size:0.83rem"><b>${escapeHtmlMin(h.name)}</b> ${escapeHtmlMin(h.message)}</span>
            </div>`).join('');
    }

    async function refreshStatus() {
        try {
            const r = await fetch('/api/baramlog/status');
            if (r.ok) paint(await r.json());
        } catch (e) { /* 서버 미응답 시 무시 */ }
    }

    async function post(action, extra) {
        const r = await fetch('/api/baramlog/status', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(Object.assign({ action }, extra || {}))
        });
        if (!r.ok) throw new Error(await r.text());
        return await r.json();
    }

    if (refreshBtn) {
        refreshBtn.addEventListener('click', async () => {
            const orig = refreshBtn.textContent;
            refreshBtn.disabled = true;
            refreshBtn.textContent = '동기화 중…';
            try {
                const st = await post('sync');   // 서버가 조회를 끝낸 뒤 최신 상태를 준다
                paint(st);
                addLogMessage(`시련 알림: 동기화 완료 (${st.lastPollAt || '-'})`);
            } catch (e) {
                addLogMessage('시련 알림: 동기화 실패 - ' + e.message);
            }
            refreshBtn.textContent = orig;
            refreshBtn.disabled = false;
        });
    }

    if (toggleBtn) {
        toggleBtn.addEventListener('click', async () => {
            toggleBtn.disabled = true;
            try {
                const st = await post(running ? 'stop' : 'start');
                paint(st);
                addLogMessage(`시련 알림: ${st.running ? '감시 시작' : '감시 중지'}`);
            } catch (e) {
                addLogMessage('시련 알림: 상태 변경 실패 - ' + e.message);
            }
            toggleBtn.disabled = false;
        });
    }

    if (testBtn) {
        testBtn.addEventListener('click', async () => {
            const orig = testBtn.textContent;
            testBtn.disabled = true;
            testBtn.textContent = '전송 중…';
            try {
                // 실제 알림과 같은 경로로 전송된다 — 도착하면 실제 알림도 정상
                const st = await post('test');
                paint(st);
                testBtn.textContent = '✓ 전송됨';
                addLogMessage('시련 알림: 텔레그램 테스트 메시지 전송 완료');
            } catch (e) {
                testBtn.textContent = '✗ 실패';
                addLogMessage('시련 알림: 텔레그램 테스트 실패 - ' + e.message);
                alert('텔레그램 테스트 실패\n\n' + e.message);
            }
            setTimeout(() => { testBtn.textContent = orig; testBtn.disabled = false; }, 2000);
        });
    }

    if (intervalSel) {
        intervalSel.addEventListener('change', async () => {
            const sec = parseInt(intervalSel.value) || 10;
            try {
                const st = await post('interval', { seconds: sec });
                try { localStorage.setItem(INTERVAL_KEY, String(sec)); } catch (e) {}
                paint(st);
                addLogMessage(`시련 알림: 확인 주기 ${sec < 60 ? sec + '초' : (sec / 60) + '분'}로 변경`);
            } catch (e) {
                addLogMessage('시련 알림: 주기 변경 실패 - ' + e.message);
            }
        });
    }

    refreshStatus();
    setInterval(refreshStatus, 5000);
}
// 입력 방식(포그라운드/백그라운드) 선택 UI.
// 백그라운드는 창을 앞으로 가져오지 않지만, 최소화된 창은 캡처가 불가능하므로
// 최소화 옵션과 함께 쓸 수 없다(서버에서도 강제 해제됨).
// 입력 방식(포그라운드/백그라운드)은 창 목록의 창별 드롭다운으로만 정한다.
// 단, "입장 후 창 최소화"를 켜면 최소화된 창은 백그라운드 캡처(PrintWindow)가
// 불가능하므로 전 창을 포그라운드로 잠근다.
function setupInputModeSelect() {
    const desc = document.getElementById('multi-input-mode-desc');
    const minimize = document.getElementById('multi-entry-minimize');

    function applyToRows() {
        const locked = !!(minimize && minimize.checked);
        document.querySelectorAll('#multi-entry-list select.multi-input-select').forEach(s => {
            if (locked) s.value = 'fg';
            s.disabled = locked;
            s.title = locked
                ? '최소화 옵션이 켜져 있어 포그라운드로 고정됩니다 (최소화된 창은 백그라운드 캡처 불가).'
                : '포그라운드: 창을 앞으로 가져와 입력 / 백그라운드: 창을 띄우지 않고 입력·캡처';
        });
        if (desc) {
            desc.innerHTML = locked
                ? '<b>포그라운드 고정</b>: "입장 후 창 최소화"가 켜져 있습니다. 최소화된 창은 백그라운드 캡처가 불가능하므로 백그라운드를 쓸 수 없습니다. 백그라운드로 돌리려면 최소화를 꺼주세요.'
                : '입력 방식은 <b>창마다</b> 고릅니다. 백그라운드는 창을 앞으로 가져오지 않아 봇이 도는 동안 다른 작업을 할 수 있습니다(게임이 관리자 권한이면 도우미도 관리자로 실행). 창 감지 때는 화면 확인을 위해 창을 활성화합니다.';
        }
    }

    if (minimize) minimize.addEventListener('change', applyToRows);
    window.refreshMultiInputMode = applyToRows; // 창 감지 후 재적용
    applyToRows();
}
// 다중 창 입장 UI (창감지 → 체크박스 목록, 최대 4개 선택)
function setupMultiEntry() {
    const detectBtn = document.getElementById('multi-entry-detect');
    const list = document.getElementById('multi-entry-list');
    setupInputModeSelect();
    if (!detectBtn || !list) return;

    // 맵 디버그: 창마다 상단 맵 이름 OCR (느려서 버튼으로만 실행)
    const mapBtn = document.getElementById('multi-entry-mapdebug');
    if (mapBtn) {
        mapBtn.addEventListener('click', async () => {
            const rows = document.querySelectorAll('#multi-entry-list .multi-map-info');
            if (rows.length === 0) {
                addLogMessage('맵 디버그: 먼저 창 감지를 해주세요.');
                return;
            }
            mapBtn.disabled = true;
            const orig = mapBtn.textContent;
            mapBtn.textContent = '읽는 중…';
            try {
                const res = await fetch('/api/multi/mapinfo');
                const infos = await res.json();
                const byHwnd = {};
                (infos || []).forEach(m => { byHwnd[String(m.hwnd)] = m; });
                rows.forEach(div => {
                    const m = byHwnd[div.dataset.hwnd];
                    if (!m) { div.style.display = 'none'; return; }
                    const img = m.mapCrop
                        ? `<img src="${m.mapCrop}" alt="맵" style="height:22px;border:1px solid var(--border-color);border-radius:3px;background:#000">`
                        : '';
                    const mapTxt = m.mapText
                        ? `${escapeHtmlMin(m.mapText)}${m.method ? ` <span style="opacity:.6">(${escapeHtmlMin(m.method)})</span>` : ''}`
                        : '(모름 — 글자 학습 필요)';
                    const nickTxt = m.nickText ? ` · 닉: ${escapeHtmlMin(m.nickText)}` : '';
                    div.innerHTML = `${img}<span style="font-size:0.72rem;color:var(--text-muted)">맵: ${mapTxt}${nickTxt}</span>`;
                    div.style.display = 'flex';
                });
                addLogMessage(`맵 디버그: ${(infos || []).length}개 창 인식 완료`);
            } catch (e) {
                addLogMessage('맵 디버그 실패: ' + e.message);
            }
            mapBtn.textContent = orig;
            mapBtn.disabled = false;
        });
    }

    detectBtn.addEventListener('click', async () => {
        detectBtn.disabled = true;
        const orig = detectBtn.textContent;
        detectBtn.textContent = '감지 중…';
        try {
            const res = await fetch('/api/multi/detect');
            const wins = await res.json();
            if (!wins || wins.length === 0) {
                list.innerHTML = '<span style="font-size:0.78rem;color:var(--text-muted)">게임 창을 찾을 수 없습니다</span>';
            } else {
                // 닉네임은 글리프 매칭으로 정확히 읽는다(사전에 없는 글자만 빈 값).
                // 크롭 이미지도 같이 보여줘서 눈으로도 확인할 수 있게 둔다.
                // 창마다 대야/칸첸 드롭다운 — 혼합 가능 (예: 2창 대야 + 1창 칸첸)
                const defMode = 'daeya';
                const defInput = 'fg'; // 새 행 기본값은 포그라운드 (창마다 개별 변경)
                list.innerHTML = wins.map((w, i) => {
                    const cropImg = w.crop
                        ? `<img src="${w.crop}" alt="닉네임" style="height:34px;border:1px solid var(--border-color);border-radius:4px;image-rendering:pixelated;background:#000">`
                        : '<span style="font-size:0.72rem;color:var(--text-muted)">(캡처 실패)</span>';
                    // 맵 정보는 "맵 디버그" 버튼을 눌렀을 때만 이 자리에 채워진다
                    const mapInfo = `<div class="multi-map-info" data-hwnd="${w.hwnd}" style="display:none;align-items:center;gap:0.5rem;padding:0 0.2rem 0.35rem 2rem"></div>`;
                    return `<label style="display:flex;align-items:center;gap:0.6rem;font-size:0.85rem;padding:0.35rem 0.2rem;cursor:pointer">
                        <input type="checkbox" value="${w.hwnd}" ${i < 4 ? 'checked' : ''}>
                        <span style="color:var(--text-muted);white-space:nowrap">창 ${i + 1}</span>
                        ${cropImg}
                        <span style="white-space:nowrap;font-weight:600">${w.nick ? escapeHtmlMin(w.nick) : '<span style="font-weight:400;color:var(--text-muted);font-size:0.75rem">(글자 학습 필요)</span>'}</span>
                        <select class="multi-mode-select" style="font-size:0.78rem;padding:0.15rem 0.3rem;border-radius:4px;border:1px solid var(--border-color);background:var(--bg-secondary,rgba(255,255,255,0.05));color:inherit">
                            <option value="daeya" ${defMode === 'daeya' ? 'selected' : ''}>대야</option>
                            <option value="kanchen" ${defMode === 'kanchen' ? 'selected' : ''}>칸첸</option>
                        </select>
                        <select class="multi-input-select" title="포그라운드: 창을 앞으로 가져와 입력 / 백그라운드: 창을 띄우지 않고 입력·캡처" style="font-size:0.78rem;padding:0.15rem 0.3rem;border-radius:4px;border:1px solid var(--border-color);background:var(--bg-secondary,rgba(255,255,255,0.05));color:inherit;display:${i < 4 ? '' : 'none'}">
                            <option value="fg" ${defInput === 'fg' ? 'selected' : ''}>포그라운드</option>
                            <option value="bg" ${defInput === 'bg' ? 'selected' : ''}>백그라운드</option>
                        </select>
                        <span style="color:var(--text-muted);font-size:0.72rem;margin-left:auto">hwnd ${w.hwnd}</span>
                    </label>${mapInfo}`;
                }).join('');
                // 최대 4개 제한 + 선택된 창에만 입력 방식 드롭다운 표시
                const syncInputVisibility = (cb) => {
                    const inp = cb.closest('label')?.querySelector('select.multi-input-select');
                    if (inp) inp.style.display = cb.checked ? '' : 'none';
                };
                list.querySelectorAll('input[type="checkbox"]').forEach(cb => {
                    cb.addEventListener('change', () => {
                        const checked = list.querySelectorAll('input[type="checkbox"]:checked');
                        if (checked.length > 4) {
                            cb.checked = false;
                            addLogMessage('다중 창 입장은 최대 4개까지입니다.');
                        }
                        syncInputVisibility(cb);
                    });
                    syncInputVisibility(cb);
                });
                // 모드 드롭다운 변경 시 중앙좌표 입력란 표시 갱신
                // (select는 인터랙티브 요소라 label의 체크박스 토글을 트리거하지 않음)
                list.querySelectorAll('select.multi-mode-select').forEach(sel => {
                    sel.addEventListener('change', updateMultiCenterRow);
                });
                updateMultiCenterRow();
                if (window.refreshMultiInputMode) window.refreshMultiInputMode();
            }
            addLogMessage(`다중 창 입장: 창 ${(wins || []).length}개 감지됨`);
        } catch (e) {
            addLogMessage('다중 창 입장: 창 감지 실패 - ' + e.message);
        }
        detectBtn.textContent = orig;
        detectBtn.disabled = false;
    });
}

function escapeHtmlMin(s) {
    return String(s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
}

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
    // 모드 라디오는 제거됨 — 다중창 목록의 창별 대야/칸첸 드롭다운이 모드를 결정한다.
    // currentMode는 서버 전송용 기본값(대야 입장)으로 고정.
    timeOptions[2].checked = true;
    updateModeCards();

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
        case ModeTrialSolo: return '시련 (솔로)';
        case ModeTrialGroup: return '시련 (그룹)';
        default: return '알 수 없음';
    }
}

function getApiModeName(mode) {
    switch (mode) {
        case ModeDaeyaEnter: return 'daeya-entrance';
        case ModeDaeyaParty: return 'daeya-party';
        case ModeKanchenEnter: return 'kanchen-entrance';
        case ModeKanchenParty: return 'kanchen-party';
        case ModeTrialSolo: return 'trial-solo';
        case ModeTrialGroup: return 'trial-group';
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
        case 'switcherLog':
            if (window.onSwitcherLog) window.onSwitcherLog(payload.message);
            break;
        case 'switcherSlots':
            if (window.onSwitcherSlots) window.onSwitcherSlots(payload);
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
    // 창별 대야/칸첸 드롭다운이 모드를 결정 — 창 선택 필수 (모드 라디오는 제거됨)
    const rows = Array.from(document.querySelectorAll('#multi-entry-list input[type="checkbox"]:checked'))
        .slice(0, 4)
        .map(cb => {
            const row = cb.closest('label');
            const sel = row?.querySelector('select.multi-mode-select');
            const inp = row?.querySelector('select.multi-input-select');
            return {
                hwnd: cb.value,
                mode: (sel && sel.value) || 'daeya',
                bg: (inp && inp.value === 'bg')
            };
        });

    if (rows.length === 0) {
        addLogMessage("오류: '창 감지' 후 입장할 창을 선택해주세요.");
        startBtn.classList.remove('active');
        isRunning = false;
        return;
    }

    // 서버 mode 파라미터: 전부 칸첸이면 kanchen-entrance, 아니면 daeya-entrance
    // (혼합/단일 로직은 multi_modes가 창별로 결정하므로 알림 표기용에 가깝다)
    const allKanchen = rows.every(r => r.mode === 'kanchen');
    const apiMode = allKanchen ? 'kanchen-entrance' : 'daeya-entrance';
    currentMode = allKanchen ? ModeKanchenEnter : ModeDaeyaEnter;

    const hours = getHoursFromOption(currentTimeOption);

    let body = `mode=${apiMode}&auto_stop=${hours}`;
    // 일시정지 후 재개: 남은 시간(초)을 함께 보내 서버 자동종료 타이머를 남은 시간으로 맞춘다.
    // (전체 시간으로 재무장하면 UI 카운트다운이 먼저 끝나 /api/stop 을 호출 →
    //  서버 완료 타이머가 취소되어 텔레그램 알림이 안 울리는 버그 방지)
    if (wasTimerPaused && countdownTime > 0) {
        body += `&auto_stop_seconds=${countdownTime}`;
    }

    body += `&multi_hwnds=${rows.map(r => r.hwnd).join(',')}`;
    body += `&multi_modes=${rows.map(r => r.mode).join(',')}`;
    const minimize = document.getElementById('multi-entry-minimize');
    if (minimize && minimize.checked) body += `&multi_minimize=1`;
    // 창별 입력 방식 (fg=포그라운드, bg=백그라운드)
    body += `&multi_bgs=${rows.map(r => r.bg ? 'bg' : 'fg').join(',')}`;
    const nBG = rows.filter(r => r.bg).length;
    if (nBG > 0) {
        addLogMessage(`입력 방식: 백그라운드 ${nBG}개 / 포그라운드 ${rows.length - nBG}개`);
    }
    // 칸첸 창이 하나라도 있으면 복귀 좌표 전송 (칸첸 창에만 적용됨, 기본 34,37)
    if (rows.some(r => r.mode === 'kanchen')) {
        const cx = (document.getElementById('multi-center-x') || {}).value || '34';
        const cy = (document.getElementById('multi-center-y') || {}).value || '37';
        body += `&center_x=${cx}&center_y=${cy}`;
    }
    const nDaeya = rows.filter(r => r.mode === 'daeya').length;
    const nKanchen = rows.length - nDaeya;
    const mixDesc = [nDaeya ? `대야 ${nDaeya}` : '', nKanchen ? `칸첸 ${nKanchen}` : ''].filter(Boolean).join(' + ');
    addLogMessage(rows.length >= 2
        ? `다중 창 입장 유지: ${rows.length}개 창 (${mixDesc})`
        : `선택한 창 1개(${rows[0].mode === 'kanchen' ? '칸첸' : '대야'})로 기존 로직 실행`);

    fetch('/api/start', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: body
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

