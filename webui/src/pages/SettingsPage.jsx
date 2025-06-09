import { useState } from 'react'
import { useApp } from '@/contexts/AppContext'
import { telegramService } from '@services/telegramService'
import Card from '@components/Common/Card'
import Toggle from '@components/Common/Toggle'
import Button from '@components/Common/Button'
import TelegramSettings from '@components/Settings/TelegramSettings'
import { useNotification } from '@hooks/useNotification'
import styles from './SettingsPage.module.css'

export default function SettingsPage() {
    const { state, actions } = useApp()
    const { showNotification } = useNotification()

    const handleSettingChange = async (key, value) => {
        try {
            actions.updateSetting(key, value)

            // 서버에 설정 저장
            const result = await telegramService.saveSetting(key, value ? 1 : 0)
            if (result) {
                const settingNames = {
                    darkMode: '다크 모드',
                    autoStartup: '시작 시 자동 실행',
                    telegramEnabled: '텔레그램 알림'
                }

                actions.addLog(`${settingNames[key]}: ${value ? '켜짐' : '꺼짐'}`)
                showNotification(`설정이 저장되었습니다`, 'success')
            } else {
                // 실패 시 롤백
                actions.updateSetting(key, !value)
                showNotification('설정 저장에 실패했습니다', 'error')
            }
        } catch (error) {
            actions.updateSetting(key, !value)
            showNotification('설정 저장 중 오류가 발생했습니다', 'error')
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
                        />
                    </div>

                    <div className={styles.settingItem}>
                        <Toggle
                            checked={state.settings.autoStartup}
                            onChange={(checked) => handleSettingChange('autoStartup', checked)}
                            label="시작 시 자동 실행"
                        />
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
                    <div className={styles.aboutDescription}>
                        <p>React + Go 기반의 현대적인 게임 매크로 자동화 도구입니다.</p>
                        <p>효율적인 게임 플레이와 편리한 관리 기능을 제공합니다.</p>
                    </div>
                </div>
            </Card>
        </div>
    )
}