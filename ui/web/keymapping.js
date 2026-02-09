// 키 맵핑 시스템 JS
(function() {
    let mappings = []; // 플랫 배열로 변환하여 사용
    let availableKeys = []; // 시퀀스용 키 목록 (플랫)
    let statusInterval = null;

    const API = {
        getMappings: () => fetch('/api/keymapping/mappings').then(r => r.json()),
        createMapping: (data) => fetch('/api/keymapping/mappings', {
            method: 'POST', headers: {'Content-Type':'application/json'}, body: JSON.stringify(data)
        }).then(r => r.json()),
        updateMapping: (data) => fetch('/api/keymapping/mappings', {
            method: 'PUT', headers: {'Content-Type':'application/json'}, body: JSON.stringify(data)
        }).then(r => r.json()),
        deleteMapping: (id) => fetch('/api/keymapping/mappings?id=' + encodeURIComponent(id), { method: 'DELETE' }).then(r => r.json()),
        toggleMapping: (id) => fetch('/api/keymapping/toggle', {
            method: 'POST', headers: {'Content-Type':'application/json'}, body: JSON.stringify({id})
        }).then(r => r.json()),
        control: (action) => fetch('/api/keymapping/control', {
            method: 'POST', headers: {'Content-Type':'application/json'}, body: JSON.stringify({action})
        }).then(r => r.json()),
        getStatus: () => fetch('/api/keymapping/status').then(r => r.json()),
        getKeys: () => fetch('/api/keymapping/keys').then(r => r.json()),
    };

    // 초기화
    function init() {
        const section = document.getElementById('keymapping-section');
        if (!section) return;

        // 이벤트 리스너
        document.getElementById('km-system-start')?.addEventListener('click', () => {
            API.control('start').then(() => refreshStatus());
        });
        document.getElementById('km-system-stop')?.addEventListener('click', () => {
            API.control('stop').then(() => refreshStatus());
        });
        document.getElementById('km-add-btn')?.addEventListener('click', () => openModal());
        document.getElementById('km-modal-close')?.addEventListener('click', closeModal);
        document.getElementById('km-modal-cancel')?.addEventListener('click', closeModal);
        document.getElementById('km-modal-save')?.addEventListener('click', saveMapping);
        document.getElementById('km-add-key-btn')?.addEventListener('click', addKeyRow);

        // 모달 바깥 클릭 닫기
        document.getElementById('km-modal')?.addEventListener('click', (e) => {
            if (e.target.id === 'km-modal') closeModal();
        });

        loadKeys();
        refreshAll();

        // 상태 폴링
        if (statusInterval) clearInterval(statusInterval);
        statusInterval = setInterval(() => {
            const sec = document.getElementById('keymapping-section');
            if (sec && sec.classList.contains('active')) refreshStatus();
        }, 2000);
    }

    async function loadKeys() {
        try {
            const keysMap = await API.getKeys();
            // map[string][]string을 플랫 배열로 변환 (시작 키, 조합키 예시 제외)
            availableKeys = [];
            const skipCategories = ['시작 키', '조합키 예시'];
            for (const [category, keys] of Object.entries(keysMap)) {
                if (skipCategories.includes(category)) continue;
                for (const key of keys) {
                    if (!availableKeys.includes(key)) {
                        availableKeys.push(key);
                    }
                }
            }
        } catch(e) {
            availableKeys = ['1','2','3','4','5','6','7','8','9','0','a','b','c','d','e','f','g','h','i','j','k','l','m','n','o','p','q','r','s','t','u','v','w','x','y','z','f1','f2','f3','f4','f5','f6','f7','f8','f9','f10','f11','f12','enter','esc','space','tab'];
        }
    }

    async function refreshAll() {
        await refreshMappings();
        await refreshStatus();
    }

    async function refreshMappings() {
        try {
            const data = await API.getMappings();
            // map[string][]*KeyMapping을 플랫 배열로 변환
            mappings = [];
            if (data && typeof data === 'object') {
                for (const [startKey, list] of Object.entries(data)) {
                    if (Array.isArray(list)) {
                        for (const m of list) {
                            mappings.push(m);
                        }
                    }
                }
            }
            renderMappingList();
        } catch(e) {
            console.error('매핑 로드 실패:', e);
        }
    }

    async function refreshStatus() {
        try {
            const stats = await API.getStatus();
            const el = (id) => document.getElementById(id);
            const running = el('km-status-running');
            if (running) {
                running.textContent = stats.running ? '실행 중' : '중지됨';
                running.style.color = stats.running ? '#22c55e' : '#ef4444';
            }
            if (el('km-status-total')) el('km-status-total').textContent = (stats.total || 0) + '개';
            if (el('km-status-enabled')) el('km-status-enabled').textContent = (stats.enabled || 0) + '개';
            if (el('km-status-duplicates')) {
                el('km-status-duplicates').textContent = (stats.duplicate_keys || 0) + '개';
                el('km-status-duplicates').style.color = (stats.duplicate_keys || 0) > 0 ? '#f59e0b' : '';
            }
        } catch(e) {}
    }

    function renderMappingList() {
        const list = document.getElementById('km-mapping-list');
        const count = document.getElementById('km-count');
        if (!list) return;
        if (count) count.textContent = mappings.length;

        if (mappings.length === 0) {
            list.innerHTML = '<div class="empty-placeholder"><p>등록된 키 맵핑이 없습니다.</p><p style="font-size:0.8rem;opacity:0.6">새 맵핑을 추가해서 시작해보세요!</p></div>';
            return;
        }

        list.innerHTML = mappings.map(m => {
            const keysPreview = (m.keys || []).map(k =>
                k.delay > 0 ? `${k.key}(${k.delay}ms)` : k.key
            ).join(' → ');

            return `<div class="km-mapping-item ${m.enabled ? '' : 'disabled'}">
                <div class="km-mapping-info">
                    <div class="km-mapping-name">${escapeHtml(m.name || '(이름 없음)')}</div>
                    <div class="km-mapping-detail">
                        <span class="km-trigger-key">${escapeHtml(m.start_key)}</span>
                        <span class="km-arrow">→</span>
                        <span class="km-sequence-preview">${escapeHtml(keysPreview || '')}</span>
                    </div>
                </div>
                <div class="km-mapping-actions">
                    <label class="switch" style="margin-right:0.5rem">
                        <input type="checkbox" ${m.enabled ? 'checked' : ''} onchange="window._kmToggle('${m.id}')">
                        <span class="slider round"></span>
                    </label>
                    <button class="km-edit-btn" onclick="window._kmEdit('${m.id}')">수정</button>
                    <button class="km-delete-btn" onclick="window._kmDelete('${m.id}')">삭제</button>
                </div>
            </div>`;
        }).join('');
    }

    // 모달
    function openModal(editId) {
        const modal = document.getElementById('km-modal');
        if (!modal) return;

        document.getElementById('km-edit-id').value = '';
        document.getElementById('km-edit-name').value = '';
        document.getElementById('km-modal-title').textContent = '새 키 맵핑 추가';

        // 트리거 키 select 초기화
        const select = document.getElementById('km-edit-startkey');
        if (select) select.value = '';

        // 키 에디터 초기화
        const editor = document.getElementById('km-edit-keys');
        if (editor) editor.innerHTML = '';
        addKeyRow();

        // 수정 모드
        if (editId) {
            const m = mappings.find(x => x.id === editId);
            if (m) {
                document.getElementById('km-edit-id').value = m.id;
                document.getElementById('km-edit-name').value = m.name;
                document.getElementById('km-modal-title').textContent = '키 맵핑 수정';

                if (select) select.value = m.start_key;

                // 키 시퀀스 복원
                if (editor) editor.innerHTML = '';
                const keys = m.keys && m.keys.length > 0 ? m.keys : [];
                keys.forEach(k => addKeyRow(k.key, k.delay));
            }
        }

        updatePreview();
        modal.style.display = 'flex';
    }

    function closeModal() {
        const modal = document.getElementById('km-modal');
        if (modal) modal.style.display = 'none';
    }

    function isComboKey(key) {
        return key && (key.includes('ctrl+') || key.includes('shift+') || key.includes('alt+') || key.includes('cmd+') || key.includes('win+'));
    }

    function updateComboIndicator(row) {
        const input = row.querySelector('.km-key-input');
        let indicator = row.querySelector('.km-combo-indicator');
        const val = (input?.value || '').trim().toLowerCase();

        if (isComboKey(val)) {
            if (!indicator) {
                indicator = document.createElement('div');
                indicator.className = 'km-combo-indicator';
                indicator.innerHTML = '⌨ 조합키';
                input.parentElement.appendChild(indicator);
            }
            indicator.style.display = '';
        } else if (indicator) {
            indicator.style.display = 'none';
        }
    }

    function addKeyRow(key, delay) {
        const editor = document.getElementById('km-edit-keys');
        if (!editor) return;

        const row = document.createElement('div');
        row.className = 'km-key-row';
        const idx = editor.children.length + 1;

        row.innerHTML = `
            <span class="km-key-index">${idx}</span>
            <div class="km-key-input-wrap">
                <input type="text" class="km-key-input" value="${escapeHtml(key || '')}" placeholder="키 입력 (예: x, ctrl+c, alt+tab)">
            </div>
            <div class="km-delay-group">
                <input type="number" class="km-delay-input" value="${delay || 0}" min="0" max="10000" placeholder="딜레이(ms)">
                <span class="km-delay-unit">ms</span>
            </div>
            <button class="km-remove-key-btn" onclick="this.parentElement.remove();window._kmUpdatePreview()">&times;</button>
        `;

        const keyInput = row.querySelector('.km-key-input');
        keyInput.addEventListener('input', () => {
            updateComboIndicator(row);
            updatePreview();
        });
        row.querySelector('.km-delay-input').addEventListener('input', updatePreview);

        editor.appendChild(row);
        updateComboIndicator(row);
        updatePreview();
    }

    function updatePreview() {
        const preview = document.getElementById('km-preview');
        if (!preview) return;

        const rows = document.querySelectorAll('#km-edit-keys .km-key-row');
        const parts = [];
        rows.forEach(row => {
            const key = (row.querySelector('.km-key-input')?.value || '').trim().toLowerCase();
            const delay = parseInt(row.querySelector('.km-delay-input')?.value) || 0;
            if (key) {
                const comboClass = isComboKey(key) ? ' km-preview-combo' : '';
                parts.push(delay > 0
                    ? `<span class="km-preview-key${comboClass}">${escapeHtml(key)}</span><span class="km-preview-delay">${delay}ms</span>`
                    : `<span class="km-preview-key${comboClass}">${escapeHtml(key)}</span>`);
            }
        });

        preview.innerHTML = parts.length > 0 ? parts.join('<span class="km-preview-arrow">→</span>') : '<span style="opacity:0.5">키를 추가하세요</span>';
    }

    async function saveMapping() {
        const id = document.getElementById('km-edit-id')?.value;
        const name = document.getElementById('km-edit-name')?.value.trim();
        const startKey = document.getElementById('km-edit-startkey')?.value;

        if (!name) { alert('이름을 입력하세요'); return; }
        if (!startKey) { alert('트리거 키를 선택하세요'); return; }

        // 키 시퀀스 구성
        const rows = document.querySelectorAll('#km-edit-keys .km-key-row');
        const keys = [];
        rows.forEach(row => {
            const key = (row.querySelector('.km-key-input')?.value || '').trim().toLowerCase();
            const delay = parseInt(row.querySelector('.km-delay-input')?.value) || 0;
            if (key) keys.push({ key, delay });
        });

        if (keys.length === 0) { alert('최소 하나의 키를 추가하세요'); return; }

        try {
            if (id) {
                await API.updateMapping({ id, name, start_key: startKey, keys });
            } else {
                await API.createMapping({ name, start_key: startKey, keys });
            }
            closeModal();
            refreshAll();
        } catch(e) {
            alert('저장 실패: ' + e.message);
        }
    }

    function escapeHtml(str) {
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }

    // 전역 함수 노출 (인라인 이벤트용)
    window._kmToggle = async (id) => {
        await API.toggleMapping(id);
        refreshAll();
    };
    window._kmEdit = (id) => openModal(id);
    window._kmDelete = async (id) => {
        if (!confirm('이 맵핑을 삭제하시겠습니까?')) return;
        await API.deleteMapping(id);
        refreshAll();
    };
    window._kmUpdatePreview = updatePreview;

    // 탭 활성화 감지
    const observer = new MutationObserver(() => {
        const sec = document.getElementById('keymapping-section');
        if (sec && sec.classList.contains('active')) {
            refreshAll();
        }
    });
    const kmSec = document.getElementById('keymapping-section');
    if (kmSec) observer.observe(kmSec, { attributes: true, attributeFilter: ['class'] });

    // DOM 로드 시 초기화
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();
