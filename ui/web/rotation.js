// 자동 사냥 프론트엔드 로직

(function() {
    'use strict';

    // 상태 변수
    let characters = [];
    let detectedWindows = [];
    let windowAssignments = {}; // characterId -> windowHwnd
    let excludedWindows = new Set(); // 제외된 윈도우 hwnd
    let screenshotCache = {}; // idx -> base64 data URL
    let rotationRunning = false;
    let editingCharId = null;
    let statusPollInterval = null;

    // DOM 요소
    const characterList = document.getElementById('character-list');
    const characterForm = document.getElementById('character-form');
    const addCharacterBtn = document.getElementById('add-character-btn');
    const saveCharacterBtn = document.getElementById('save-character-btn');
    const cancelCharacterBtn = document.getElementById('cancel-character-btn');
    const windowList = document.getElementById('window-list');
    const detectWindowsBtn = document.getElementById('detect-windows-btn');
    const applyAssignBtn = document.getElementById('apply-assign-btn');
    const rotationStartBtn = document.getElementById('rotation-start-btn');
    const rotationStopBtn = document.getElementById('rotation-stop-btn');
    const saveCoordsBtn = document.getElementById('save-coords-btn');
    const rotationBadge = document.getElementById('rotation-badge');
    const rotationCurrent = document.getElementById('rotation-current');
    const rotationTimer = document.getElementById('rotation-timer');
    const rotationRemaining = document.getElementById('rotation-remaining');
    const rotationQueue = document.getElementById('rotation-queue');
    const rotationLog = document.getElementById('rotation-log');

    // 초기화
    document.addEventListener('DOMContentLoaded', () => {
        if (!characterList) return;
        setupRotationListeners();
        loadCharacters();
        loadCoordinates();
        loadOCRConfig();
        checkOCRAvailability();
        setupScheduleControls();
        loadScheduleStatus();
        setInterval(loadScheduleStatus, 1000);
        setupMinimizeToggle();
        setupPeachCapture();
        loadPeachPreview();
    });

    // === 복숭아 아이콘 캡처 ===
    let peachScreenshotData = null;
    let peachImageWidth = 0;
    let peachImageHeight = 0;

    function setupPeachCapture() {
        const btn = document.getElementById('peach-capture-btn');
        if (btn) btn.addEventListener('click', openPeachCapture);
        const testBtn = document.getElementById('peach-test-btn');
        if (testBtn) testBtn.addEventListener('click', testPeachMatch);
    }

    async function testPeachMatch() {
        const btn = document.getElementById('peach-test-btn');
        const result = document.getElementById('peach-test-result');
        const summary = document.getElementById('peach-test-summary');
        const imgEl = document.getElementById('peach-test-image');
        if (!btn || !result || !summary || !imgEl) return;

        btn.disabled = true;
        btn.textContent = '매칭 중...';
        try {
            let hwnd = getFirstAssignedHwnd();
            if (!hwnd) {
                try {
                    const r = await fetch('/api/rotation/detect-with-ocr');
                    const list = await r.json();
                    if (list && list.length > 0) hwnd = list[0].hwnd;
                } catch(e) {}
            }
            const res = await fetch('/api/peach/test', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ hwnd: hwnd || 0 })
            });
            if (!res.ok) {
                const txt = await res.text();
                summary.innerHTML = `<span style="color:#ef4444">테스트 실패: ${escapeHtml(txt)}</span>`;
                imgEl.style.display = 'none';
                result.style.display = 'block';
                return;
            }
            const data = await res.json();
            if (data.found) {
                summary.innerHTML = `<span style="color:#22c55e">✅ 매칭 성공</span>&nbsp; ` +
                    `위치 (${data.x}, ${data.y}) / 중심 (${data.centerX}, ${data.centerY}) / scale ${data.scale.toFixed(2)} / ` +
                    `매칭 크기 ${data.matchedW}×${data.matchedH}px (needle ${data.needleW}×${data.needleH}px)`;
            } else {
                summary.innerHTML = `<span style="color:#ef4444">❌ 매칭 실패</span> — needle을 찾지 못했습니다. ` +
                    `복숭아 아이콘이 화면에 보이는지 확인하거나 다시 캡처해주세요.`;
            }
            imgEl.src = data.image;
            imgEl.style.display = '';
            result.style.display = 'block';
            addRotationLog(data.found ? '복숭아 매칭 테스트 성공' : '복숭아 매칭 테스트 실패');
        } catch (e) {
            summary.innerHTML = `<span style="color:#ef4444">테스트 오류: ${escapeHtml(e.message)}</span>`;
            result.style.display = 'block';
            imgEl.style.display = 'none';
        } finally {
            btn.disabled = false;
            btn.textContent = '매칭 테스트';
        }
    }

    async function openPeachCapture() {
        const hwnd = getFirstAssignedHwnd(); // 0이면 백엔드에서 자동 감지
        const btn = document.getElementById('peach-capture-btn');
        if (btn) { btn.textContent = '스크린샷 촬영 중...'; btn.disabled = true; }
        try {
            // hwnd 0이면 첫 게임 창을 사용. 백엔드 capture API에서 fallback 처리되지만
            // screenshot/full 은 hwnd 필수 → 미할당 시 게임 창 자동 탐색 결과 활용
            let useHwnd = hwnd;
            if (!useHwnd) {
                // 윈도우 감지 API로 첫 번째 hwnd 가져오기
                try {
                    const r = await fetch('/api/rotation/detect-with-ocr');
                    const list = await r.json();
                    if (list && list.length > 0) useHwnd = list[0].hwnd;
                } catch (e) {}
            }
            if (!useHwnd) {
                addRotationLog('게임 창을 찾을 수 없습니다. 먼저 윈도우를 감지해주세요.');
                return;
            }
            const r2 = await fetch('/api/rotation/screenshot/full?hwnd=' + useHwnd);
            const data = await r2.json();
            if (!data.image) throw new Error('스크린샷 없음');
            peachScreenshotData = data.image;
            peachImageWidth = data.width;
            peachImageHeight = data.height;
            showPeachRegionModal(useHwnd);
        } catch (e) {
            addRotationLog('스크린샷 촬영 실패: ' + e.message);
        } finally {
            if (btn) { btn.textContent = '스크린샷에서 영역 선택'; btn.disabled = false; }
        }
    }

    function showPeachRegionModal(hwnd) {
        const modal = document.getElementById('peach-region-modal');
        const canvas = document.getElementById('peach-region-canvas');
        const wrap = document.getElementById('peach-region-canvas-wrap');
        const selection = document.getElementById('peach-region-selection');
        const coordsDisplay = document.getElementById('peach-region-coords-display');
        const saveBtn = document.getElementById('peach-region-save-btn');
        const cancelBtn = document.getElementById('peach-region-cancel-btn');
        const closeBtn = document.getElementById('peach-region-modal-close');
        if (!modal || !canvas) return;

        modal.style.display = 'flex';
        saveBtn.disabled = true;
        selection.style.display = 'none';
        coordsDisplay.textContent = '영역을 드래그하세요';

        const img = new Image();
        img.onload = function() {
            const maxW = wrap.clientWidth - 4;
            const scale = maxW / img.width;
            const dispW = Math.floor(img.width * scale);
            const dispH = Math.floor(img.height * scale);
            canvas.width = dispW;
            canvas.height = dispH;
            wrap.style.height = dispH + 'px';
            const ctx = canvas.getContext('2d');
            ctx.drawImage(img, 0, 0, dispW, dispH);

            let dragging = false;
            let startX = 0, startY = 0;
            let selRect = { x: 0, y: 0, w: 0, h: 0 };

            function pos(e) {
                const r = canvas.getBoundingClientRect();
                return { x: e.clientX - r.left, y: e.clientY - r.top };
            }
            function showSel(x, y, w, h) {
                selection.style.display = 'block';
                selection.style.left = x + 'px';
                selection.style.top = y + 'px';
                selection.style.width = w + 'px';
                selection.style.height = h + 'px';
            }

            canvas.onmousedown = (e) => { dragging = true; const p = pos(e); startX = p.x; startY = p.y; selection.style.display = 'none'; e.preventDefault(); };
            canvas.onmousemove = (e) => {
                if (!dragging) return;
                const p = pos(e);
                const x = Math.min(startX, p.x), y = Math.min(startY, p.y);
                const w = Math.abs(p.x - startX), h = Math.abs(p.y - startY);
                showSel(x, y, w, h);
                selRect = { x, y, w, h };
                const ox = Math.round(x / scale), oy = Math.round(y / scale);
                const ow = Math.round(w / scale), oh = Math.round(h / scale);
                coordsDisplay.textContent = `X:${ox}, Y:${oy}, ${ow}x${oh}px`;
            };
            const finishDrag = () => {
                if (!dragging) return;
                dragging = false;
                if (selRect.w > 5 && selRect.h > 5) saveBtn.disabled = false;
            };
            canvas.onmouseup = finishDrag;
            canvas.onmouseleave = finishDrag;

            saveBtn.onclick = async () => {
                const ox = Math.round(selRect.x / scale);
                const oy = Math.round(selRect.y / scale);
                const ow = Math.round(selRect.w / scale);
                const oh = Math.round(selRect.h / scale);
                try {
                    const res = await fetch('/api/peach/capture', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ hwnd: hwnd, x: ox, y: oy, width: ow, height: oh })
                    });
                    if (!res.ok) {
                        const txt = await res.text();
                        alert('저장 실패: ' + txt);
                        return;
                    }
                    addRotationLog(`복숭아 아이콘 저장됨 (${ow}x${oh}px)`);
                    closeModal();
                    loadPeachPreview();
                } catch (e) {
                    alert('저장 실패: ' + e.message);
                }
            };

            function closeModal() {
                modal.style.display = 'none';
                canvas.onmousedown = null;
                canvas.onmousemove = null;
                canvas.onmouseup = null;
                canvas.onmouseleave = null;
            }
            cancelBtn.onclick = closeModal;
            closeBtn.onclick = closeModal;
        };
        img.src = peachScreenshotData;
    }

    async function loadPeachPreview() {
        try {
            const r = await fetch('/api/peach/preview');
            const data = await r.json();
            const wrap = document.getElementById('peach-preview-wrap');
            const imgEl = document.getElementById('peach-preview-img');
            if (data.exists && wrap && imgEl) {
                imgEl.src = data.image;
                wrap.style.display = 'flex';
            } else if (wrap) {
                wrap.style.display = 'none';
            }
        } catch (e) {}
    }

    // === 딸깍 (최소화 모드) ===
    function setupMinimizeToggle() {
        const t = document.getElementById('minimize-toggle');
        if (!t) return;
        t.addEventListener('change', () => {
            saveCoordinates(); // 좌표 API에 함께 저장
            addRotationLog(`딸깍 모드 ${t.checked ? 'ON' : 'OFF'} — 저장됨`);
        });
    }

    // === 자동사냥 예약 ===
    function setupScheduleControls() {
        const toggle = document.getElementById('schedule-toggle');
        const timeInput = document.getElementById('schedule-time');
        if (!toggle || !timeInput) return;
        toggle.addEventListener('change', async () => {
            if (toggle.checked) {
                const time = timeInput.value || '12:00';
                try {
                    const res = await fetch('/api/rotation/schedule', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ time })
                    });
                    if (!res.ok) {
                        const txt = await res.text();
                        alert('예약 실패: ' + txt);
                        toggle.checked = false;
                    } else {
                        addRotationLog(`자동사냥 ${time}에 예약됨`);
                    }
                } catch (e) {
                    alert('예약 실패: ' + e.message);
                    toggle.checked = false;
                }
            } else {
                await fetch('/api/rotation/schedule', { method: 'DELETE' });
                addRotationLog('자동사냥 예약 취소됨');
            }
            loadScheduleStatus();
        });
    }

    async function loadScheduleStatus() {
        const toggle = document.getElementById('schedule-toggle');
        const status = document.getElementById('schedule-status');
        const nowEl = document.getElementById('schedule-now');
        if (!toggle || !status) return;
        try {
            const res = await fetch('/api/rotation/schedule');
            const data = await res.json();
            if (nowEl && data.nowKST) nowEl.textContent = data.nowKST;
            if (data.running) {
                toggle.checked = true;
                const remain = data.remainingSeconds || 0;
                const h = Math.floor(remain / 3600);
                const m = Math.floor((remain % 3600) / 60);
                const s = remain % 60;
                status.textContent = `${data.targetTimeKST}에 시작 예정 (남은 ${h}h ${m}m ${s}s)`;
                status.style.color = '#22c55e';
            } else {
                if (toggle.checked) toggle.checked = false;
                status.textContent = '예약 없음';
                status.style.color = '';
            }
        } catch (e) {
            // ignore
        }
    }

    function setupRotationListeners() {
        if (addCharacterBtn) addCharacterBtn.addEventListener('click', showAddForm);
        if (saveCharacterBtn) saveCharacterBtn.addEventListener('click', saveCharacter);
        if (cancelCharacterBtn) cancelCharacterBtn.addEventListener('click', hideForm);
        if (detectWindowsBtn) detectWindowsBtn.addEventListener('click', detectWindows);
        if (applyAssignBtn) applyAssignBtn.addEventListener('click', applyAssignments);
        const autoAssignBtn = document.getElementById('auto-assign-btn');
        if (autoAssignBtn) autoAssignBtn.addEventListener('click', autoAssign);
        if (rotationStartBtn) rotationStartBtn.addEventListener('click', startRotation);
        if (rotationStopBtn) rotationStopBtn.addEventListener('click', stopRotation);
        if (saveCoordsBtn) saveCoordsBtn.addEventListener('click', saveCoordinates);
        const ocrSelectBtn = document.getElementById('ocr-select-region-btn');
        if (ocrSelectBtn) ocrSelectBtn.addEventListener('click', openOCRRegionSelector);
        const ocrTestBtn = document.getElementById('ocr-test-btn');
        if (ocrTestBtn) ocrTestBtn.addEventListener('click', testOCR);
    }

    // === 캐릭터 CRUD ===

    function loadCharacters() {
        fetch('/api/rotation/characters')
            .then(r => r.json())
            .then(data => {
                characters = data || [];
                renderCharacters();
            })
            .catch(() => { characters = []; renderCharacters(); });
    }

    function renderCharacters() {
        if (!characterList) return;
        if (characters.length === 0) {
            characterList.innerHTML = '<p class="empty-placeholder">등록된 캐릭터가 없습니다.</p>';
            return;
        }

        characterList.innerHTML = characters.map((c, i) => `
            <div class="char-item ${c.enabled === false ? 'disabled' : ''}" data-id="${c.id}">
                <label class="char-toggle">
                    <input type="checkbox" ${c.enabled !== false ? 'checked' : ''} onchange="rotationToggleChar('${c.id}', this.checked)">
                </label>
                <div class="char-order">${i + 1}</div>
                <div class="char-info">
                    <div class="char-name">${escapeHtml(c.name)}</div>
                    <div class="char-detail">${escapeHtml(c.huntingArea?.name || '')} (${c.huntingArea?.dropdownIndex || 0}번째) / ${c.durationMins}분</div>
                </div>
                <div class="char-actions">
                    <button class="char-move-btn" onclick="rotationMoveChar('${c.id}', -1)" ${i === 0 ? 'disabled' : ''} title="위로">▲</button>
                    <button class="char-move-btn" onclick="rotationMoveChar('${c.id}', 1)" ${i === characters.length - 1 ? 'disabled' : ''} title="아래로">▼</button>
                    <button class="char-edit-btn" onclick="rotationEditChar('${c.id}')">수정</button>
                    <button class="char-delete-btn" onclick="rotationDeleteChar('${c.id}')">삭제</button>
                </div>
            </div>
        `).join('');
    }

    function showAddForm() {
        editingCharId = null;
        document.getElementById('char-name').value = '';
        document.getElementById('char-area').value = '';
        document.getElementById('char-dropdown-index').value = '0';
        document.getElementById('char-duration').value = '120';
        const peachSel = document.getElementById('char-peach-type');
        if (peachSel) peachSel.value = '';
        characterForm.style.display = 'block';
        characterForm.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    }

    function hideForm() {
        characterForm.style.display = 'none';
        editingCharId = null;
    }

    function saveCharacter() {
        const name = document.getElementById('char-name').value.trim();
        const area = document.getElementById('char-area').value.trim();
        const dropdownIndex = parseInt(document.getElementById('char-dropdown-index').value) || 0;
        const duration = parseInt(document.getElementById('char-duration').value) || 120;
        const peachType = document.getElementById('char-peach-type')?.value || '';

        if (!name) { alert('캐릭터 이름을 입력해주세요.'); return; }
        if (!area) { alert('사냥터 이름을 입력해주세요.'); return; }

        // 수정 모드면 기존 order/enabled 유지, 신규면 마지막 순서로
        const existing = editingCharId ? characters.find(c => c.id === editingCharId) : null;
        const order = existing ? existing.order : characters.length;
        const enabled = existing ? existing.enabled : true;

        const profile = {
            name: name,
            huntingArea: { name: area, dropdownIndex: dropdownIndex },
            durationMins: duration,
            order: order,
            enabled: enabled,
            peachType: peachType
        };

        if (editingCharId) {
            profile.id = editingCharId;
            fetch('/api/rotation/characters', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(profile)
            })
            .then(r => { if (r.ok) { hideForm(); loadCharacters(); addRotationLog(`${name} 수정됨`); } });
        } else {
            fetch('/api/rotation/characters', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(profile)
            })
            .then(r => { if (r.ok) { hideForm(); loadCharacters(); addRotationLog(`${name} 추가됨`); } });
        }
    }

    // 전역 함수로 노출
    window.rotationEditChar = function(id) {
        const char = characters.find(c => c.id === id);
        if (!char) return;
        editingCharId = id;
        document.getElementById('char-name').value = char.name;
        document.getElementById('char-area').value = char.huntingArea?.name || '';
        document.getElementById('char-dropdown-index').value = char.huntingArea?.dropdownIndex || 0;
        document.getElementById('char-duration').value = char.durationMins;
        const peachSel = document.getElementById('char-peach-type');
        if (peachSel) peachSel.value = char.peachType || '';
        characterForm.style.display = 'block';
        characterForm.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    };

    window.rotationDeleteChar = function(id) {
        if (!confirm('정말 삭제하시겠습니까?')) return;
        fetch(`/api/rotation/characters?id=${id}`, { method: 'DELETE' })
            .then(r => { if (r.ok) { loadCharacters(); addRotationLog('캐릭터 삭제됨'); } });
    };

    window.rotationToggleChar = function(id, enabled) {
        fetch('/api/rotation/characters/toggle', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id, enabled })
        })
        .then(r => { if (r.ok) loadCharacters(); });
    };

    window.rotationMoveChar = function(id, direction) {
        fetch('/api/rotation/characters/move', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id, direction })
        })
        .then(r => { if (r.ok) loadCharacters(); });
    };

    // 드롭다운 선택 변경 시 즉시 할당 반영
    window.rotationWindowAssignChanged = function() {
        applyAssignments();
    };

    // 윈도우 포함/제외 토글
    window.rotationToggleWindow = function(hwnd, included) {
        if (included) {
            excludedWindows.delete(hwnd);
        } else {
            excludedWindows.add(hwnd);
            // 제외된 창에 할당된 캐릭터가 있으면 해제
            for (const charId in windowAssignments) {
                if (windowAssignments[charId] === hwnd) {
                    delete windowAssignments[charId];
                }
            }
        }
        renderWindows();
        // 할당 상태에 따라 캐릭터 활성화/비활성화 동기화
        syncCharacterEnabled();
        addRotationLog(included ? '윈도우 포함됨' : '윈도우 제외됨');
    };

    // === 윈도우 감지 & 할당 ===

    async function detectWindows() {
        detectWindowsBtn.textContent = '감지 중...';
        detectWindowsBtn.disabled = true;

        try {
            let useOCR = false;
            let data;

            // OCR 감지 먼저 시도
            try {
                const r = await fetch('/api/rotation/detect-with-ocr');
                if (r.ok) {
                    data = await r.json();
                    useOCR = true;
                }
            } catch (e) {
                // OCR 실패 시 기존 방식 fallback
            }

            // OCR 실패 시 기존 방식
            if (!useOCR) {
                const r = await fetch('/api/rotation/windows');
                data = await r.json();
            }

            detectedWindows = data || [];
            screenshotCache = {};
            addRotationLog(`${detectedWindows.length}개의 게임 창 감지됨`);

            if (useOCR && characters.length > 0) {
                ocrAutoAssign(data);
            } else if (characters.length > 0 && detectedWindows.length > 0) {
                autoAssign();
                addRotationLog('캐릭터를 감지된 창에 자동 할당했습니다.');
            }

            renderWindows();
            loadScreenshotsSequential(0);
        } catch (e) {
            windowList.innerHTML = '<p class="empty-placeholder">창 감지에 실패했습니다.</p>';
        } finally {
            detectWindowsBtn.textContent = '감지';
            detectWindowsBtn.disabled = false;
        }
    }

    function ocrAutoAssign(ocrResults) {
        const assignments = [];
        windowAssignments = {};

        for (const win of ocrResults) {
            if (win.matchedId && win.confidence !== 'none') {
                assignments.push({
                    characterId: win.matchedId,
                    windowHwnd: win.hwnd
                });
                windowAssignments[win.matchedId] = win.hwnd;
                const matchType = win.confidence === 'exact' ? '정확' : win.confidence === 'remaining' ? '소거법' : '부분';
                addRotationLog(`OCR: "${win.detectedName || '(미인식)'}" → ${win.matchedName} (${matchType} 일치)`);
            }
        }

        if (assignments.length > 0) {
            sendAssignments(assignments);
            addRotationLog(`OCR로 ${assignments.length}개 캐릭터 자동 할당 완료`);
        }

        // 매칭 실패한 캐릭터 안내
        const matchedCharIds = new Set(assignments.map(a => a.characterId));
        const unmatchedChars = characters.filter(c => !matchedCharIds.has(c.id));
        if (unmatchedChars.length > 0) {
            addRotationLog(`${unmatchedChars.length}개 캐릭터 OCR 매칭 실패 - 수동 할당 필요`);
        }

        syncCharacterEnabled();
    }

    async function loadScreenshotsSequential(idx) {
        if (idx >= detectedWindows.length) return;
        const w = detectedWindows[idx];
        try {
            const r = await fetch(`/api/rotation/screenshot?hwnd=${w.hwnd}`);
            const data = await r.json();
            if (data.image) {
                screenshotCache[idx] = data.image;
            }
            const imgEl = document.getElementById(`window-thumb-${idx}`);
            const loadingEl = document.getElementById(`window-thumb-loading-${idx}`);
            if (imgEl && data.image) {
                imgEl.src = data.image;
                imgEl.style.display = 'block';
            }
            if (loadingEl) loadingEl.style.display = 'none';
        } catch(e) {
            const loadingEl = document.getElementById(`window-thumb-loading-${idx}`);
            if (loadingEl) loadingEl.textContent = '스크린샷 불가';
        }
        // 다음 창
        loadScreenshotsSequential(idx + 1);
    }

    function renderWindows() {
        const assignButtons = document.getElementById('assign-buttons');
        if (detectedWindows.length === 0) {
            windowList.innerHTML = '<p class="empty-placeholder">게임 창을 찾지 못했습니다.</p>';
            if (assignButtons) assignButtons.style.display = 'none';
            return;
        }

        windowList.innerHTML = detectedWindows.map((w, idx) => {
            const isExcluded = excludedWindows.has(w.hwnd);
            const cached = screenshotCache[idx];
            const charOptions = characters.map(c =>
                `<option value="${c.id}" ${windowAssignments[c.id] == w.hwnd ? 'selected' : ''}>${escapeHtml(c.name)}</option>`
            ).join('');

            // OCR 감지 이름 표시
            const detectedName = w.detectedName || '';
            const confidence = w.confidence || '';
            let ocrBadge = '';
            if (detectedName) {
                ocrBadge = `<span class="ocr-badge ${confidence}">${escapeHtml(detectedName)}</span>`;
            } else if (w.confidence === 'none') {
                ocrBadge = '<span class="ocr-badge none">OCR 미감지</span>';
            }

            return `
                <div class="window-item-card ${isExcluded ? 'excluded' : ''}" data-hwnd="${w.hwnd}">
                    <div class="window-item-header">
                        <label class="window-toggle">
                            <input type="checkbox" ${!isExcluded ? 'checked' : ''} onchange="rotationToggleWindow(${w.hwnd}, this.checked)">
                        </label>
                        <div class="window-order">${idx + 1}</div>
                        <div class="window-info">
                            <div class="window-title">${escapeHtml(w.title)}</div>
                            ${ocrBadge ? `<div class="window-ocr-name">${ocrBadge}</div>` : ''}
                        </div>
                        <select class="window-assign-select" data-hwnd="${w.hwnd}" ${isExcluded ? 'disabled' : ''} onchange="rotationWindowAssignChanged()">
                            <option value="">-- 미할당 --</option>
                            ${charOptions}
                        </select>
                    </div>
                    <div class="window-thumb-container">
                        <img id="window-thumb-${idx}" class="window-thumb" ${cached ? `src="${cached}" style="display:block"` : 'style="display:none"'} alt="스크린샷">
                        <div class="window-thumb-loading" id="window-thumb-loading-${idx}" ${cached ? 'style="display:none"' : ''}>스크린샷 로딩...</div>
                    </div>
                </div>
            `;
        }).join('');

        if (assignButtons) assignButtons.style.display = 'flex';
    }

    function applyAssignments() {
        const selects = document.querySelectorAll('.window-assign-select');
        const assignments = [];
        windowAssignments = {};

        selects.forEach(select => {
            const charId = select.value;
            const hwnd = parseInt(select.dataset.hwnd);
            // 제외된 창은 할당하지 않음
            if (charId && hwnd && !excludedWindows.has(hwnd)) {
                assignments.push({ characterId: charId, windowHwnd: hwnd });
                windowAssignments[charId] = hwnd;
            }
        });

        if (assignments.length === 0) {
            addRotationLog('할당할 캐릭터를 선택해주세요.');
            return;
        }

        sendAssignments(assignments);
        // 할당 상태에 따라 캐릭터 활성화/비활성화 동기화
        syncCharacterEnabled();
    }

    function autoAssign() {
        const assignments = [];
        windowAssignments = {};

        // 제외되지 않은 창만 사용
        const availableWindows = detectedWindows.filter(w => !excludedWindows.has(w.hwnd));
        const count = Math.min(characters.length, availableWindows.length);

        for (let i = 0; i < count; i++) {
            assignments.push({
                characterId: characters[i].id,
                windowHwnd: availableWindows[i].hwnd
            });
            windowAssignments[characters[i].id] = availableWindows[i].hwnd;
        }

        sendAssignments(assignments);
        renderWindows();
        // 할당 상태에 따라 캐릭터 활성화/비활성화 동기화
        syncCharacterEnabled();
    }

    function sendAssignments(assignments) {
        fetch('/api/rotation/assign', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(assignments)
        })
        .then(r => {
            if (r.ok) {
                const names = assignments.map(a => {
                    const c = characters.find(ch => ch.id === a.characterId);
                    return c ? c.name : '?';
                }).join(', ');
                addRotationLog(`할당 완료: ${names} (${assignments.length}개)`);
            }
        });
    }

    // 할당 상태에 따라 캐릭터 활성화/비활성화 자동 동기화
    function syncCharacterEnabled() {
        const assignedIds = new Set(Object.keys(windowAssignments));
        const togglePromises = [];

        for (const c of characters) {
            const shouldBeEnabled = assignedIds.has(c.id);
            const currentlyEnabled = c.enabled !== false;

            if (shouldBeEnabled !== currentlyEnabled) {
                togglePromises.push(
                    fetch('/api/rotation/characters/toggle', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ id: c.id, enabled: shouldBeEnabled })
                    })
                );
            }
        }

        if (togglePromises.length > 0) {
            Promise.all(togglePromises).then(() => {
                loadCharacters(); // 캐릭터 목록 새로고침 (체크박스 반영)
            });
        }
    }

    // === 좌표 설정 ===

    function loadCoordinates() {
        fetch('/api/rotation/coordinates')
            .then(r => r.json())
            .then(data => {
                if (data) {
                    setCoordValue('coord-sword-x', data.swordButtonX);
                    setCoordValue('coord-sword-y', data.swordButtonY);
                    setCoordValue('coord-dropdown-x', data.dropdownArrowX);
                    setCoordValue('coord-dropdown-y', data.dropdownArrowY);
                    setCoordValue('coord-item-height', data.dropdownItemHeight);
                    setCoordValue('coord-first-y', data.dropdownFirstItemY);
                    setCoordValue('coord-start-x', data.startButtonX);
                    setCoordValue('coord-start-y', data.startButtonY);
                    setCoordValue('coord-confirm-x', data.confirmButtonX || 0);
                    setCoordValue('coord-confirm-y', data.confirmButtonY || 0);
                    setCoordValue('coord-alert-confirm-x', data.alertConfirmX || 0);
                    setCoordValue('coord-alert-confirm-y', data.alertConfirmY || 0);
                    setCoordValue('coord-revive-x', data.reviveX || 0);
                    setCoordValue('coord-revive-y', data.reviveY || 0);
                    setCoordValue('coord-sect-x', data.sectButtonX || 0);
                    setCoordValue('coord-sect-y', data.sectButtonY || 0);
                    setCoordValue('coord-peach-receive-x', data.peachReceiveX || 0);
                    setCoordValue('coord-peach-receive-y', data.peachReceiveY || 0);
                    setCoordValue('coord-receive-accept-x', data.receiveAcceptX || 0);
                    setCoordValue('coord-receive-accept-y', data.receiveAcceptY || 0);
                    const minToggle = document.getElementById('minimize-toggle');
                    if (minToggle) minToggle.checked = !!data.minimizeAfterStart;
                    updateCoordDisplays();
                }
            })
            .catch(() => {});
    }

    function updateCoordDisplays() {
        const sx = getCoordValue('coord-sword-x'), sy = getCoordValue('coord-sword-y');
        const dx = getCoordValue('coord-dropdown-x'), dy = getCoordValue('coord-dropdown-y');
        const bx = getCoordValue('coord-start-x'), by = getCoordValue('coord-start-y');
        const fiy = getCoordValue('coord-first-y');
        const ih = getCoordValue('coord-item-height');

        const cx = getCoordValue('coord-confirm-x'), cy = getCoordValue('coord-confirm-y');
        const acx = getCoordValue('coord-alert-confirm-x'), acy = getCoordValue('coord-alert-confirm-y');
        const rvx = getCoordValue('coord-revive-x'), rvy = getCoordValue('coord-revive-y');
        const sectX = getCoordValue('coord-sect-x'), sectY = getCoordValue('coord-sect-y');
        const prX = getCoordValue('coord-peach-receive-x'), prY = getCoordValue('coord-peach-receive-y');
        const raX = getCoordValue('coord-receive-accept-x'), raY = getCoordValue('coord-receive-accept-y');

        const swordDisp = document.getElementById('coord-sword-display');
        const dropdownDisp = document.getElementById('coord-dropdown-display');
        const startDisp = document.getElementById('coord-start-display');
        const firstItemDisp = document.getElementById('coord-first-item-display');
        const itemHeightDisp = document.getElementById('coord-item-height-display');
        const confirmDisp = document.getElementById('coord-confirm-display');
        const alertConfirmDisp = document.getElementById('coord-alert-confirm-display');
        const reviveDisp = document.getElementById('coord-revive-display');

        if (swordDisp) swordDisp.textContent = (sx || sy) ? `(${sx}, ${sy})` : '미설정';
        if (dropdownDisp) dropdownDisp.textContent = (dx || dy) ? `(${dx}, ${dy})` : '미설정';
        if (startDisp) startDisp.textContent = (bx || by) ? `(${bx}, ${by})` : '미설정';
        if (firstItemDisp) firstItemDisp.textContent = fiy > 0 ? `Y: ${fiy}` : '미설정';
        if (itemHeightDisp) itemHeightDisp.textContent = ih > 0 ? `${ih}px` : '미설정';
        if (confirmDisp) confirmDisp.textContent = (cx || cy) ? `(${cx}, ${cy})` : '미설정';
        if (alertConfirmDisp) alertConfirmDisp.textContent = (acx || acy) ? `(${acx}, ${acy})` : '미설정';
        if (reviveDisp) reviveDisp.textContent = (rvx || rvy) ? `(${rvx}, ${rvy})` : '미설정';

        const sectDisp = document.getElementById('coord-sect-display');
        const peachReceiveDisp = document.getElementById('coord-peach-receive-display');
        const receiveAcceptDisp = document.getElementById('coord-receive-accept-display');
        if (sectDisp) sectDisp.textContent = (sectX || sectY) ? `(${sectX}, ${sectY})` : '미설정';
        if (peachReceiveDisp) peachReceiveDisp.textContent = (prX || prY) ? `(${prX}, ${prY})` : '미설정';
        if (receiveAcceptDisp) receiveAcceptDisp.textContent = (raX || raY) ? `(${raX}, ${raY})` : '미설정';
    }

    function saveCoordinates() {
        const coords = {
            swordButtonX: getCoordValue('coord-sword-x'),
            swordButtonY: getCoordValue('coord-sword-y'),
            dropdownArrowX: getCoordValue('coord-dropdown-x'),
            dropdownArrowY: getCoordValue('coord-dropdown-y'),
            dropdownItemHeight: getCoordValue('coord-item-height'),
            dropdownFirstItemY: getCoordValue('coord-first-y'),
            startButtonX: getCoordValue('coord-start-x'),
            startButtonY: getCoordValue('coord-start-y'),
            confirmButtonX: getCoordValue('coord-confirm-x'),
            confirmButtonY: getCoordValue('coord-confirm-y'),
            alertConfirmX: getCoordValue('coord-alert-confirm-x'),
            alertConfirmY: getCoordValue('coord-alert-confirm-y'),
            reviveX: getCoordValue('coord-revive-x'),
            reviveY: getCoordValue('coord-revive-y'),
            sectButtonX: getCoordValue('coord-sect-x'),
            sectButtonY: getCoordValue('coord-sect-y'),
            peachReceiveX: getCoordValue('coord-peach-receive-x'),
            peachReceiveY: getCoordValue('coord-peach-receive-y'),
            receiveAcceptX: getCoordValue('coord-receive-accept-x'),
            receiveAcceptY: getCoordValue('coord-receive-accept-y'),
            minimizeAfterStart: !!document.getElementById('minimize-toggle')?.checked
        };

        fetch('/api/rotation/coordinates', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(coords)
        })
        .then(r => {
            if (r.ok) addRotationLog('좌표 설정 저장됨');
        });
    }

    function setCoordValue(id, val) {
        const el = document.getElementById(id);
        if (el) el.value = val;
    }

    function getCoordValue(id) {
        const el = document.getElementById(id);
        return el ? parseInt(el.value) || 0 : 0;
    }

    // === 클릭 캡처 모드 ===

    function setupCaptureBtns() {
        // 이벤트 위임: 동적/정적 버튼 모두 처리
        document.addEventListener('click', (e) => {
            const btn = e.target.closest('.capture-btn');
            if (!btn) return;
            const target = btn.dataset.target;
            const msg = btn.dataset.msg;
            if (target && msg) startCapture(target, msg);
        });
    }

    function getFirstAssignedHwnd() {
        for (const charId in windowAssignments) {
            if (windowAssignments[charId]) return windowAssignments[charId];
        }
        if (detectedWindows.length > 0) return detectedWindows[0].hwnd;
        return 0; // 백엔드에서 자동 감지
    }

    let captureInProgress = false;
    let captureInterval = null;

    function startCapture(target, message) {
        if (captureInProgress) return;

        const hwnd = getFirstAssignedHwnd(); // 0이면 백엔드에서 자동 감지

        const overlay = document.getElementById('capture-overlay');
        const countdownEl = document.getElementById('capture-countdown');
        const messageEl = document.getElementById('capture-message');

        if (!overlay) return;

        captureInProgress = true;

        // 이전 interval 정리
        if (captureInterval) {
            clearInterval(captureInterval);
            captureInterval = null;
        }

        overlay.style.display = 'flex';
        messageEl.textContent = message;

        let count = 3;
        countdownEl.textContent = count;

        captureInterval = setInterval(() => {
            count--;
            if (count > 0) {
                countdownEl.textContent = count;
            } else {
                clearInterval(captureInterval);
                captureInterval = null;
                countdownEl.textContent = '...';
                messageEl.textContent = '마우스 위치를 감지하는 중...';

                fetch('/api/rotation/capture', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ windowHwnd: hwnd, delaySec: 0 })
                })
                .then(r => r.json())
                .then(data => {
                    if (target === 'sword') {
                        setCoordValue('coord-sword-x', data.x);
                        setCoordValue('coord-sword-y', data.y);
                    } else if (target === 'dropdown') {
                        setCoordValue('coord-dropdown-x', data.x);
                        setCoordValue('coord-dropdown-y', data.y);
                    } else if (target === 'start') {
                        setCoordValue('coord-start-x', data.x);
                        setCoordValue('coord-start-y', data.y);
                    } else if (target === 'confirm') {
                        setCoordValue('coord-confirm-x', data.x);
                        setCoordValue('coord-confirm-y', data.y);
                    } else if (target === 'alert-confirm') {
                        setCoordValue('coord-alert-confirm-x', data.x);
                        setCoordValue('coord-alert-confirm-y', data.y);
                    } else if (target === 'revive') {
                        setCoordValue('coord-revive-x', data.x);
                        setCoordValue('coord-revive-y', data.y);
                    } else if (target === 'sect') {
                        setCoordValue('coord-sect-x', data.x);
                        setCoordValue('coord-sect-y', data.y);
                    } else if (target === 'peach-receive') {
                        setCoordValue('coord-peach-receive-x', data.x);
                        setCoordValue('coord-peach-receive-y', data.y);
                    } else if (target === 'receive-accept') {
                        setCoordValue('coord-receive-accept-x', data.x);
                        setCoordValue('coord-receive-accept-y', data.y);
                    } else if (target === 'firstitem') {
                        setCoordValue('coord-first-y', data.y);
                    } else if (target === 'seconditem') {
                        const firstY = getCoordValue('coord-first-y');
                        if (firstY > 0) {
                            const height = data.y - firstY;
                            if (height > 0) {
                                setCoordValue('coord-item-height', height);
                                addRotationLog(`항목 높이 자동 계산: ${height}px (두 번째 Y=${data.y} - 첫 번째 Y=${firstY})`);
                            } else {
                                addRotationLog('오류: 두 번째 항목이 첫 번째보다 위에 있습니다. 다시 시도해주세요.');
                            }
                        } else {
                            addRotationLog('먼저 "첫 번째 항목 위치"를 찍어주세요.');
                        }
                    }

                    updateCoordDisplays();
                    addRotationLog(`${message.replace('를 클릭하세요', '').replace('에 마우스를 올리세요', '')} 좌표: (${data.x}, ${data.y})`);
                    saveCoordinates();
                    overlay.style.display = 'none';
                    captureInProgress = false;
                })
                .catch(err => {
                    addRotationLog('좌표 캡처 실패: ' + err.message);
                    overlay.style.display = 'none';
                    captureInProgress = false;
                });
            }
        }, 1000);
    }

    // 초기화 시 캡처 버튼도 설정
    document.addEventListener('DOMContentLoaded', () => {
        setTimeout(setupCaptureBtns, 100);
    });

    // === 자동 사냥 시작/중지 ===

    async function startRotation() {
        // 예약이 걸려있으면 즉시 시작 방지 (사용자가 실수로 시작 버튼 눌렀을 가능성)
        try {
            const r = await fetch('/api/rotation/schedule');
            const sd = await r.json();
            if (sd && sd.running) {
                const ok = confirm(`현재 ${sd.targetTimeKST}에 예약된 자동사냥이 있습니다.\n\n예약을 무시하고 지금 즉시 시작하시겠습니까?\n\n[확인] = 예약 취소 + 즉시 시작\n[취소] = 아무 동작 안 함`);
                if (!ok) {
                    addRotationLog('예약 유지 — 즉시 시작 취소');
                    return;
                }
                // 예약 취소
                await fetch('/api/rotation/schedule', { method: 'DELETE' });
                addRotationLog('예약 취소됨 — 즉시 시작 진행');
            }
        } catch (e) {
            // schedule API 실패해도 시작 진행
        }

        // 시작 전 좌표 검증
        const itemHeight = getCoordValue('coord-item-height');
        const firstY = getCoordValue('coord-first-y');
        const swordX = getCoordValue('coord-sword-x');
        const swordY = getCoordValue('coord-sword-y');
        const dropX = getCoordValue('coord-dropdown-x');
        const dropY = getCoordValue('coord-dropdown-y');
        const startX = getCoordValue('coord-start-x');
        const startY = getCoordValue('coord-start-y');

        const warnings = [];
        if (!swordX && !swordY) warnings.push('칼 버튼');
        if (!dropX && !dropY) warnings.push('드롭다운 화살표');
        if (!firstY) warnings.push('첫 번째 항목 위치');
        if (!itemHeight) warnings.push('항목 높이 (두 번째 항목 미설정)');
        if (!startX && !startY) warnings.push('시작 버튼');

        if (warnings.length > 0) {
            const msg = `다음 좌표가 설정되지 않았습니다:\n- ${warnings.join('\n- ')}\n\n그래도 시작하시겠습니까?`;
            if (!confirm(msg)) return;
        }

        fetch('/api/rotation/start', { method: 'POST' })
            .then(r => {
                if (r.ok) {
                    rotationRunning = true;
                    updateRotationUI();
                    addRotationLog('자동 사냥 시작!');
                    startStatusPolling();
                } else {
                    return r.text().then(t => { throw new Error(t); });
                }
            })
            .catch(err => {
                addRotationLog('시작 실패: ' + err.message);
            });
    }

    function stopRotation() {
        fetch('/api/rotation/stop', { method: 'POST' })
            .then(r => {
                if (r.ok) {
                    rotationRunning = false;
                    updateRotationUI();
                    addRotationLog('자동 사냥 중지');
                    stopStatusPolling();
                }
            });
    }

    // === 상태 폴링 ===

    function startStatusPolling() {
        stopStatusPolling();
        pollStatus();
        statusPollInterval = setInterval(pollStatus, 2000);
    }

    function stopStatusPolling() {
        if (statusPollInterval) {
            clearInterval(statusPollInterval);
            statusPollInterval = null;
        }
    }

    function pollStatus() {
        fetch('/api/rotation/status')
            .then(r => r.json())
            .then(data => {
                updateStatusDisplay(data);
                if (!data.running && rotationRunning) {
                    rotationRunning = false;
                    updateRotationUI();
                    stopStatusPolling();
                }
            })
            .catch(() => {});
    }

    function updateStatusDisplay(status) {
        if (!status) return;

        if (rotationBadge) {
            const stateLabels = {
                'idle': '대기', 'activating': '활성화', 'starting': '사냥시작',
                'hunting': '사냥중', 'switching': '전환중', 'complete': '완료'
            };
            rotationBadge.textContent = stateLabels[status.state] || status.state;
            rotationBadge.className = 'rotation-badge ' + (status.running ? 'running' : '');
        }

        if (rotationCurrent && status.running) {
            rotationCurrent.textContent = `${status.currentCharacter || ''} - ${status.currentArea || ''} (${status.completedCount}/${status.totalCharacters})`;
        } else if (rotationCurrent && status.state === 'complete') {
            rotationCurrent.textContent = '모든 캐릭터 사냥 완료!';
        }

        if (rotationTimer && rotationRemaining) {
            if (status.running && status.remainingSeconds > 0) {
                rotationTimer.style.display = 'flex';
                const mins = Math.floor(status.remainingSeconds / 60);
                const secs = status.remainingSeconds % 60;
                rotationRemaining.textContent = `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
            } else {
                rotationTimer.style.display = 'none';
            }
        }

        if (rotationQueue && characters.length > 0) {
            const enabledChars = characters.filter(c => c.enabled !== false);
            rotationQueue.innerHTML = enabledChars.map((c, i) => {
                let stateClass = 'pending';
                if (i < status.completedCount) stateClass = 'completed';
                else if (i === status.currentIndex && status.running) stateClass = 'active';
                return `<div class="queue-item ${stateClass}">${escapeHtml(c.name)}</div>`;
            }).join('');
        }
    }

    function updateRotationUI() {
        if (rotationStartBtn) {
            rotationStartBtn.disabled = rotationRunning;
            if (rotationRunning) rotationStartBtn.classList.add('active');
            else rotationStartBtn.classList.remove('active');
        }
        if (rotationStopBtn) {
            rotationStopBtn.disabled = !rotationRunning;
            if (rotationRunning) rotationStopBtn.classList.remove('active');
        }
    }

    // === 자동 사냥 로그 ===

    function addRotationLog(message) {
        if (!rotationLog) return;
        const time = new Date().toLocaleTimeString('ko-KR');
        const entry = document.createElement('div');
        entry.className = 'rotation-log-entry';
        entry.textContent = `[${time}] ${message}`;
        rotationLog.appendChild(entry);
        rotationLog.scrollTop = rotationLog.scrollHeight;

        while (rotationLog.children.length > 100) {
            rotationLog.removeChild(rotationLog.firstChild);
        }
    }

    // === 백엔드 이벤트 수신 ===

    const originalDispatch = window.dispatchAppEvent;
    window.dispatchAppEvent = function(event) {
        if (originalDispatch) originalDispatch(event);

        const { type, payload } = event;
        switch (type) {
            case 'rotationStatus':
                updateStatusDisplay(payload);
                break;
            case 'rotationLog':
                if (payload && payload.message) addRotationLog(payload.message);
                break;
            case 'rotationComplete':
                rotationRunning = false;
                updateRotationUI();
                stopStatusPolling();
                addRotationLog('자동 사냥 완료!');
                break;
            case 'rotationError':
                if (payload && payload.message) addRotationLog('오류: ' + payload.message);
                rotationRunning = false;
                updateRotationUI();
                stopStatusPolling();
                break;
        }
    };

    // === 유틸리티 ===

    function escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text || '';
        return div.innerHTML;
    }

    // === OCR 설정 ===

    let ocrScreenshotData = null; // 원본 스크린샷 데이터
    let ocrImageWidth = 0;
    let ocrImageHeight = 0;

    function loadOCRConfig() {
        fetch('/api/rotation/ocr/config')
            .then(r => r.json())
            .then(data => {
                if (data) {
                    setCoordValue('ocr-region-x', data.nameRegionX);
                    setCoordValue('ocr-region-y', data.nameRegionY);
                    setCoordValue('ocr-region-w', data.nameRegionWidth);
                    setCoordValue('ocr-region-h', data.nameRegionHeight);
                    updateOCRRegionDisplay();
                }
            })
            .catch(() => {});
    }

    function updateOCRRegionDisplay() {
        const x = getCoordValue('ocr-region-x');
        const y = getCoordValue('ocr-region-y');
        const w = getCoordValue('ocr-region-w');
        const h = getCoordValue('ocr-region-h');
        const el = document.getElementById('ocr-region-display');
        if (el) {
            if (w > 0 && h > 0) {
                const posText = x === 0 ? '자동(오른쪽 위)' : `X:${x}`;
                el.textContent = `${posText}, Y:${y}, ${w}x${h}px`;
                el.style.color = 'var(--success-color)';
            } else {
                el.textContent = '미설정';
                el.style.color = 'var(--text-muted)';
            }
        }
    }

    function saveOCRConfig() {
        const cfg = {
            nameRegionX: getCoordValue('ocr-region-x'),
            nameRegionY: getCoordValue('ocr-region-y'),
            nameRegionWidth: getCoordValue('ocr-region-w'),
            nameRegionHeight: getCoordValue('ocr-region-h'),
            enabled: true
        };

        return fetch('/api/rotation/ocr/config', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(cfg)
        })
        .then(r => {
            if (r.ok) {
                addRotationLog('OCR 설정 저장됨');
                updateOCRRegionDisplay();
            }
        });
    }

    function checkOCRAvailability() {
        fetch('/api/rotation/ocr/check')
            .then(r => r.json())
            .then(data => {
                const el = document.getElementById('ocr-lang-status');
                if (el) {
                    if (data.available) {
                        el.textContent = '사용 가능';
                        el.style.color = 'var(--success-color)';
                    } else {
                        el.textContent = '한국어 언어 팩 미설치';
                        el.style.color = 'var(--danger-color)';
                    }
                }
            })
            .catch(() => {});
    }

    // === OCR 영역 선택 모달 ===

    async function openOCRRegionSelector() {
        const hwnd = getFirstAssignedHwnd();
        if (!hwnd) {
            addRotationLog('먼저 윈도우를 감지해주세요.');
            return;
        }

        const btn = document.getElementById('ocr-select-region-btn');
        if (btn) { btn.textContent = '스크린샷 촬영 중...'; btn.disabled = true; }

        try {
            const r = await fetch('/api/rotation/screenshot/full?hwnd=' + hwnd);
            const data = await r.json();
            if (!data.image) throw new Error('스크린샷 없음');

            ocrScreenshotData = data.image;
            ocrImageWidth = data.width;
            ocrImageHeight = data.height;

            showOCRRegionModal();
        } catch (e) {
            addRotationLog('스크린샷 촬영 실패: ' + e.message);
        } finally {
            if (btn) { btn.textContent = '스크린샷에서 영역 선택'; btn.disabled = false; }
        }
    }

    function showOCRRegionModal() {
        const modal = document.getElementById('ocr-region-modal');
        const canvas = document.getElementById('ocr-region-canvas');
        const wrap = document.getElementById('ocr-region-canvas-wrap');
        const selection = document.getElementById('ocr-region-selection');
        const coordsDisplay = document.getElementById('ocr-region-coords-display');
        const saveBtn = document.getElementById('ocr-region-save-btn');
        const cancelBtn = document.getElementById('ocr-region-cancel-btn');
        const closeBtn = document.getElementById('ocr-region-modal-close');

        if (!modal || !canvas) return;

        modal.style.display = 'flex';
        saveBtn.disabled = true;
        selection.style.display = 'none';
        coordsDisplay.textContent = '영역을 드래그하세요';

        const img = new Image();
        img.onload = function() {
            // 캔버스 크기를 wrap 너비에 맞춤
            const maxW = wrap.clientWidth - 4;
            const scale = maxW / img.width;
            const dispW = Math.floor(img.width * scale);
            const dispH = Math.floor(img.height * scale);

            canvas.width = dispW;
            canvas.height = dispH;
            wrap.style.height = dispH + 'px';

            const ctx = canvas.getContext('2d');
            ctx.drawImage(img, 0, 0, dispW, dispH);

            // 기존 설정 표시
            const curX = getCoordValue('ocr-region-x');
            const curY = getCoordValue('ocr-region-y');
            const curW = getCoordValue('ocr-region-w');
            const curH = getCoordValue('ocr-region-h');
            if (curW > 0 && curH > 0) {
                let drawX = curX === 0 ? (ocrImageWidth - curW - 10) : curX;
                showSelection(drawX * scale, curY * scale, curW * scale, curH * scale);
            }

            // 드래그 이벤트
            let dragging = false;
            let startX = 0, startY = 0;
            let selRect = { x: 0, y: 0, w: 0, h: 0 };

            function getCanvasPos(e) {
                const rect = canvas.getBoundingClientRect();
                return {
                    x: e.clientX - rect.left,
                    y: e.clientY - rect.top
                };
            }

            function showSelection(x, y, w, h) {
                selection.style.display = 'block';
                selection.style.left = x + 'px';
                selection.style.top = y + 'px';
                selection.style.width = w + 'px';
                selection.style.height = h + 'px';
            }

            canvas.onmousedown = function(e) {
                dragging = true;
                const pos = getCanvasPos(e);
                startX = pos.x;
                startY = pos.y;
                selection.style.display = 'none';
                e.preventDefault();
            };

            canvas.onmousemove = function(e) {
                if (!dragging) return;
                const pos = getCanvasPos(e);
                const x = Math.min(startX, pos.x);
                const y = Math.min(startY, pos.y);
                const w = Math.abs(pos.x - startX);
                const h = Math.abs(pos.y - startY);
                showSelection(x, y, w, h);
                selRect = { x, y, w, h };

                // 원본 좌표로 변환하여 표시
                const origX = Math.round(x / scale);
                const origY = Math.round(y / scale);
                const origW = Math.round(w / scale);
                const origH = Math.round(h / scale);
                coordsDisplay.textContent = `X:${origX}, Y:${origY}, ${origW}x${origH}px`;
            };

            canvas.onmouseup = function(e) {
                if (!dragging) return;
                dragging = false;
                if (selRect.w > 5 && selRect.h > 5) {
                    saveBtn.disabled = false;
                }
            };

            canvas.onmouseleave = function() {
                if (dragging) {
                    dragging = false;
                    if (selRect.w > 5 && selRect.h > 5) {
                        saveBtn.disabled = false;
                    }
                }
            };

            // 저장 버튼
            saveBtn.onclick = function() {
                const origX = Math.round(selRect.x / scale);
                const origY = Math.round(selRect.y / scale);
                const origW = Math.round(selRect.w / scale);
                const origH = Math.round(selRect.h / scale);

                setCoordValue('ocr-region-x', origX);
                setCoordValue('ocr-region-y', origY);
                setCoordValue('ocr-region-w', origW);
                setCoordValue('ocr-region-h', origH);

                saveOCRConfig();
                modal.style.display = 'none';
                canvas.onmousedown = null;
                canvas.onmousemove = null;
                canvas.onmouseup = null;
                canvas.onmouseleave = null;
                addRotationLog(`OCR 영역 설정: X:${origX}, Y:${origY}, ${origW}x${origH}px`);
            };

            // 취소/닫기
            function closeModal() {
                modal.style.display = 'none';
                canvas.onmousedown = null;
                canvas.onmousemove = null;
                canvas.onmouseup = null;
                canvas.onmouseleave = null;
            }
            cancelBtn.onclick = closeModal;
            closeBtn.onclick = closeModal;
        };
        img.src = ocrScreenshotData;
    }

    // OCR 테스트
    async function testOCR() {
        if (!detectedWindows || detectedWindows.length === 0) {
            addRotationLog('먼저 윈도우를 감지해주세요.');
            return;
        }

        const btn = document.getElementById('ocr-test-btn');
        const resultEl = document.getElementById('ocr-test-result');
        if (btn) { btn.textContent = 'OCR 인식 중...'; btn.disabled = true; }

        try {
            const r = await fetch('/api/rotation/detect-with-ocr');
            const data = await r.json();

            if (resultEl && data && data.length > 0) {
                resultEl.style.display = 'block';

                // 각 창의 크롭 이미지도 가져오기
                const cropPromises = data.map(w =>
                    fetch('/api/rotation/ocr/debug-crop?hwnd=' + w.hwnd)
                        .then(r => r.json())
                        .catch(() => null)
                );
                const crops = await Promise.all(cropPromises);

                resultEl.innerHTML = data.map((w, i) => {
                    let nameText = w.detectedName || '(인식 실패)';
                    let errorText = w.error ? `<div class="ocr-test-error">${escapeHtml(w.error)}</div>` : '';
                    let cropImg = crops[i] && crops[i].image
                        ? `<div class="ocr-crop-preview"><img src="${crops[i].image}" alt="크롭 영역" style="max-width:100%;height:auto;border:1px solid var(--border-color);margin-top:4px;image-rendering:pixelated;"></div>`
                        : '';
                    return `<div class="ocr-test-item">` +
                        `<span class="ocr-test-name">${escapeHtml(nameText)}</span>` +
                        (w.matchedName ? `<span class="ocr-badge ${w.confidence}">${escapeHtml(w.matchedName)}</span>` : '') +
                        `</div>` + errorText + cropImg;
                }).join('');
                addRotationLog('OCR 테스트 완료');
            }
        } catch (e) {
            addRotationLog('OCR 테스트 실패: ' + e.message);
        } finally {
            if (btn) { btn.textContent = 'OCR 테스트'; btn.disabled = false; }
        }
    }

})();
