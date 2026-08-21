// 창 전환 — 슬롯 5개 카드 렌더 + 등록/비우기 버튼 + 핫키 이벤트 실시간 반영
(function () {
    const SLOT_COUNT = 5;
    let slots = [];

    function el(id) { return document.getElementById(id); }

    async function api(path, body) {
        const opt = body
            ? { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }
            : {};
        const res = await fetch(path, opt);
        if (!res.ok) throw new Error(await res.text());
        return res.json();
    }

    function render() {
        const wrap = el('switcher-slots');
        if (!wrap) return;
        // 텍스트 입력 중이면 재렌더 보류(입력 날아감 방지) — blur 시 다시 그려짐
        const ae = document.activeElement;
        if (ae && ae.classList && ae.classList.contains('slot-clip')) return;
        wrap.innerHTML = '';
        for (let i = 1; i <= SLOT_COUNT; i++) {
            const s = slots.find(x => x.slot === i) || { slot: i, hwnd: 0, title: '', valid: false };
            const filled = s.hwnd && s.hwnd !== 0;
            const dead = filled && !s.valid;

            const card = document.createElement('div');
            card.className = 'switcher-slot' + (filled ? ' filled' : '') + (dead ? ' dead' : '');

            const title = filled
                ? (s.title && s.title.trim() ? s.title : '(제목 없음)') + (dead ? ' — 창 닫힘' : '')
                : '비어 있음';
            const hotkey = s.hotkey || ('슬롯' + i);

            card.innerHTML =
                '<div class="slot-num">' + escapeHtml(hotkey) + '<span class="slot-idx">슬롯 ' + i + '</span></div>' +
                '<div class="slot-title" title="' + escapeHtml(title) + '">' + escapeHtml(title) + '</div>' +
                '<div class="slot-actions">' +
                '  <button class="slot-register" data-slot="' + i + '">현재 창 등록</button>' +
                (filled ? '  <button class="slot-clear" data-slot="' + i + '">비우기</button>' : '') +
                '</div>' +
                '<label class="slot-cliplabel">전환 시 복사할 텍스트</label>' +
                '<textarea class="slot-clip" data-slot="' + i + '" rows="2" placeholder="여기 적어두면 이 창으로 전환할 때 클립보드에 복사됩니다 (붙여넣기는 직접)">' +
                escapeHtml(s.clipText || '') + '</textarea>';
            wrap.appendChild(card);
        }

        wrap.querySelectorAll('.slot-register').forEach(b => {
            b.addEventListener('click', () => startDelayedRegister(parseInt(b.dataset.slot), b));
        });
        wrap.querySelectorAll('.slot-clear').forEach(b => {
            b.addEventListener('click', async () => {
                try { slots = await api('/api/switcher/clear', { slot: parseInt(b.dataset.slot) }); render(); }
                catch (e) { logLine('비우기 실패: ' + e.message); }
            });
        });
        // 텍스트는 포커스 잃을 때 저장 (이벤트로 슬롯이 다시 그려져도 입력 중엔 안 건드림)
        wrap.querySelectorAll('.slot-clip').forEach(t => {
            t.addEventListener('blur', async () => {
                try { await api('/api/switcher/text', { slot: parseInt(t.dataset.slot), text: t.value }); }
                catch (e) { logLine('텍스트 저장 실패: ' + e.message); }
            });
        });
    }

    // 3초 지연 등록: helper 창이 맨 앞이면 자기 자신이 잡히므로, 카운트다운 동안 대상 창을 클릭.
    let registering = false;
    async function startDelayedRegister(slot, btn) {
        if (registering) return;
        registering = true;
        const DELAY = 3;
        logLine('슬롯 ' + slot + ' 등록: ' + DELAY + '초 안에 기억할 창을 클릭하세요…');
        let left = DELAY;
        const origText = btn.textContent;
        btn.textContent = left + '초…';
        const iv = setInterval(() => {
            left--;
            btn.textContent = left > 0 ? left + '초…' : '등록 중';
            if (left <= 0) clearInterval(iv);
        }, 1000);
        try {
            await api('/api/switcher/register', { slot: slot, delaySec: DELAY });
            // 실제 등록 결과는 switcherSlots 이벤트로 들어와 render() 됨
        } catch (e) {
            logLine('등록 실패: ' + e.message);
        }
        setTimeout(() => { registering = false; btn.textContent = origText; refresh(); }, (DELAY + 1) * 1000);
    }

    function logLine(msg) {
        const box = el('switcher-log');
        if (!box) return;
        const time = new Date().toLocaleTimeString('ko-KR', { hour12: false });
        const div = document.createElement('div');
        div.textContent = '[' + time + '] ' + msg;
        box.appendChild(div);
        while (box.childElementCount > 50) box.removeChild(box.firstChild);
        box.scrollTop = box.scrollHeight;
    }

    function escapeHtml(s) {
        return String(s).replace(/[&<>"']/g, c =>
            ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
    }

    async function refresh() {
        try { slots = await api('/api/switcher/slots'); render(); } catch (e) { /* 서버 준비 전 무시 */ }
    }

    // 핫키로 등록/이동/실패 시 서버가 보내는 이벤트 (script.js의 dispatchAppEvent가 위임)
    window.onSwitcherSlots = function (payload) { slots = payload || []; render(); };
    window.onSwitcherLog = function (msg) { logLine(msg); };

    document.addEventListener('DOMContentLoaded', () => {
        render();
        refresh();
        // 창 전환 탭을 열 때 최신 상태 반영(창이 닫혔을 수 있으니)
        document.querySelectorAll('.nav-button[data-section="switcher"]').forEach(btn => {
            btn.addEventListener('click', refresh);
        });
    });
})();
