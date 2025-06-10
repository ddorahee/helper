// webui/src/pages/SettingsPage.jsx 완전 수정
import { useState, useEffect } from 'react'
import { useApp } from '@/contexts/AppContext'
import { appService } from '@services/appService'
import Card from '@components/Common/Card'
import Toggle from '@components/Common/Toggle'
import Button from '@components/Common/Button'
import TelegramSettings from '@components/Settings/TelegramSettings'
import { useNotification } from '@hooks/useNotification'
import styles from './SettingsPage.module.css'

export default function SettingsPage() {
    const { state, actions } = useApp()
    const { showNotification } = useNotification()
    const [saving, setSaving] = useState(false)
    const [loading, setLoading] = useState(true)

    // 컴포넌트 마운트 시 설정 로드
    useEffect(() => {
        loadSettings()
    }, [])

    // 설정 로드
    const loadSettings = async () => {
        try {
            console.log('설정 로드 시작...')
            setLoading(true)

            const settings = await appService.loadSettings()
            console.log('로드된 설정:', settings)

            if (settings) {
                actions.setSettings(settings)
                actions.addLog('설정이 로드되었습니다.')
            }
        } catch (error) {
            console.error('설정 로드 오류:', error)
            actions.addLog('설정 로드 중 오류가 발생했습니다.')
            showNotification('설정 로드에 실패했습니다', 'error')
        } finally {
            setLoading(false)
        }
    }

    const handleSettingChange = async (key, value) => {
        if (saving) {
            console.log('이미 저장 중이므로 무시:', key, value)
            return
        }

        setSaving(true)

        try {
            console.log('설정 변경 요청:', { key, value })

            // 먼저 클라이언트 상태 업데이트
            const oldValue = state.settings[key]
            actions.updateSetting(key, value)

            // 서버에 설정 저장
            const settingKeyMap = {
                darkMode: 'dark_mode',
                autoStartup: 'auto_startup',
                telegramEnabled: 'telegram_enabled'
            }

            const apiKey = settingKeyMap[key] || key
            console.log('API 키 매핑:', key, '->', apiKey)

            const result = await appService.saveSetting(apiKey, value)

            if (result) {
                const settingNames = {
                    darkMode: '다크 모드',
                    autoStartup: '시작 시 자동 실행',
                    telegramEnabled: '텔레그램 알림'
                }

                const settingName = settingNames[key] || key
                const statusText = value ? '켜짐' : '꺼짐'

                actions.addLog(`${settingName}: ${statusText}`)
                showNotification(`${settingName} 설정이 저장되었습니다`, 'success')

                console.log('설정 저장 성공:', { key, value, apiKey })
            } else {
                // 실패 시 롤백
                console.error('설정 저장 실패, 롤백:', { key, oldValue })
                actions.updateSetting(key, oldValue)
                showNotification('설정 저장에 실패했습니다', 'error')
                actions.addLog(`설정 저장 실패: ${key}`)
            }
        } catch (error) {
            console.error('설정 저장 중 오류:', error)

            // 실패 시 롤백
            const oldValue = !value // 토글이므로 반대값으로 롤백
            actions.updateSetting(key, oldValue)
            showNotification('설정 저장 중 오류가 발생했습니다', 'error')
            actions.addLog(`설정 저장 오류: ${error.message}`)
        } finally {
            setSaving(false)
        }
    }

    // 모든 설정 저장 (테스트용)
    const handleSaveAllSettings = async () => {
        setSaving(true)

        try {
            console.log('전체 설정 저장 시작:', state.settings)
            const success = await appService.saveAllSettings(state.settings)

            if (success) {
                showNotification('모든 설정이 저장되었습니다', 'success')
                actions.addLog('모든 설정 저장 완료')
            } else {
                showNotification('일부 설정 저장에 실패했습니다', 'warning')
                actions.addLog('일부 설정 저장 실패')
            }
        } catch (error) {
            console.error('전체 설정 저장 오류:', error)
            showNotification('설정 저장 중 오류가 발생했습니다', 'error')
            actions.addLog(`전체 설정 저장 오류: ${error.message}`)
        } finally {
            setSaving(false)
        }
    }

    // 설정 새로고침
    const handleRefreshSettings = async () => {
        await loadSettings()
        showNotification('설정을 새로고침했습니다', 'info')
    }

    if (loading) {
        return (
            <div className={styles.settingsPage}>
                <Card title="설정 로딩 중...">
                    <div style={{ textAlign: 'center', padding: '2rem' }}>
                        설정을 불러오는 중입니다...
                    </div>
                </Card>
            </div>
        )
    }

    return (
        <div className={styles.settingsPage}>
            {/* 애플리케이션 설정 */}
            <Card title="애플리케이션 설정">
                <div className={styles.settingsForm}>
                    <div className={styles.settingItem}>
                        <Toggle
                            checked={state.settings.darkMode}
                            onChange={(checked) => handleSettingChange('darkMode', checked)}
                            label="다크 모드"
                            disabled={saving}
                        />
                        {saving && <span className={styles.savingIndicator}>저장 중...</span>}
                    </div>

                    <div className={styles.settingItem}>
                        <Toggle
                            checked={state.settings.autoStartup}
                            onChange={(checked) => handleSettingChange('autoStartup', checked)}
                            label="시작 시 자동 실행"
                            disabled={saving}
                        />
                    </div>

                    {/* 디버그 버튼들 */}
                    <div className={styles.settingItem}>
                        <div className={styles.debugButtons}>
                            <Button
                                variant="outline"
                                size="sm"
                                onClick={handleSaveAllSettings}
                                loading={saving}
                            >
                                모든 설정 저장
                            </Button>
                            <Button
                                variant="secondary"
                                size="sm"
                                onClick={handleRefreshSettings}
                                disabled={saving}
                            >
                                설정 새로고침
                            </Button>
                        </div>
                    </div>
                </div>
            </Card>

            {/* 텔레그램 설정 */}
            <TelegramSettings />

            {/* 프로그램 정보 */}
            <Card title="프로그램 정보">
                <div className={styles.aboutContent}>
                    <p><strong>도우미</strong></p>
                    <p>버전: <span>{state.version}</span></p>
                    <p>제작: minggi</p>
                    <p>빌드 날짜: <span>{state.buildDate}</span></p>

                    {/* 현재 설정 상태 표시 (디버그용) */}
                    <div className={styles.debugInfo}>
                        <h4>현재 설정 상태:</h4>
                        <ul>
                            <li>다크 모드: {state.settings.darkMode ? '켜짐' : '꺼짐'}</li>
                            <li>자동 시작: {state.settings.autoStartup ? '켜짐' : '꺼짐'}</li>
                            <li>텔레그램: {state.settings.telegramEnabled ? '켜짐' : '꺼짐'}</li>
                        </ul>
                    </div>

                    <div className={styles.aboutDescription}>
                        <p>React + Go 기반의 현대적인 게임 매크로 자동화 도구입니다.</p>
                        <p>효율적인 게임 플레이와 편리한 관리 기능을 제공합니다.</p>
                    </div>
                </div>
            </Card>
        </div>
    )
}