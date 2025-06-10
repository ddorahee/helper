// webui/src/pages/SettingsPage.jsx 수정
import { useState } from 'react'
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

    const handleSettingChange = async (key, value) => {
        setSaving(true)

        try {
            console.log('설정 변경 요청:', { key, value })

            // 먼저 클라이언트 상태 업데이트
            actions.updateSetting(key, value)

            // 서버에 설정 저장
            const settingKeyMap = {
                darkMode: 'dark_mode',
                autoStartup: 'auto_startup',
                telegramEnabled: 'telegram_enabled'
            }

            const apiKey = settingKeyMap[key] || key
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
                actions.updateSetting(key, !value)
                showNotification('설정 저장에 실패했습니다', 'error')
                actions.addLog(`설정 저장 실패: ${key}`)

                console.error('설정 저장 실패:', { key, value, apiKey })
            }
        } catch (error) {
            console.error('설정 저장 중 오류:', error)

            // 실패 시 롤백
            actions.updateSetting(key, !value)
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
                    </div>

                    <div className={styles.settingItem}>
                        <Toggle
                            checked={state.settings.autoStartup}
                            onChange={(checked) => handleSettingChange('autoStartup', checked)}
                            label="시작 시 자동 실행"
                            disabled={saving}
                        />
                    </div>

                    {/* 디버그용 저장 버튼 */}
                    <div className={styles.settingItem}>
                        <Button
                            variant="outline"
                            size="sm"
                            onClick={handleSaveAllSettings}
                            loading={saving}
                        >
                            모든 설정 저장
                        </Button>
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
                    <div className={styles.aboutDescription}>
                        <p>React + Go 기반의 현대적인 게임 매크로 자동화 도구입니다.</p>
                        <p>효율적인 게임 플레이와 편리한 관리 기능을 제공합니다.</p>

                        <div className={styles.debugInfo}>
                            <h4>현재 설정 상태:</h4>
                            <ul>
                                <li>다크 모드: {state.settings.darkMode ? '켜짐' : '꺼짐'}</li>
                                <li>자동 시작: {state.settings.autoStartup ? '켜짐' : '꺼짐'}</li>
                                <li>텔레그램: {state.settings.telegramEnabled ? '켜짐' : '꺼짐'}</li>
                            </ul>
                        </div>
                    </div>
                </div>
            </Card>
        </div>
    )
}