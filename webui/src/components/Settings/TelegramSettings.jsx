import { useState, useEffect } from 'react'
import { Send, Save } from 'lucide-react'
import { useApp } from '@/contexts/AppContext'
import { telegramService } from '@services/telegramService'
import { useNotification } from '@hooks/useNotification'
import Card from '@components/Common/Card'
import Toggle from '@components/Common/Toggle'
import Button from '@components/Common/Button'
import styles from './TelegramSettings.module.css'

export default function TelegramSettings() {
    const { state, actions } = useApp()
    const { showNotification } = useNotification()
    const [token, setToken] = useState('')
    const [chatId, setChatId] = useState('')
    const [loading, setLoading] = useState(false)

    useEffect(() => {
        loadTelegramStatus()
    }, [])

    const loadTelegramStatus = async () => {
        try {
            const status = await telegramService.getStatus()
            if (status.enabled) {
                actions.updateSetting('telegramEnabled', true)
            }
        } catch (error) {
            console.error('Failed to load telegram status:', error)
        }
    }

    const handleTelegramToggle = async (enabled) => {
        actions.updateSetting('telegramEnabled', enabled)

        if (!enabled) {
            await telegramService.saveSetting('telegram_enabled', 0)
            actions.addLog('텔레그램 알림: 꺼짐')
        } else {
            actions.addLog('텔레그램 알림: 켜짐')
        }
    }

    const handleSave = async () => {
        if (!token.trim() || !chatId.trim()) {
            showNotification('봇 토큰과 채팅 ID를 모두 입력해주세요.', 'error')
            return
        }

        setLoading(true)

        try {
            const success = await telegramService.saveConfig(token.trim(), chatId.trim())

            if (success) {
                showNotification('텔레그램 설정이 저장되었습니다! 🎉', 'success')
                actions.addLog('텔레그램 설정이 저장되었습니다.')
                actions.updateSetting('telegramEnabled', true)

                // 입력 필드 초기화
                setToken('')
                setChatId('')
            } else {
                showNotification('설정 저장에 실패했습니다. 다시 시도해주세요.', 'error')
                actions.addLog('텔레그램 설정 저장 실패')
            }
        } catch (error) {
            showNotification('설정 저장 중 오류가 발생했습니다.', 'error')
            actions.addLog('텔레그램 설정 저장 중 오류 발생')
        } finally {
            setLoading(false)
        }
    }

    const handleTest = async () => {
        if (!state.settings.telegramEnabled) {
            showNotification('먼저 텔레그램 설정을 저장해주세요.', 'warning')
            return
        }

        setLoading(true)

        try {
            const success = await telegramService.testConnection()

            if (success) {
                showNotification('테스트 메시지가 전송되었습니다! 📱', 'success')
                actions.addLog('텔레그램 테스트 메시지 전송 완료')
            } else {
                showNotification('테스트에 실패했습니다. 설정을 확인해주세요.', 'error')
                actions.addLog('텔레그램 테스트 실패')
            }
        } catch (error) {
            showNotification('테스트 중 오류가 발생했습니다.', 'error')
            actions.addLog('텔레그램 테스트 중 오류 발생')
        } finally {
            setLoading(false)
        }
    }

    return (
        <Card title="📱 텔레그램 알림 설정">
            <div className={styles.telegramForm}>
                <div className={styles.settingItem}>
                    <Toggle
                        checked={state.settings.telegramEnabled}
                        onChange={handleTelegramToggle}
                        label="텔레그램 알림 사용"
                    />
                </div>

                {state.settings.telegramEnabled && (
                    <div className={styles.telegramConfig}>
                        <div className={styles.formGroup}>
                            <label htmlFor="bot-token" className={styles.label}>
                                봇 토큰:
                            </label>
                            <input
                                type="password"
                                id="bot-token"
                                value={token}
                                onChange={(e) => setToken(e.target.value)}
                                placeholder="7903573798:AAGTXj_rk2xMygTee06OVVYlQhxqm95H2CU"
                                className={styles.input}
                            />
                            <small className={styles.formHelp}>
                                @BotFather에서 받은 봇 토큰을 입력하세요
                            </small>
                        </div>

                        <div className={styles.formGroup}>
                            <label htmlFor="chat-id" className={styles.label}>
                                채팅 ID:
                            </label>
                            <input
                                type="text"
                                id="chat-id"
                                value={chatId}
                                onChange={(e) => setChatId(e.target.value)}
                                placeholder="123456789"
                                className={styles.input}
                            />
                            <small className={styles.formHelp}>
                                개인 채팅 ID 또는 그룹 채팅 ID를 입력하세요
                            </small>
                        </div>

                        <div className={styles.telegramActions}>
                            <Button
                                variant="success"
                                onClick={handleSave}
                                loading={loading}
                                icon={<Save size={16} />}
                                disabled={!token.trim() || !chatId.trim()}
                            >
                                저장
                            </Button>

                            <Button
                                variant="primary"
                                onClick={handleTest}
                                loading={loading}
                                icon={<Send size={16} />}
                                disabled={!state.settings.telegramEnabled}
                            >
                                테스트
                            </Button>
                        </div>
                    </div>
                )}

                <div className={styles.telegramHelp}>
                    <h4>🤖 텔레그램 봇 설정 방법:</h4>
                    <ol>
                        <li>텔레그램에서 <code>@BotFather</code>를 검색하여 대화를 시작합니다.</li>
                        <li><code>/newbot</code> 명령을 입력하여 새 봇을 생성합니다.</li>
                        <li>봇 이름과 사용자명을 설정합니다.</li>
                        <li>받은 토큰을 위의 "봇 토큰" 필드에 입력합니다.</li>
                        <li><code>@userinfobot</code>에게 메시지를 보내 채팅 ID를 확인합니다.</li>
                        <li>채팅 ID를 위의 "채팅 ID" 필드에 입력합니다.</li>
                        <li>"저장" 버튼을 클릭한 후 "테스트" 버튼으로 연결을 확인합니다.</li>
                    </ol>
                </div>
            </div>
        </Card>
    )
}