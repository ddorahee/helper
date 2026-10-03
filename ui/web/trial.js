// 시련 탭 — 던전(대야/칸첸) · 솔로/그룹 · 창마다 역할과 입력 방식 · 칸첸 캐릭터별 스킬 키
// 서버: /api/trial/config(설정) · /api/trial/detect(창 목록) · /api/trial/status(진행) · /api/start · /api/stop
// 서버 로그는 dispatchAppEvent({type:'trialLog'}) 로 들어와 아래 로그 칸에 쌓인다.
(function() {
    'use strict';

    const MODE_KEY = 'trialMode';       // 솔로/그룹 — 이 PC 에만 기억
    const PREFS_KEY = 'trialWinPrefs';  // 캐릭터 이름 → { solo, group, input }
    const LOG_MAX = 200;
    const POLL_MS = 2000;
    const DETECT_LABEL = '창 감지';

    const ROLE_LABEL = { solo: '솔로', leader: '그룹장', member: '그룹원' };
    const BADGE = {
        idle: { text: '대기', cls: '' },
        running: { text: '실행 중', cls: ' running' },
        done: { text: '완료', cls: ' trial-badge-done' },
        stopped: { text: '중지됨', cls: ' trial-badge-stopped' },
    };

    // 서버 설정 — 칸첸 중앙 자리는 메인화면 칸첸 '사냥 자리'라 여기선 읽기만 한다
    const config = {
        dungeon: 'daeya',
        maxRuns: 10,
        daeyaTargetX: 29,
        daeyaTargetY: 32,
        kanchenTargetX: 34,
        kanchenTargetY: 37,
        skillKeys: [],
        skillKeysByChar: {},
    };
    let configLoaded = false;          // 불러오기 전엔 저장하지 않는다(서버 값을 기본값으로 덮지 않게)
    let saveChain = Promise.resolve();

    let trialMode = 'solo';
    let wins = [];                     // 마지막 감지 결과 [{hwnd, nick, nickCrop, mapText, place}]
    let winState = {};                 // hwnd → { solo: ''|'solo', group: ''|'leader'|'member', input: 'fg'|'bg' }
    let detectedOnce = false;
    let detectFailed = false;
    let detecting = false;

    let running = false;
    let busy = false;                  // 시작/중지 요청 중
    let badgeState = 'idle';
    let lineOverride = '';             // 끝났을 때 한 줄 요약 — 설정을 바꾸면 준비 안내로 돌아간다
    let lastStatus = null;
    let pollTimer = null;
    let pollGen = 0;
    let statusErrLogged = false;

    let els = {};

    function init() {
        if (!document.getElementById('trial-section')) return;
        const byId = id => document.getElementById(id);
        els = {
            badge: byId('trial-badge'),
            current: byId('trial-current'),
            startBtn: byId('trial-start-btn'),
            stopBtn: byId('trial-stop-btn'),
            dungeonSeg: byId('trial-dungeon-seg'),
            modeSeg: byId('trial-mode-seg'),
            maxRuns: byId('trial-max-runs'),
            detectBtn: byId('trial-detect-btn'),
            list: byId('trial-window-list'),
            daeyaCard: byId('trial-daeya-card'),
            daeyaX: byId('trial-daeya-x'),
            daeyaY: byId('trial-daeya-y'),
            kanchenCard: byId('trial-kanchen-card'),
            kanchenCenter: byId('trial-kanchen-center'),
            skillKeys: byId('trial-skill-keys'),
            skillChars: byId('trial-skill-chars'),
            log: byId('trial-log'),
        };

        try {
            const m = localStorage.getItem(MODE_KEY);
            if (m === 'solo' || m === 'group') trialMode = m;
        } catch (e) { /* 기본은 솔로 */ }

        bindEvents();
        renderConfigInputs();
        renderWindows();
        renderSkillRows();
        renderTopbar();

        // 켤 때 한 번: 설정 · 진행 상태(이미 돌고 있으면 이어서 보여준다) · 창 목록(창을 띄우지 않고)
        loadConfig();
        checkStatus();
        detect(true);
    }

    function bindEvents() {
        segButtons(els.dungeonSeg).forEach(btn => btn.addEventListener('click', () => setDungeon(btn.dataset.value)));
        segButtons(els.modeSeg).forEach(btn => btn.addEventListener('click', () => setMode(btn.dataset.value)));

        if (els.maxRuns) {
            els.maxRuns.addEventListener('change', () => {
                config.maxRuns = clampInt(els.maxRuns.value, 1, 99, config.maxRuns);
                els.maxRuns.value = config.maxRuns;
                settingsChanged();
                saveConfig();
            });
        }
        // 대야 서 있을 자리
        [[els.daeyaX, 'daeyaTargetX'], [els.daeyaY, 'daeyaTargetY']].forEach(([input, field]) => {
            if (!input) return;
            input.addEventListener('change', () => {
                config[field] = clampInt(input.value, 0, 999, config[field]);
                input.value = config[field];
                saveConfig();
            });
        });
        // 칸첸 기본 스킬 키 — 쓰는 동안 값만 들고 있다가 칸을 벗어나면 저장
        if (els.skillKeys) {
            els.skillKeys.addEventListener('input', () => { config.skillKeys = parseKeys(els.skillKeys.value); });
            els.skillKeys.addEventListener('change', () => {
                config.skillKeys = parseKeys(els.skillKeys.value);
                saveConfig();
            });
        }
        // 창 목록은 감지할 때마다 다시 그리니 목록 칸에 한 번만 건다
        if (els.list) els.list.addEventListener('change', onWindowListChange);
        if (els.detectBtn) els.detectBtn.addEventListener('click', () => detect(false));
        if (els.startBtn) els.startBtn.addEventListener('click', startTrial);
        if (els.stopBtn) els.stopBtn.addEventListener('click', stopTrial);

        // 탭을 열 때 — 칸첸 중앙 자리는 메인화면에서 바뀌었을 수 있어 다시 읽고, 로그는 맨 아래로
        const nav = document.querySelector('.nav-button[data-section="trial"]');
        if (nav) {
            nav.addEventListener('click', () => {
                refreshKanchenCenter();
                if (els.log) els.log.scrollTop = els.log.scrollHeight;
            });
        }
    }

    // ===== 던전 · 모드 · 횟수 =====

    function setMode(mode) {
        if ((mode !== 'solo' && mode !== 'group') || mode === trialMode) return;
        trialMode = mode;
        try { localStorage.setItem(MODE_KEY, mode); } catch (e) { /* 이번 실행에만 */ }
        renderSegments();
        renderWindows();    // 역할 칸의 선택지가 바뀐다
        renderSkillRows();
        settingsChanged();
    }

    function setDungeon(dungeon) {
        if ((dungeon !== 'daeya' && dungeon !== 'kanchen') || dungeon === config.dungeon) return;
        config.dungeon = dungeon;
        renderSegments();
        renderDungeonCards();
        settingsChanged();
        saveConfig();
    }

    function settingsChanged() {
        lineOverride = '';
        renderTopbar();
    }

    // 설정 값을 화면에 채운다 — 쓰고 있는 칸은 건드리지 않는다
    function renderConfigInputs() {
        renderSegments();
        renderDungeonCards();
        setInputValue(els.maxRuns, config.maxRuns);
        setInputValue(els.daeyaX, config.daeyaTargetX);
        setInputValue(els.daeyaY, config.daeyaTargetY);
        setInputValue(els.skillKeys, config.skillKeys.join(', '));
        renderKanchenCenter();
    }

    function renderSegments() {
        markSeg(els.dungeonSeg, config.dungeon);
        markSeg(els.modeSeg, trialMode);
    }

    function markSeg(seg, value) {
        segButtons(seg).forEach(btn => {
            const on = btn.dataset.value === value;
            btn.classList.toggle('active', on);
            btn.setAttribute('aria-pressed', on ? 'true' : 'false');
        });
    }

    function segButtons(seg) {
        return seg ? Array.from(seg.querySelectorAll('.trial-seg-btn')) : [];
    }

    // 고른 던전의 설정 카드만 보인다
    function renderDungeonCards() {
        if (els.daeyaCard) els.daeyaCard.style.display = config.dungeon === 'daeya' ? '' : 'none';
        if (els.kanchenCard) els.kanchenCard.style.display = config.dungeon === 'kanchen' ? '' : 'none';
    }

    function renderKanchenCenter() {
        if (els.kanchenCenter) els.kanchenCenter.textContent = `X ${config.kanchenTargetX} · Y ${config.kanchenTargetY}`;
    }

    function setInputValue(input, value) {
        if (input && input !== document.activeElement) input.value = String(value);
    }

    // ===== 서버 설정 (GET/POST /api/trial/config) =====

    async function fetchConfig() {
        const res = await fetch('/api/trial/config');
        if (!res.ok) throw new Error((await safeText(res)) || ('HTTP ' + res.status));
        return res.json();
    }

    async function loadConfig() {
        try {
            applyConfig(await fetchConfig());
            configLoaded = true;
            renderConfigInputs();
            renderSkillRows();
            renderTopbar();
        } catch (e) {
            addLog('시련 설정을 불러오지 못했습니다 - ' + e.message, 'err');
        }
    }

    // 탭을 열 때 — 메인화면 칸첸 '사냥 자리'가 바뀌었을 수 있어 중앙 자리만 다시 읽는다
    async function refreshKanchenCenter() {
        if (!configLoaded) { loadConfig(); return; }
        try {
            const c = await fetchConfig();
            config.kanchenTargetX = clampInt(c && c.kanchenTargetX, 0, 999, config.kanchenTargetX);
            config.kanchenTargetY = clampInt(c && c.kanchenTargetY, 0, 999, config.kanchenTargetY);
            renderKanchenCenter();
        } catch (e) { /* 다음에 탭을 열 때 다시 읽는다 */ }
    }

    function applyConfig(c) {
        c = c && typeof c === 'object' ? c : {};
        config.dungeon = c.dungeon === 'kanchen' ? 'kanchen' : 'daeya';
        config.maxRuns = clampInt(c.maxRuns > 0 ? c.maxRuns : 10, 1, 99, 10);
        config.daeyaTargetX = clampInt(c.daeyaTargetX, 0, 999, 29);
        config.daeyaTargetY = clampInt(c.daeyaTargetY, 0, 999, 32);
        config.kanchenTargetX = clampInt(c.kanchenTargetX, 0, 999, 34);
        config.kanchenTargetY = clampInt(c.kanchenTargetY, 0, 999, 37);
        config.skillKeys = normKeys(c.skillKeys);
        config.skillKeysByChar = {};
        const byChar = c.skillKeysByChar;
        if (byChar && typeof byChar === 'object' && !Array.isArray(byChar)) {
            Object.keys(byChar).forEach(name => {
                const n = String(name).trim();
                const keys = normKeys(byChar[name]);
                if (n && keys.length) config.skillKeysByChar[n] = keys;
            });
        }
    }

    // 늘 설정 전체를 보낸다. 겹치지 않게 차례로 보내고, 보낼 때의 최신 값을 쓴다
    function saveConfig() {
        if (!configLoaded) {
            addLog('설정을 아직 불러오지 못해 저장하지 않았습니다.', 'err');
            loadConfig();
            return Promise.resolve(false);
        }
        saveChain = saveChain.then(postConfig, postConfig);
        return saveChain;
    }

    async function postConfig() {
        try {
            const res = await fetch('/api/trial/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    dungeon: config.dungeon,
                    maxRuns: config.maxRuns,
                    daeyaTargetX: config.daeyaTargetX,
                    daeyaTargetY: config.daeyaTargetY,
                    kanchenTargetX: config.kanchenTargetX,
                    kanchenTargetY: config.kanchenTargetY,
                    skillKeys: config.skillKeys,
                    skillKeysByChar: config.skillKeysByChar,
                }),
            });
            if (!res.ok) throw new Error((await safeText(res)) || ('HTTP ' + res.status));
            // 응답(저장된 설정)에서는 읽기 전용인 중앙 자리만 받는다 — 쓰고 있는 값을 덮지 않게
            const saved = await res.json().catch(() => null);
            if (saved && typeof saved === 'object') {
                config.kanchenTargetX = clampInt(saved.kanchenTargetX, 0, 999, config.kanchenTargetX);
                config.kanchenTargetY = clampInt(saved.kanchenTargetY, 0, 999, config.kanchenTargetY);
                renderKanchenCenter();
            }
            return true;
        } catch (e) {
            addLog('설정 저장 실패 - ' + e.message, 'err');
            return false;
        }
    }

    // ===== 창 목록 (GET /api/trial/detect) =====
    // 켤 때 한 번은 ?poll=1(창을 띄우지 않음), 그다음은 '창 감지'를 누를 때만 읽는다.
    // 실행 중에도 poll 로만 읽는다 — 돌고 있는 창을 건드리지 않게.

    async function detect(quiet) {
        if (detecting) return;
        detecting = true;
        if (els.detectBtn) {
            els.detectBtn.disabled = true;
            els.detectBtn.textContent = '감지 중…';
        }
        try {
            const res = await fetch('/api/trial/detect' + (quiet || running ? '?poll=1' : ''));
            if (!res.ok) throw new Error((await safeText(res)) || ('HTTP ' + res.status));
            const data = await res.json();
            mergeWindows(Array.isArray(data) ? data : []);
            detectFailed = false;
            const names = wins.map((w, i) => `창 ${i + 1}${w.nick ? ' ' + w.nick : ''}`).join(', ');
            addLog(wins.length ? `창 ${wins.length}개 감지 — ${names}` : '게임 창을 찾을 수 없습니다');
        } catch (e) {
            detectFailed = true;
            addLog('창 감지 실패 - ' + e.message, 'err');
        } finally {
            detecting = false;
            detectedOnce = true;
            if (els.detectBtn) {
                els.detectBtn.disabled = false;
                els.detectBtn.textContent = DETECT_LABEL;
            }
            renderWindows();
            renderSkillRows();
            renderTopbar();
        }
    }

    function normalizeWin(raw) {
        raw = raw && typeof raw === 'object' ? raw : {};
        const crop = typeof raw.nickCrop === 'string' && raw.nickCrop.indexOf('data:image/') === 0 ? raw.nickCrop : '';
        return {
            hwnd: Number(raw.hwnd) || 0,
            nick: typeof raw.nick === 'string' ? raw.nick.trim() : '',
            nickCrop: crop,
            mapText: typeof raw.mapText === 'string' ? raw.mapText.trim() : '',
            place: isTrialPlace(raw.place) ? raw.place : '',
        };
    }

    // 새 감지 결과에 지금 고른 값을 잇는다 — 같은 hwnd 면 그대로, 아니면 같은 이름이던 창,
    // 그다음 저장된 설정(이름 기준). 처음 보는 창은 시련 안(시련장/칸첸/대야)에 있는 창부터 빈 역할을 채운다.
    function mergeWindows(list) {
        const prevByHwnd = {};
        const stateByNick = {};
        wins.forEach(w => {
            const key = String(w.hwnd);
            prevByHwnd[key] = w;
            if (w.nick && winState[key] && !stateByNick[w.nick]) stateByNick[w.nick] = winState[key];
        });
        const prefs = loadPrefs();

        const next = [];
        const seen = new Set();
        list.forEach(raw => {
            const w = normalizeWin(raw);
            if (!w.hwnd || seen.has(w.hwnd)) return;
            seen.add(w.hwnd);
            // 이름은 한 번 읽혔던 창이면 잠깐 가려져 못 읽어도 유지한다(같은 창은 같은 캐릭터)
            const p = prevByHwnd[String(w.hwnd)];
            if (!w.nick && p && p.nick) w.nick = p.nick;
            next.push(w);
        });

        // rank: 0 지금 화면(hwnd) · 1 같은 이름이던 창 · 2 저장된 설정 · 3 처음 보는 창
        const rows = next.map((w, i) => {
            const key = String(w.hwnd);
            if (winState[key]) return { w, i, rank: 0, st: copyState(winState[key]) };
            if (w.nick && stateByNick[w.nick]) return { w, i, rank: 1, st: copyState(stateByNick[w.nick]) };
            const pref = w.nick ? prefs[w.nick] : null;
            if (pref && typeof pref === 'object') return { w, i, rank: 2, st: copyState(pref) };
            return { w, i, rank: 3, st: copyState(null) };
        });

        // 역할이 겹치면 우선순위가 높은 쪽(같으면 앞 창)만 남긴다
        const taken = { solo: false, leader: false, member: false };
        rows.slice().sort((a, b) => a.rank - b.rank || a.i - b.i).forEach(r => {
            if (r.st.solo) {
                if (taken.solo) r.st.solo = '';
                else taken.solo = true;
            }
            if (r.st.group) {
                if (taken[r.st.group]) r.st.group = '';
                else taken[r.st.group] = true;
            }
        });

        // 처음 보는 창 — 솔로는 첫 창, 그룹은 앞의 두 창이 그룹장/그룹원 (이미 누가 맡은 역할은 건너뛴다)
        rows.filter(r => r.rank === 3)
            .sort((a, b) => (isTrialPlace(a.w.place) ? 0 : 1) - (isTrialPlace(b.w.place) ? 0 : 1) || a.i - b.i)
            .forEach(r => {
                if (!taken.solo) {
                    r.st.solo = 'solo';
                    taken.solo = true;
                }
                if (!taken.leader) {
                    r.st.group = 'leader';
                    taken.leader = true;
                } else if (!taken.member) {
                    r.st.group = 'member';
                    taken.member = true;
                }
            });

        const nextState = {};
        rows.forEach(r => { nextState[String(r.w.hwnd)] = r.st; });
        wins = next;
        winState = nextState;
    }

    function copyState(s) {
        s = s && typeof s === 'object' ? s : {};
        return {
            solo: s.solo === 'solo' ? 'solo' : '',
            group: s.group === 'leader' || s.group === 'member' ? s.group : '',
            input: s.input === 'bg' ? 'bg' : 'fg',
        };
    }

    function renderWindows() {
        const box = els.list;
        if (!box) return;
        if (!wins.length) {
            if (!detectedOnce) {
                box.innerHTML = '<div class="trial-loading">게임 창을 찾는 중…</div>';
            } else if (detectFailed) {
                box.innerHTML = '<div class="empty-placeholder">창을 읽지 못했습니다. 창 감지를 다시 눌러 주세요.</div>';
            } else {
                box.innerHTML = '<div class="empty-placeholder">게임 창을 찾을 수 없습니다.<br>게임을 켠 뒤 창 감지를 누르세요.</div>';
            }
            return;
        }
        const roleOptions = trialMode === 'solo'
            ? [['', '안 씀'], ['solo', '솔로']]
            : [['', '안 씀'], ['leader', '그룹장'], ['member', '그룹원']];

        box.innerHTML = wins.map((w, i) => {
            const key = String(w.hwnd);
            const st = winState[key] || copyState(null);
            const role = trialMode === 'solo' ? st.solo : st.group;
            let name;
            if (w.nick) name = `<b title="${esc(w.nick)}">${esc(w.nick)}</b>`;
            else if (w.nickCrop) name = `<img src="${esc(w.nickCrop)}" alt="캐릭터 이름">`;
            else name = '<span class="trial-muted">(이름 못 읽음)</span>';
            const opts = roleOptions
                .map(([value, label]) => `<option value="${value}"${value === role ? ' selected' : ''}>${label}</option>`)
                .join('');
            return `<div class="trial-win-row${role ? ' on' : ''}" data-hwnd="${key}" data-role="${role}">
                <div class="trial-win-main">
                    <span class="trial-win-idx" title="hwnd ${key}">창 ${i + 1}</span>
                    <span class="trial-win-name">${name}</span>
                    ${placeCell(w)}
                </div>
                <div class="trial-win-ctrls">
                    <select class="trial-role-select" title="역할">${opts}</select>
                </div>
            </div>`;
        }).join('');
    }

    // 지금 서 있는 맵 + 꼬리표 (시련장 / 칸첸 시련 / 대야 시련 / 시련 밖)
    function placeCell(w) {
        if (!w.mapText && !w.place) {
            return '<span class="trial-win-map"><span class="trial-muted">(위치 못 읽음)</span></span>';
        }
        let tag;
        if (w.place === 'lobby') tag = '<span class="trial-tag trial-tag-lobby">시련장</span>';
        else if (w.place === 'kanchen') tag = '<span class="mode-chip mode-chip-kanchen trial-tag">칸첸 시련</span>';
        else if (w.place === 'daeya') tag = '<span class="mode-chip mode-chip-daeya trial-tag">대야 시련</span>';
        else tag = '<span class="trial-tag trial-tag-out">시련 밖</span>';
        const text = w.mapText ? `<span class="trial-win-maptext">${esc(w.mapText)}</span>` : '';
        return `<span class="trial-win-map" title="${esc(w.mapText)}">${text}${tag}</span>`;
    }

    function onWindowListChange(e) {
        const sel = e.target;
        const row = sel && sel.closest ? sel.closest('.trial-win-row') : null;
        if (!row) return;
        const key = row.dataset.hwnd;
        const st = winState[key];
        if (!st) return;
        if (sel.classList.contains('trial-role-select')) {
            setRole(key, sel.value);
        } else {
            return;
        }
        syncRows();
        savePrefs();
        renderSkillRows();  // 캐릭터 칸의 역할 표시
        settingsChanged();
    }

    // 역할 하나는 창 하나만. 다른 창이 쥐고 있던 역할은 이 창으로 옮긴다 —
    // 그룹에서 이 창이 다른 역할이었으면 맞바꿔서 그룹장·그룹원이 둘 다 남게 한다
    function setRole(key, value) {
        const st = winState[key];
        if (!st) return;
        if (trialMode === 'solo') {
            const role = value === 'solo' ? 'solo' : '';
            if (role) eachState((k, s) => { if (k !== key) s.solo = ''; });
            st.solo = role;
            return;
        }
        const role = value === 'leader' || value === 'member' ? value : '';
        const prev = st.group;
        if (role) {
            eachState((k, s) => {
                if (k !== key && s.group === role) s.group = prev && prev !== role ? prev : '';
            });
        }
        st.group = role;
    }

    function eachState(fn) {
        wins.forEach(w => {
            const key = String(w.hwnd);
            if (winState[key]) fn(key, winState[key]);
        });
    }

    // 다시 그리지 않고 고른 값·강조만 맞춘다(열려 있는 드롭다운이 닫히지 않게)
    function syncRows() {
        if (!els.list) return;
        els.list.querySelectorAll('.trial-win-row').forEach(row => {
            const st = winState[row.dataset.hwnd];
            if (!st) return;
            const role = trialMode === 'solo' ? st.solo : st.group;
            const roleSel = row.querySelector('.trial-role-select');
            if (roleSel && roleSel.value !== role) roleSel.value = role;
            row.classList.toggle('on', !!role);
            row.dataset.role = role;
        });
    }

    // 지금 창들의 역할·입력 방식을 캐릭터 이름으로 기억한다(이름 못 읽은 창은 빼고).
    // 지금 창이 맡은 역할은 창이 없는 캐릭터의 기록에서 지운다 — 다음에 둘 다 떠도 겹치지 않게
    function loadPrefs() {
        try {
            const p = JSON.parse(localStorage.getItem(PREFS_KEY) || '{}');
            return p && typeof p === 'object' && !Array.isArray(p) ? p : {};
        } catch (e) {
            return {};
        }
    }

    function savePrefs() {
        const prefs = loadPrefs();
        const held = { solo: false, leader: false, member: false };
        const present = new Set();
        wins.forEach(w => {
            const st = winState[String(w.hwnd)];
            if (!st) return;
            if (st.solo) held.solo = true;
            if (st.group) held[st.group] = true;
            if (w.nick) {
                present.add(w.nick);
                prefs[w.nick] = copyState(st);
            }
        });
        Object.keys(prefs).forEach(nick => {
            if (present.has(nick)) return;
            const p = copyState(prefs[nick]);
            if (p.solo && held.solo) p.solo = '';
            if (p.group && held[p.group]) p.group = '';
            prefs[nick] = p;
        });
        try {
            localStorage.setItem(PREFS_KEY, JSON.stringify(prefs));
        } catch (e) { /* 저장 못 해도 이번 실행에는 지장 없다 */ }
    }

    // 지금 모드에서 역할을 맡은 창 { solo, leader, member }
    function assigned() {
        const a = { solo: null, leader: null, member: null };
        wins.forEach(w => {
            const st = winState[String(w.hwnd)];
            if (!st) return;
            if (st.solo && !a.solo) a.solo = w;
            if (st.group && !a[st.group]) a[st.group] = w;
        });
        return a;
    }

    function countRole(role) {
        const field = role === 'solo' ? 'solo' : 'group';
        return wins.filter(w => {
            const st = winState[String(w.hwnd)];
            return st && st[field] === role;
        }).length;
    }

    function roleOf(w) {
        const st = winState[String(w.hwnd)];
        if (!st) return '';
        return trialMode === 'solo' ? st.solo : st.group;
    }

    function winName(w) {
        return w.nick || `창 ${wins.indexOf(w) + 1}`;
    }

    // 시련은 전부 백그라운드로 돈다 — 입장·걷기·스킬 모두 창을 앞으로 안 가져온다
    // (사용자 2026-10-03: 포그라운드 선택지는 없앰)
    function isBg() {
        return true;
    }

    function winDesc(w) {
        return winName(w);
    }

    // ===== 칸첸 시련 스킬 키 — 기본 + 캐릭터별 (메인화면 칸첸 카드와 같은 틀) =====

    function parseKeys(text) {
        return String(text || '').split(',').map(k => k.trim().toLowerCase()).filter(Boolean);
    }

    function normKeys(v) {
        if (Array.isArray(v)) return v.map(k => String(k).trim().toLowerCase()).filter(Boolean);
        if (typeof v === 'string') return parseKeys(v);
        return [];
    }

    // 비운 캐릭터는 목록에서 빼서 기본 키를 쓰게 한다
    function setCharSkills(name, text) {
        const keys = parseKeys(text);
        if (keys.length) config.skillKeysByChar[name] = keys;
        else delete config.skillKeysByChar[name];
    }

    function renderSkillRows() {
        const box = els.skillChars;
        if (!box) return;
        // 쓰고 있는 칸이 있으면 다시 그린 뒤에도 이어서 쓸 수 있게 기억해 둔다
        const active = document.activeElement;
        const focusName = active && active.classList && active.classList.contains('trial-skill-input')
            ? active.closest('.kanchen-skill-row')?.dataset.name : null;

        // 창 순서대로(지금 감지된 캐릭터) + 저장돼 있지만 지금 창이 없는 캐릭터
        const seen = new Set();
        const rows = [];
        wins.forEach(w => {
            if (!w.nick || seen.has(w.nick)) return;
            seen.add(w.nick);
            rows.push({ name: w.nick, role: roleOf(w), missing: false });
        });
        Object.keys(config.skillKeysByChar).sort((a, b) => a.localeCompare(b, 'ko')).forEach(name => {
            if (!seen.has(name)) rows.push({ name, role: '', missing: true });
        });

        if (!rows.length) {
            box.innerHTML = '<div class="kanchen-skill-empty">창 감지로 읽은 캐릭터가 여기에 나옵니다.</div>';
            return;
        }
        box.innerHTML = rows.map(r => {
            const n = esc(r.name);
            const keys = esc((config.skillKeysByChar[r.name] || []).join(', '));
            const chip = r.role
                ? `<span class="mode-chip trial-role-chip trial-role-${r.role}" title="지금 맡은 역할">${ROLE_LABEL[r.role]}</span>`
                : '';
            const missing = r.missing ? '<span class="kanchen-skill-missing">창 없음</span>' : '';
            const del = r.missing
                ? '<button type="button" class="kanchen-skill-del" title="이 캐릭터의 키를 지웁니다">✕</button>'
                : '<span></span>';
            return `<div class="kanchen-skill-row${r.missing ? ' missing' : ''}" data-name="${n}">
                <span class="kanchen-skill-name" title="${n}"><span class="kanchen-skill-label">${n}</span>${chip}${missing}</span>
                <input type="text" class="trial-skill-input" placeholder="기본 키 사용" value="${keys}">
                ${del}
            </div>`;
        }).join('');

        box.querySelectorAll('.kanchen-skill-row').forEach(row => {
            const name = row.dataset.name;
            const input = row.querySelector('.trial-skill-input');
            if (!input) return;
            input.addEventListener('input', () => setCharSkills(name, input.value));
            input.addEventListener('change', () => {
                setCharSkills(name, input.value);
                saveConfig();  // 바꾸면 바로 저장
            });
            const delBtn = row.querySelector('.kanchen-skill-del');
            if (delBtn) {
                delBtn.addEventListener('click', () => {
                    delete config.skillKeysByChar[name];
                    saveConfig();
                    renderSkillRows();
                });
            }
            if (name === focusName) {
                input.focus();
                input.setSelectionRange(input.value.length, input.value.length);
                // 다시 그리느라 change 가 끊겼을 수 있다 — 칸을 벗어날 때 한 번 저장
                input.addEventListener('blur', () => saveConfig(), { once: true });
            }
        });
    }

    // ===== 상단 바 =====

    function renderTopbar() {
        const badge = BADGE[badgeState] || BADGE.idle;
        if (els.badge) {
            els.badge.textContent = badge.text;
            els.badge.className = 'rotation-badge' + badge.cls;
        }
        const text = running ? runningLine(lastStatus) : (lineOverride || readyHint());
        if (els.current) {
            els.current.textContent = text;
            els.current.title = text;
        }
        if (els.startBtn) els.startBtn.disabled = running || busy;
        if (els.stopBtn) els.stopBtn.disabled = !running || busy;
    }

    // 실행 중 — 예: 칸첸 시련 · 그룹 · 3/10회 · 전투
    function runningLine(st) {
        st = st || {};
        const parts = [dungeonLabel(st.dungeon) || dungeonLabel(config.dungeon), st.group ? '그룹' : '솔로'];
        const max = Number(st.maxRuns) || 0;
        if (max > 0) parts.push(`${Number(st.runCount) || 0}/${max}회`);
        if (st.phase) parts.push(String(st.phase));
        return parts.join(' · ');
    }

    // 쉬는 중 — 무엇이 빠졌는지, 다 됐으면 무엇으로 돌지
    function readyHint() {
        if (!detectedOnce) return '게임 창을 읽는 중…';
        if (!wins.length) return '게임 창이 없습니다 — 게임을 켜고 창 감지를 누르세요';
        const a = assigned();
        const label = dungeonLabel(config.dungeon);
        if (trialMode === 'solo') {
            if (!a.solo) return '창 선택에서 솔로로 돌릴 창을 고르세요';
            return `${label} · 솔로 ${winName(a.solo)} · ${config.maxRuns}회`;
        }
        if (!a.leader || !a.member) return '창 선택에서 그룹장과 그룹원을 고르세요';
        return `${label} · 그룹 ${winName(a.leader)} + ${winName(a.member)} · ${config.maxRuns}회`;
    }

    // ===== 시작 / 중지 =====

    async function startTrial() {
        if (running || busy) return;
        const a = assigned();
        let err = '';
        if (!wins.length) {
            err = '먼저 창 감지로 게임 창을 찾아 주세요.';
        } else if (trialMode === 'solo') {
            const n = countRole('solo');
            if (n === 0) err = '솔로로 돌릴 창을 하나 고르세요 (창 선택 → 역할).';
            else if (n > 1) err = '솔로는 창 하나만 고를 수 있습니다.';
        } else if (!a.leader && !a.member) {
            err = '그룹장과 그룹원을 고르세요 (창 선택 → 역할).';
        } else if (!a.leader) {
            err = '그룹장을 고르세요 (창 선택 → 역할).';
        } else if (!a.member) {
            err = '그룹원을 고르세요 (창 선택 → 역할).';
        } else if (a.leader.hwnd === a.member.hwnd) {
            err = '그룹장과 그룹원은 다른 창이어야 합니다.';
        } else if (countRole('leader') > 1 || countRole('member') > 1) {
            err = '그룹장과 그룹원은 한 창씩만 고를 수 있습니다.';
        }
        if (err) {
            addLog('시작 못 함 — ' + err, 'err');
            alert(err);
            return;
        }

        // 횟수는 칸에서 다시 읽는다(바꾸자마자 시작을 눌렀을 수 있다)
        if (els.maxRuns) {
            config.maxRuns = clampInt(els.maxRuns.value, 1, 99, config.maxRuns);
            els.maxRuns.value = config.maxRuns;
        }
        const dungeon = config.dungeon;
        const maxRuns = config.maxRuns;

        busy = true;
        renderTopbar();
        try {
            // 설정(던전·횟수)을 먼저 저장 — 실패해도 시작 요청에 같은 값이 들어간다
            if (configLoaded) await saveConfig();
            else addLog('설정을 불러오지 못해 저장 없이 시작합니다.', 'err');

            let body;
            let desc;
            if (trialMode === 'solo') {
                const w = a.solo;
                body = `mode=trial-solo&trial_max_runs=${maxRuns}&trial_dungeon=${dungeon}`
                    + `&hwnd=${w.hwnd}&bg=${isBg(w) ? 1 : 0}&nick=${encodeURIComponent(w.nick || '')}`;
                desc = `솔로 ${winDesc(w)}`;
            } else {
                const L = a.leader;
                const M = a.member;
                body = `mode=trial-group&trial_max_runs=${maxRuns}&trial_dungeon=${dungeon}`
                    + `&leader_hwnd=${L.hwnd}&member_hwnd=${M.hwnd}`
                    + `&leader_bg=${isBg(L) ? 1 : 0}&member_bg=${isBg(M) ? 1 : 0}`
                    + `&leader_nick=${encodeURIComponent(L.nick || '')}&member_nick=${encodeURIComponent(M.nick || '')}`;
                desc = `그룹장 ${winDesc(L)} + 그룹원 ${winDesc(M)}`;
            }

            const res = await fetch('/api/start', {
                method: 'POST',
                headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                body: body,
            });
            if (!res.ok) {
                const text = (await safeText(res)) || ('HTTP ' + res.status);
                note('시작 실패: ' + text, 'err');
                alert('시작 실패: ' + text);
                return;
            }
            savePrefs();
            running = true;
            badgeState = 'running';
            lineOverride = '';
            statusErrLogged = false;
            lastStatus = { running: true, runCount: 0, maxRuns: maxRuns, dungeon: dungeon, group: trialMode === 'group', phase: '시작 중' };
            note(`시작 — ${dungeonLabel(dungeon)} · ${desc} · ${maxRuns}회`);
            startPolling();
        } catch (e) {
            note('시작 실패: ' + e.message, 'err');
            alert('시작 실패: ' + e.message);
        } finally {
            busy = false;
            renderTopbar();
        }
    }

    async function stopTrial() {
        if (!running || busy) return;
        busy = true;
        renderTopbar();
        try {
            const res = await fetch('/api/stop', { method: 'POST' });
            if (!res.ok) throw new Error((await safeText(res)) || ('HTTP ' + res.status));
            stopPolling();
            running = false;
            badgeState = 'stopped';
            const max = lastStatus ? Number(lastStatus.maxRuns) || 0 : 0;
            lineOverride = max > 0 ? `중지했습니다 · ${Number(lastStatus.runCount) || 0}/${max}회` : '중지했습니다';
            note('시련 중지');
        } catch (e) {
            note('중지 실패: ' + e.message, 'err');
        } finally {
            busy = false;
            renderTopbar();
        }
    }

    // ===== 진행 상태 (GET /api/trial/status) =====

    async function fetchStatus() {
        const res = await fetch('/api/trial/status');
        if (!res.ok) throw new Error('HTTP ' + res.status);
        return res.json();
    }

    // 켤 때 한 번 — 이미 돌고 있으면 이어서 보여준다
    async function checkStatus() {
        try {
            const st = await fetchStatus();
            if (st && st.running && !running) {
                running = true;
                badgeState = 'running';
                lineOverride = '';
                lastStatus = st;
                addLog('시련이 돌고 있습니다 — 진행 상태를 이어서 봅니다');
                startPolling();
                renderTopbar();
            }
        } catch (e) { /* 서버가 아직 준비 전일 수 있다 — 조용히 넘긴다 */ }
    }

    // 실행 중에만 2초마다. 끝났다고 하면 멈추고 완료로 바꾼다
    function startPolling() {
        stopPolling();
        const gen = pollGen;
        const tick = async () => {
            if (gen !== pollGen) return;
            try {
                const st = await fetchStatus();
                if (gen !== pollGen || !running) return;
                if (!st || !st.running) {
                    finish(st);
                    return;
                }
                lastStatus = st;
                renderTopbar();
            } catch (e) {
                if (gen !== pollGen) return;
                if (!statusErrLogged) {
                    statusErrLogged = true;
                    addLog('진행 상태를 읽지 못했습니다 - ' + e.message, 'err');
                }
            }
            if (gen === pollGen && running) pollTimer = setTimeout(tick, POLL_MS);
        };
        pollTimer = setTimeout(tick, POLL_MS);
    }

    function stopPolling() {
        pollGen++;
        if (pollTimer) clearTimeout(pollTimer);
        pollTimer = null;
    }

    function finish(st) {
        stopPolling();
        running = false;
        badgeState = 'done';
        // 끝난 뒤 횟수를 0 으로 돌려주는 서버도 있을 수 있어, 비어 있으면 마지막으로 본 값을 쓴다
        const src = st && Number(st.maxRuns) > 0 ? st : (lastStatus || {});
        const max = Number(src.maxRuns) || 0;
        const count = max > 0 ? `${Number(src.runCount) || 0}/${max}회` : '';
        const label = dungeonLabel(src.dungeon) || dungeonLabel(config.dungeon);
        lineOverride = `${label}${count ? ' · ' + count : ''} · 끝`;
        note(`시련 끝${count ? ' (' + count + ')' : ''}`);
        renderTopbar();
    }

    // ===== 로그 =====

    // 이 탭의 로그 칸에만 남긴다
    function addLog(msg, kind) {
        appendLog(msg, kind || 'own');
    }

    // 시작·중지·끝·실패는 로그 탭(서버 로그)에도 남긴다
    function note(msg, kind) {
        addLog(msg, kind);
        if (typeof addLogMessage === 'function') addLogMessage('[시련] ' + msg);
    }

    function appendLog(msg, kind) {
        const box = els.log || document.getElementById('trial-log');
        if (!box) return;
        const empty = box.querySelector('.trial-log-empty');
        if (empty) empty.remove();
        // 위로 올려 읽는 중이면 끌어내리지 않는다(숨어 있을 땐 탭을 열 때 맨 아래로 간다)
        const atBottom = box.scrollHeight - box.scrollTop - box.clientHeight < 40;
        const line = document.createElement('div');
        line.className = 'trial-log-line' + (kind ? ' ' + kind : '');
        const time = new Date().toLocaleTimeString('ko-KR', { hour12: false });
        line.textContent = `[${time}] ${msg}`;
        box.appendChild(line);
        while (box.children.length > LOG_MAX) box.removeChild(box.firstChild);
        if (atBottom) box.scrollTop = box.scrollHeight;
    }

    // ===== 도우미 =====

    function dungeonLabel(d) {
        if (d === 'kanchen') return '칸첸 시련';
        if (d === 'daeya') return '대야 시련';
        return '';
    }

    function isTrialPlace(p) {
        return p === 'lobby' || p === 'kanchen' || p === 'daeya';
    }

    function clampInt(v, min, max, def) {
        const n = parseInt(v, 10);
        if (isNaN(n)) return def;
        return Math.min(max, Math.max(min, n));
    }

    function esc(s) {
        if (typeof escapeHtmlMin === 'function') return escapeHtmlMin(s);
        return String(s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
    }

    async function safeText(res) {
        try {
            return (await res.text()).trim();
        } catch (e) {
            return '';
        }
    }

    // ===== 서버 이벤트 — 앞 핸들러를 이어서 부른다(rotation.js 와 같은 방식) =====
    const prevDispatch = window.dispatchAppEvent;
    window.dispatchAppEvent = function(event) {
        try {
            if (typeof prevDispatch === 'function') prevDispatch(event);
        } finally {
            if (event && event.type === 'trialLog') {
                const msg = event.payload && event.payload.message;
                if (msg) appendLog(String(msg), /실패|오류|에러/.test(String(msg)) ? 'err' : '');
            }
        }
    };

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();
