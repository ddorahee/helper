// 시련 시스템 JS
(function() {
    let trialMode = 'solo';
    let detectedWindows = []; // [{hwnd, name}]
    let selectedSolo = null;
    let selectedLeader = null;
    let selectedMember = null;
    let trialRunning = false;
    let logPollInterval = null;

    function init() {
        const section = document.getElementById('trial-section');
        if (!section) return;

        document.querySelectorAll('input[name="trial-mode"]').forEach(radio => {
            radio.addEventListener('change', (e) => {
                trialMode = e.target.value;
                renderRoleSelect();
            });
        });

        document.getElementById('trial-detect-btn')?.addEventListener('click', detectWindows);
        document.getElementById('trial-start-btn')?.addEventListener('click', startTrial);
        document.getElementById('trial-stop-btn')?.addEventListener('click', stopTrial);
    }

    function addLog(msg) {
        const el = document.getElementById('trial-log');
        if (!el) return;
        const time = new Date().toLocaleTimeString('ko-KR', {hour12: false});
        const line = document.createElement('div');
        line.textContent = `[${time}] ${msg}`;
        // 첫 줄이 안내 문구면 제거
        if (el.children.length === 1 && el.children[0].style.opacity) {
            el.innerHTML = '';
        }
        el.appendChild(line);
        el.scrollTop = el.scrollHeight;
        // 최대 100줄
        while (el.children.length > 100) el.removeChild(el.firstChild);
    }

    async function detectWindows() {
        const btn = document.getElementById('trial-detect-btn');
        const list = document.getElementById('trial-window-list');
        if (!list) return;

        btn.disabled = true;
        btn.textContent = '감지 중...';
        addLog('바람창 감지 시작...');

        try {
            const res = await fetch('/api/trial/detect');
            const data = await res.json();

            detectedWindows = [];
            selectedSolo = null;
            selectedLeader = null;
            selectedMember = null;

            if (!data || data.length === 0) {
                list.innerHTML = '<span style="color:#ef4444">바람창을 찾을 수 없습니다</span>';
                addLog('바람창을 찾을 수 없습니다');
                return;
            }

            detectedWindows = data.map(w => ({
                hwnd: w.hwnd,
                name: w.detectedName || '(인식 실패)'
            }));

            addLog(`${detectedWindows.length}개 바람창 감지됨`);
            detectedWindows.forEach((w, i) => {
                addLog(`  ${i + 1}. ${w.name}`);
            });

            // 기본 선택
            if (detectedWindows.length >= 1) selectedSolo = detectedWindows[0].hwnd;
            if (detectedWindows.length >= 1) selectedLeader = detectedWindows[0].hwnd;
            if (detectedWindows.length >= 2) selectedMember = detectedWindows[1].hwnd;

            renderWindowList();
            renderRoleSelect();
        } catch(e) {
            list.innerHTML = '<span style="color:#ef4444">감지 실패: ' + e.message + '</span>';
            addLog('감지 실패: ' + e.message);
        } finally {
            btn.disabled = false;
            btn.textContent = '감지';
        }
    }

    function renderWindowList() {
        const list = document.getElementById('trial-window-list');
        if (!list) return;

        if (detectedWindows.length === 0) {
            list.innerHTML = '<span style="opacity:0.5">감지 버튼을 눌러 바람창을 찾아주세요</span>';
            return;
        }

        list.innerHTML = detectedWindows.map((w, i) => {
            return `<div style="padding:0.4rem 0.6rem;margin-bottom:0.3rem;background:rgba(255,255,255,0.05);border-radius:6px;display:flex;align-items:center;gap:0.5rem">
                <span style="font-size:0.75rem;opacity:0.4;min-width:1.2rem">${i + 1}</span>
                <span style="font-weight:500">${escapeHtml(w.name)}</span>
                <span style="font-size:0.7rem;opacity:0.3;margin-left:auto">hwnd:${w.hwnd}</span>
            </div>`;
        }).join('');
    }

    function renderRoleSelect() {
        const card = document.getElementById('trial-role-card');
        const container = document.getElementById('trial-role-select');
        if (!card || !container) return;

        if (detectedWindows.length === 0) {
            card.style.display = 'none';
            return;
        }

        card.style.display = '';

        if (trialMode === 'solo') {
            container.innerHTML = `
                <div style="margin-bottom:0.3rem;font-size:0.8rem;opacity:0.6">캐릭터 선택</div>
                ${makeSelect('trial-solo-select', selectedSolo)}
            `;
            document.getElementById('trial-solo-select')?.addEventListener('change', (e) => {
                selectedSolo = parseInt(e.target.value);
            });
        } else {
            container.innerHTML = `
                <div style="margin-bottom:0.5rem">
                    <div style="font-size:0.8rem;color:#3b82f6;font-weight:bold;margin-bottom:0.3rem">그룹장</div>
                    ${makeSelect('trial-leader-select', selectedLeader)}
                </div>
                <div>
                    <div style="font-size:0.8rem;color:#22c55e;font-weight:bold;margin-bottom:0.3rem">그룹원</div>
                    ${makeSelect('trial-member-select', selectedMember)}
                </div>
            `;
            document.getElementById('trial-leader-select')?.addEventListener('change', (e) => {
                selectedLeader = parseInt(e.target.value);
            });
            document.getElementById('trial-member-select')?.addEventListener('change', (e) => {
                selectedMember = parseInt(e.target.value);
            });
        }
    }

    function makeSelect(id, selectedHwnd) {
        const options = detectedWindows.map(w => {
            const sel = w.hwnd === selectedHwnd ? 'selected' : '';
            return `<option value="${w.hwnd}" ${sel}>${escapeHtml(w.name)}</option>`;
        }).join('');
        return `<select id="${id}" style="width:100%;padding:0.4rem;font-size:0.85rem;border-radius:6px;border:1px solid rgba(255,255,255,0.1);background:rgba(255,255,255,0.05);color:inherit">${options}</select>`;
    }

    async function startTrial() {
        if (trialRunning) return;

        if (detectedWindows.length === 0) {
            alert('먼저 감지 버튼으로 바람창을 감지해주세요');
            return;
        }

        const maxRuns = document.getElementById('trial-max-runs')?.value || 10;
        let apiMode, body;

        if (trialMode === 'solo') {
            if (!selectedSolo) { alert('캐릭터를 선택해주세요'); return; }
            apiMode = 'trial-solo';
            body = `mode=${apiMode}&trial_max_runs=${maxRuns}&hwnd=${selectedSolo}`;
            addLog(`솔로 시련 시작 (${getNameByHwnd(selectedSolo)}, ${maxRuns}회)`);
        } else {
            if (!selectedLeader || !selectedMember) { alert('그룹장과 그룹원을 선택해주세요'); return; }
            if (selectedLeader === selectedMember) { alert('그룹장과 그룹원은 다른 캐릭터여야 합니다'); return; }
            apiMode = 'trial-group';
            body = `mode=${apiMode}&trial_max_runs=${maxRuns}&leader_hwnd=${selectedLeader}&member_hwnd=${selectedMember}`;
            addLog(`그룹 시련 시작 (장:${getNameByHwnd(selectedLeader)}, 원:${getNameByHwnd(selectedMember)}, ${maxRuns}회)`);
        }

        try {
            const res = await fetch('/api/start', {
                method: 'POST',
                headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                body: body
            });

            if (res.ok) {
                trialRunning = true;
                updateStatus('실행 중');
                startLogPoll();
            } else {
                const text = await res.text();
                addLog('시작 실패: ' + text);
                alert('시작 실패: ' + text);
            }
        } catch(e) {
            addLog('시작 실패: ' + e.message);
        }
    }

    async function stopTrial() {
        try {
            await fetch('/api/stop', { method: 'POST' });
            trialRunning = false;
            updateStatus('중지됨');
            addLog('시련 중지됨');
            stopLogPoll();
        } catch(e) {
            addLog('중지 실패: ' + e.message);
        }
    }

    function startLogPoll() {
        if (logPollInterval) return;
        logPollInterval = setInterval(async () => {
            try {
                const res = await fetch('/api/status');
                if (res.ok) {
                    const data = await res.json();
                    if (!data.running && trialRunning) {
                        trialRunning = false;
                        updateStatus('완료');
                        addLog('시련 완료');
                        stopLogPoll();
                    }
                }
            } catch(e) {}
        }, 3000);
    }

    function stopLogPoll() {
        if (logPollInterval) {
            clearInterval(logPollInterval);
            logPollInterval = null;
        }
    }

    function getNameByHwnd(hwnd) {
        const w = detectedWindows.find(w => w.hwnd === hwnd);
        return w ? w.name : '?';
    }

    function updateStatus(text) {
        const el = document.getElementById('trial-status');
        if (el) el.textContent = text;
    }

    function escapeHtml(str) {
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();
