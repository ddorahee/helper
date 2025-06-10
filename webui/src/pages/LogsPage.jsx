import { useState, useEffect, useRef } from 'react'
import { RefreshCw, Trash2, Filter } from 'lucide-react'
import { useApp } from '@/contexts/AppContext'
import { appService } from '@services/appService'
import Card from '@components/Common/Card'
import Button from '@components/Common/Button'
import Toggle from '@components/Common/Toggle'
import styles from './LogsPage.module.css'

export default function LogsPage() {
    const { actions } = useApp()
    const [logs, setLogs] = useState([])
    const [loading, setLoading] = useState(false)
    const [autoRefresh, setAutoRefresh] = useState(false)
    const [showDebug, setShowDebug] = useState(false)
    const [filter, setFilter] = useState('')

    // 가상화 스크롤을 위한 상태
    const [scrollTop, setScrollTop] = useState(0)
    const [containerHeight, setContainerHeight] = useState(400)
    const containerRef = useRef(null)
    const loadingRef = useRef(false)
    const intervalRef = useRef(null)

    // 가상화 설정
    const ITEM_HEIGHT = 60 // 각 로그 항목의 고정 높이
    const BUFFER_SIZE = 5 // 버퍼 항목 수

    // 로그 불러오기 함수
    const loadLogs = async () => {
        if (loadingRef.current) {
            console.log('이미 로딩 중이므로 요청 무시')
            return
        }

        loadingRef.current = true
        setLoading(true)

        try {
            console.log('로그 데이터 요청 시작...')
            const data = await appService.getLogs()
            console.log('받은 로그 데이터:', data)

            if (data && data.logs && Array.isArray(data.logs)) {
                // 로그 개수 제한 (최대 1000개)
                const limitedLogs = data.logs.slice(-1000)
                setLogs(limitedLogs)
                console.log('로그 설정 완료, 개수:', limitedLogs.length)
            } else {
                console.warn('로그 데이터 형식이 올바르지 않음:', data)
                setLogs([
                    '로그 데이터를 불러올 수 없습니다.',
                    '서버 응답 형식이 올바르지 않습니다.',
                    JSON.stringify(data).slice(0, 100) + '...'
                ])
            }
        } catch (error) {
            console.error('로그 로딩 오류:', error)
            actions.addLog('로그를 불러오는 중 오류가 발생했습니다.')
            setLogs([
                '로그를 불러오는 중 오류가 발생했습니다.',
                `오류 내용: ${error.message || error}`,
                '네트워크 연결을 확인하고 다시 시도해주세요.'
            ])
        } finally {
            setLoading(false)
            loadingRef.current = false
        }
    }

    // 로그 지우기
    const clearLogs = async () => {
        if (!confirm('모든 로그를 삭제하시겠습니까?')) return

        try {
            const success = await appService.clearLogs()
            if (success) {
                setLogs([])
                actions.addLog('로그가 삭제되었습니다.')
            } else {
                actions.addLog('로그 삭제에 실패했습니다.')
            }
        } catch (error) {
            console.error('로그 삭제 오류:', error)
            actions.addLog('로그 삭제 중 오류가 발생했습니다.')
        }
    }

    // 수동 새로고침
    const handleRefresh = () => {
        console.log('수동 새로고침 요청')
        loadLogs()
    }

    // 자동 새로고침 토글
    const handleAutoRefreshToggle = (enabled) => {
        console.log('자동 새로고침 토글:', enabled)
        setAutoRefresh(enabled)

        if (intervalRef.current) {
            clearInterval(intervalRef.current)
            intervalRef.current = null
        }

        if (enabled) {
            intervalRef.current = setInterval(() => {
                console.log('자동 새로고침 실행')
                loadLogs()
            }, 30000)
        }
    }

    // 컴포넌트 마운트 시 초기 로드
    useEffect(() => {
        console.log('LogsPage 마운트됨')
        loadLogs()

        // 컨테이너 높이 설정
        if (containerRef.current) {
            setContainerHeight(containerRef.current.clientHeight)
        }

        return () => {
            if (intervalRef.current) {
                clearInterval(intervalRef.current)
                intervalRef.current = null
            }
        }
    }, [])

    // 스크롤 핸들러
    const handleScroll = (e) => {
        setScrollTop(e.target.scrollTop)
    }

    // 디버그 로그 판단 함수
    const isDebugLog = (log) => {
        if (typeof log !== 'string') return false
        const lowerLog = log.toLowerCase()
        return lowerLog.includes('debug') ||
            lowerLog.includes('초기화') ||
            lowerLog.includes('설정') ||
            lowerLog.includes('디버그')
    }

    // 로그 레벨 판단 함수
    const getLogLevel = (log) => {
        if (typeof log !== 'string') return 'info'
        const lowerLog = log.toLowerCase()
        if (lowerLog.includes('error') || lowerLog.includes('오류') || lowerLog.includes('실패')) {
            return 'error'
        } else if (lowerLog.includes('warn') || lowerLog.includes('경고')) {
            return 'warning'
        } else if (isDebugLog(log)) {
            return 'debug'
        }
        return 'info'
    }

    // 로그 텍스트 자르기 함수
    const truncateLog = (log, maxLength = 200) => {
        if (!log || typeof log !== 'string') return ''
        if (log.length <= maxLength) return log
        return log.slice(0, maxLength) + '...'
    }

    // 로그 필터링
    const filteredLogs = logs.filter(log => {
        if (typeof log !== 'string') return false
        if (!showDebug && isDebugLog(log)) return false
        if (filter && !log.toLowerCase().includes(filter.toLowerCase())) return false
        return true
    })

    // 가상화 계산
    const totalHeight = filteredLogs.length * ITEM_HEIGHT
    const startIndex = Math.max(0, Math.floor(scrollTop / ITEM_HEIGHT) - BUFFER_SIZE)
    const endIndex = Math.min(
        filteredLogs.length,
        Math.ceil((scrollTop + containerHeight) / ITEM_HEIGHT) + BUFFER_SIZE
    )
    const visibleLogs = filteredLogs.slice(startIndex, endIndex)

    return (
        <div className={styles.logsPage}>
            <Card
                title="로그 기록"
                className={styles.logsCard}
                headerContent={
                    <div className={styles.logsActions}>
                        <Button
                            variant="outline"
                            size="sm"
                            onClick={handleRefresh}
                            disabled={loading}
                            icon={<RefreshCw size={16} />}
                        >
                            새로고침
                        </Button>
                        <Button
                            variant="outline"
                            size="sm"
                            onClick={clearLogs}
                            icon={<Trash2 size={16} />}
                            className={styles.dangerButton}
                        >
                            지우기
                        </Button>
                    </div>
                }
            >
                <div className={styles.logsContent}>
                    <div
                        ref={containerRef}
                        className={styles.logsContainer}
                        onScroll={handleScroll}
                    >
                        {loading ? (
                            <div className={styles.logPlaceholder}>로그를 불러오는 중...</div>
                        ) : filteredLogs.length === 0 ? (
                            <div className={styles.logPlaceholder}>
                                표시할 로그가 없습니다.
                                <br />
                                <small>프로그램 활동이 시작되면 여기에 로그가 표시됩니다.</small>
                            </div>
                        ) : (
                            <div
                                className={styles.virtualList}
                                style={{ height: totalHeight }}
                            >
                                <div
                                    className={styles.virtualContent}
                                    style={{
                                        transform: `translateY(${startIndex * ITEM_HEIGHT}px)`,
                                        height: (endIndex - startIndex) * ITEM_HEIGHT
                                    }}
                                >
                                    {visibleLogs.map((log, index) => {
                                        const actualIndex = startIndex + index
                                        return (
                                            <div
                                                key={`log-${actualIndex}`}
                                                className={`${styles.logEntry} ${styles[getLogLevel(log)]}`}
                                                style={{
                                                    height: ITEM_HEIGHT,
                                                    minHeight: ITEM_HEIGHT,
                                                    maxHeight: ITEM_HEIGHT
                                                }}
                                                title={log} // 전체 텍스트를 툴팁으로 표시
                                            >
                                                <div className={styles.logText}>
                                                    {truncateLog(log)}
                                                </div>
                                            </div>
                                        )
                                    })}
                                </div>
                            </div>
                        )}
                    </div>
                </div>
            </Card>

            <Card title="로그 설정" className={styles.settingsCard}>
                <div className={styles.settingsGrid}>
                    <div className={styles.settingItem}>
                        <Toggle
                            checked={autoRefresh}
                            onChange={handleAutoRefreshToggle}
                            label="자동 새로고침 (30초)"
                        />
                    </div>
                    <div className={styles.settingItem}>
                        <Toggle
                            checked={showDebug}
                            onChange={setShowDebug}
                            label="디버그 메시지 표시"
                        />
                    </div>
                    <div className={styles.filterGroup}>
                        <label className={styles.filterLabel}>
                            <Filter size={16} />
                            필터
                        </label>
                        <input
                            type="text"
                            value={filter}
                            onChange={(e) => setFilter(e.target.value || '')}
                            placeholder="필터링할 텍스트 입력"
                            className={styles.filterInput}
                        />
                    </div>
                    <div className={styles.logStats}>
                        <small>총 {logs.length}개 로그 (표시: {filteredLogs.length}개)</small>
                    </div>
                </div>
            </Card>
        </div>
    )
}