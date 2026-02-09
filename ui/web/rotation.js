// 순환 사냥 프론트엔드 로직

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
    });

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
        characterForm.style.display = 'block';
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

        if (!name) { alert('캐릭터 이름을 입력해주세요.'); return; }
        if (!area) { alert('사냥터 이름을 입력해주세요.'); return; }

        const profile = {
            name: name,
            huntingArea: { name: area, dropdownIndex: dropdownIndex },
            durationMins: duration,
            order: characters.length,
            enabled: true
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
        characterForm.style.display = 'block';
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

    function detectWindows() {
        detectWindowsBtn.textContent = '감지 중...';
        detectWindowsBtn.disabled = true;

        fetch('/api/rotation/windows')
            .then(r => r.json())
            .then(data => {
                detectedWindows = data || [];
                screenshotCache = {}; // 새 감지이므로 캐시 초기화
                renderWindows();
                addRotationLog(`${detectedWindows.length}개의 게임 창 감지됨`);

                // 캐릭터가 있으면 자동 할당
                if (characters.length > 0 && detectedWindows.length > 0) {
                    autoAssign();
                    addRotationLog('캐릭터를 감지된 창에 자동 할당했습니다.');
                }

                // 스크린샷 순차 로드
                loadScreenshotsSequential(0);
            })
            .catch(() => {
                windowList.innerHTML = '<p class="empty-placeholder">창 감지에 실패했습니다.</p>';
            })
            .finally(() => {
                detectWindowsBtn.textContent = '감지';
                detectWindowsBtn.disabled = false;
            });
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

            return `
                <div class="window-item-card ${isExcluded ? 'excluded' : ''}" data-hwnd="${w.hwnd}">
                    <div class="window-item-header">
                        <label class="window-toggle">
                            <input type="checkbox" ${!isExcluded ? 'checked' : ''} onchange="rotationToggleWindow(${w.hwnd}, this.checked)">
                        </label>
                        <div class="window-order">${idx + 1}</div>
                        <div class="window-info">
                            <div class="window-title">${escapeHtml(w.title)}</div>
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

        const swordDisp = document.getElementById('coord-sword-display');
        const dropdownDisp = document.getElementById('coord-dropdown-display');
        const startDisp = document.getElementById('coord-start-display');
        const firstItemDisp = document.getElementById('coord-first-item-display');
        const itemHeightDisp = document.getElementById('coord-item-height-display');
        const confirmDisp = document.getElementById('coord-confirm-display');

        if (swordDisp) swordDisp.textContent = (sx || sy) ? `(${sx}, ${sy})` : '미설정';
        if (dropdownDisp) dropdownDisp.textContent = (dx || dy) ? `(${dx}, ${dy})` : '미설정';
        if (startDisp) startDisp.textContent = (bx || by) ? `(${bx}, ${by})` : '미설정';
        if (firstItemDisp) firstItemDisp.textContent = fiy > 0 ? `Y: ${fiy}` : '미설정';
        if (itemHeightDisp) itemHeightDisp.textContent = ih > 0 ? `${ih}px` : '미설정';
        if (confirmDisp) confirmDisp.textContent = (cx || cy) ? `(${cx}, ${cy})` : '미설정';
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
            confirmButtonY: getCoordValue('coord-confirm-y')
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
        document.querySelectorAll('.capture-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                const target = btn.dataset.target;
                const msg = btn.dataset.msg;
                if (target && msg) startCapture(target, msg);
            });
        });
    }

    function getFirstAssignedHwnd() {
        for (const charId in windowAssignments) {
            if (windowAssignments[charId]) return windowAssignments[charId];
        }
        if (detectedWindows.length > 0) return detectedWindows[0].hwnd;
        return 0;
    }

    function startCapture(target, message) {
        const hwnd = getFirstAssignedHwnd();
        if (!hwnd) {
            addRotationLog('먼저 윈도우를 감지해주세요.');
            return;
        }

        const overlay = document.getElementById('capture-overlay');
        const countdownEl = document.getElementById('capture-countdown');
        const messageEl = document.getElementById('capture-message');

        if (!overlay) return;

        overlay.style.display = 'flex';
        messageEl.textContent = message;

        let count = 3;
        countdownEl.textContent = count;

        const interval = setInterval(() => {
            count--;
            if (count > 0) {
                countdownEl.textContent = count;
            } else {
                clearInterval(interval);
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
                    } else if (target === 'firstitem') {
                        setCoordValue('coord-first-y', data.y);
                    } else if (target === 'seconditem') {
                        // 두 번째 항목의 Y에서 첫 번째 항목 Y를 빼서 항목 높이 계산
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
                    // 캡처할 때마다 자동으로 서버에 좌표 저장
                    saveCoordinates();
                    overlay.style.display = 'none';
                })
                .catch(err => {
                    addRotationLog('좌표 캡처 실패: ' + err.message);
                    overlay.style.display = 'none';
                });
            }
        }, 1000);
    }

    // 초기화 시 캡처 버튼도 설정
    document.addEventListener('DOMContentLoaded', () => {
        setTimeout(setupCaptureBtns, 100);
    });

    // === 순환 시작/중지 ===

    function startRotation() {
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
                    addRotationLog('순환 사냥 시작!');
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
                    addRotationLog('순환 사냥 중지');
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

    // === 순환 로그 ===

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
                addRotationLog('순환 사냥 완료!');
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

})();
