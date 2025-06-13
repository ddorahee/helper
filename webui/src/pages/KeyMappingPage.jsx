// KeyMappingPage.jsx - 최종 수정 (삭제/토글 문제 해결)
import { useState, useEffect } from 'react'
import { Plus, Edit, Trash2, Play, Square, ToggleLeft, ToggleRight, Copy, Users } from 'lucide-react'
import { useApp } from '@/contexts/AppContext'
import { keyMappingService } from '@services/keyMappingService'
import { useNotification } from '@hooks/useNotification'
import Card from '@components/Common/Card'
import Button from '@components/Common/Button'
import KeyMappingModal from '@components/KeyMapping/KeyMappingModal'
import styles from './KeyMappingPage.module.css'

export default function KeyMappingPage() {
    const { actions } = useApp()
    const { showNotification } = useNotification()
    const [mappings, setMappings] = useState({})
    const [duplicateInfo, setDuplicateInfo] = useState({})
    const [stats, setStats] = useState({})
    const [loading, setLoading] = useState(false)
    const [isModalOpen, setIsModalOpen] = useState(false)
    const [editingMapping, setEditingMapping] = useState(null)
    const [systemRunning, setSystemRunning] = useState(false)
    const [updating, setUpdating] = useState(false)

    useEffect(() => {
        console.log('KeyMappingPage 마운트됨')
        loadMappings()
        loadStatus()

        return () => {
            console.log('KeyMappingPage 언마운트됨')
        }
    }, [])

    // 키 맵핑 목록 로드
    const loadMappings = async () => {
        try {
            console.log('키 맵핑 목록 로드 시작...')
            setLoading(true)
            const data = await keyMappingService.getMappings()
            console.log('로드된 데이터:', data)

            if (data.success) {
                setMappings(data.mappings || {})
                setStats(data.stats || {})
                setDuplicateInfo(data.duplicate_info || {})
            } else {
                showNotification('키 맵핑 목록을 불러올 수 없습니다', 'error')
            }
        } catch (error) {
            console.error('키 맵핑 로드 실패:', error)
            showNotification('키 맵핑 목록 로드 중 오류가 발생했습니다', 'error')
        } finally {
            setLoading(false)
        }
    }

    // 시스템 상태 로드
    const loadStatus = async () => {
        try {
            const data = await keyMappingService.getStatus()
            if (data.success) {
                setSystemRunning(data.stats?.running || false)
            }
        } catch (error) {
            console.error('상태 로드 실패:', error)
        }
    }

    // 키 맵핑 추가/수정
    const handleSaveMapping = async (mappingData) => {
        try {
            let success = false

            if (editingMapping) {
                const realStartKey = getRealStartKey(editingMapping)
                success = await keyMappingService.updateMapping({
                    old_start_key: realStartKey,
                    ...mappingData
                })

                if (success) {
                    showNotification(`키 맵핑 '${mappingData.name}'이 수정되었습니다`, 'success')
                    actions.addLog(`키 맵핑 수정: ${mappingData.name}`)
                }
            } else {
                success = await keyMappingService.createMapping(mappingData)

                if (success) {
                    showNotification(`키 맵핑 '${mappingData.name}'이 추가되었습니다`, 'success')
                    actions.addLog(`키 맵핑 추가: ${mappingData.name}`)
                }
            }

            if (success) {
                setIsModalOpen(false)
                setEditingMapping(null)
                await loadMappings()
            } else {
                showNotification('키 맵핑 저장에 실패했습니다', 'error')
            }
        } catch (error) {
            console.error('키 맵핑 저장 실패:', error)
            showNotification('키 맵핑 저장 중 오류가 발생했습니다', 'error')
        }
    }

    // 키 맵핑 삭제 (시작키 기반으로 단순화)
    const handleDeleteMapping = async (mappingKey, mapping) => {
        console.log('삭제 요청:', { mappingKey, mapping })

        if (!mapping || !mapping.start_key) {
            console.error('삭제 실패: 시작키 정보가 없음')
            showNotification('삭제할 키 맵핑 정보가 올바르지 않습니다', 'error')
            return
        }

        const displayName = `${mapping.name}${mapping.is_duplicate ? ` (${mapping.duplicate_index + 1}/${mapping.total_duplicates})` : ''}`

        if (!confirm(`키 맵핑 '${displayName}'을 삭제하시겠습니까?`)) {
            return
        }

        try {
            setUpdating(true)
            console.log('삭제 API 호출:', mapping.start_key)

            // 시작키 기반 삭제만 사용
            const result = await keyMappingService.deleteMapping(mapping.start_key)
            console.log('삭제 API 응답:', result)

            if (result) {
                showNotification(`키 맵핑 '${displayName}'이 삭제되었습니다`, 'success')
                actions.addLog(`키 맵핑 삭제: ${displayName}`)
                await loadMappings()
            } else {
                showNotification('키 맵핑 삭제에 실패했습니다', 'error')
            }
        } catch (error) {
            console.error('키 맵핑 삭제 실패:', error)
            showNotification(`키 맵핑 삭제 중 오류: ${error.message}`, 'error')
        } finally {
            setUpdating(false)
        }
    }

    // 키 맵핑 토글 (단순화)
    const handleToggleMapping = async (mappingKey, mapping) => {
        console.log('토글 핸들러 호출:', { mappingKey, mapping })

        if (updating || !mapping || !mapping.start_key) {
            console.log('토글 조건 불충족:', { updating, hasMapping: !!mapping, hasStartKey: !!mapping?.start_key })
            return
        }

        try {
            setUpdating(true)
            console.log('토글 요청 시작:', mapping.name, mapping.start_key)

            // 시작키 기반 토글만 사용
            const success = await keyMappingService.toggleMapping(mapping.start_key)
            console.log('토글 API 응답:', success)

            if (success) {
                const newStatus = mapping.enabled ? '비활성화' : '활성화'
                const displayName = `${mapping.name}${mapping.is_duplicate ? ` (${mapping.duplicate_index + 1}/${mapping.total_duplicates})` : ''}`

                showNotification(`키 맵핑 '${displayName}'이 ${newStatus}되었습니다`, 'success')
                actions.addLog(`키 맵핑 ${newStatus}: ${displayName}`)

                await loadMappings()
            } else {
                showNotification('키 맵핑 상태 변경에 실패했습니다', 'error')
            }
        } catch (error) {
            console.error('키 맵핑 토글 실패:', error)
            showNotification('키 맵핑 상태 변경 중 오류가 발생했습니다', 'error')
        } finally {
            setUpdating(false)
        }
    }

    // 시스템 시작/중지
    const handleSystemControl = async (action) => {
        try {
            setUpdating(true)
            const success = await keyMappingService.controlSystem(action)

            if (success) {
                const message = action === 'start' ? '키 맵핑 시스템이 시작되었습니다' : '키 맵핑 시스템이 중지되었습니다'
                showNotification(message, 'success')
                actions.addLog(message)
                setSystemRunning(action === 'start')
                await loadStatus()
            } else {
                showNotification(`키 맵핑 시스템 ${action === 'start' ? '시작' : '중지'}에 실패했습니다`, 'error')
            }
        } catch (error) {
            console.error('시스템 제어 실패:', error)
            showNotification('시스템 제어 중 오류가 발생했습니다', 'error')
        } finally {
            setUpdating(false)
        }
    }

    // 새 맵핑 추가
    const handleAddMapping = () => {
        setEditingMapping(null)
        setIsModalOpen(true)
    }

    // 맵핑 수정 (단순화)
    const handleEditMapping = (mapping) => {
        setEditingMapping(mapping)
        setIsModalOpen(true)
    }

    // 실제 시작키 추출
    const getRealStartKey = (mapping) => {
        return mapping.start_key || 'delete'
    }

    // 기존 시작키 목록 추출
    const getExistingStartKeys = () => {
        const startKeys = new Set()
        Object.values(mappings).forEach(mapping => {
            if (mapping.start_key) {
                startKeys.add(mapping.start_key)
            }
        })
        return Array.from(startKeys)
    }

    // 키 시퀀스 포맷팅
    const formatKeySequence = (keys) => {
        if (!keys || keys.length === 0) return ''
        return keys.map(key => {
            const keyDisplay = isComboKey(key.key) ? key.key.toUpperCase() : key.key.toUpperCase()
            return `${keyDisplay}(${key.delay}ms)`
        }).join(', ')
    }

    // 조합키 확인
    const isComboKey = (key) => {
        return key && (
            key.includes('ctrl+') ||
            key.includes('shift+') ||
            key.includes('alt+') ||
            key.includes('cmd+') ||
            key.includes('win+')
        )
    }

    // 중복키 그룹 정보 생성
    const getDuplicateGroupInfo = (mapping) => {
        if (!mapping.is_duplicate) return null

        return {
            current: mapping.duplicate_index + 1,
            total: mapping.total_duplicates,
            isActive: mapping.enabled
        }
    }

    return (
        <div className={styles.keyMappingPage}>
            {/* 시스템 상태 카드 */}
            <Card
                title="키 맵핑 시스템 상태"
                className={styles.statusCard}
                headerContent={
                    <div className={styles.systemControls}>
                        <Button
                            variant={systemRunning ? 'secondary' : 'success'}
                            size="sm"
                            onClick={() => handleSystemControl('start')}
                            disabled={systemRunning || updating}
                            icon={<Play size={16} />}
                        >
                            시작
                        </Button>
                        <Button
                            variant="danger"
                            size="sm"
                            onClick={() => handleSystemControl('stop')}
                            disabled={!systemRunning || updating}
                            icon={<Square size={16} />}
                        >
                            중지
                        </Button>
                    </div>
                }
            >
                <div className={styles.statsGrid}>
                    <div className={styles.statItem}>
                        <span className={styles.statLabel}>상태</span>
                        <span className={`${styles.statValue} ${systemRunning ? styles.running : styles.stopped}`}>
                            {systemRunning ? '실행 중' : '중지됨'}
                        </span>
                    </div>
                    <div className={styles.statItem}>
                        <span className={styles.statLabel}>전체 맵핑</span>
                        <span className={styles.statValue}>{stats.total || 0}개</span>
                    </div>
                    <div className={styles.statItem}>
                        <span className={styles.statLabel}>활성화</span>
                        <span className={styles.statValue}>{stats.enabled || 0}개</span>
                    </div>
                    <div className={styles.statItem}>
                        <span className={styles.statLabel}>중복키</span>
                        <span className={styles.statValue}>{stats.duplicate_keys || 0}개</span>
                    </div>
                </div>

                {/* 중복키 정보 표시 */}
                {Object.keys(duplicateInfo).length > 0 && (
                    <div className={styles.duplicateInfoSection}>
                        <h4><Copy size={16} /> 중복키 정보</h4>
                        {Object.entries(duplicateInfo).map(([startKey, info]) => (
                            <div key={startKey} className={styles.duplicateKeyInfo}>
                                <span className={styles.duplicateKeyName}>{startKey.toUpperCase()}</span>
                                <span className={styles.duplicateKeyCount}>{info.count}개 맵핑</span>
                                <span className={styles.duplicateKeyActive}>활성화: {info.active}</span>
                            </div>
                        ))}
                    </div>
                )}
            </Card>

            {/* 키 맵핑 목록 카드 */}
            <Card
                title="키 맵핑 목록"
                className={styles.mappingsCard}
                headerContent={
                    <Button
                        variant="primary"
                        size="sm"
                        onClick={handleAddMapping}
                        icon={<Plus size={16} />}
                        disabled={updating}
                    >
                        새 맵핑 추가
                    </Button>
                }
            >
                {loading ? (
                    <div className={styles.loading}>키 맵핑을 불러오는 중...</div>
                ) : Object.keys(mappings).length === 0 ? (
                    <div className={styles.emptyState}>
                        <p>등록된 키 맵핑이 없습니다.</p>
                        <p>새 맵핑을 추가해서 시작해보세요!</p>
                    </div>
                ) : (
                    <div className={styles.mappingsList}>
                        {Object.entries(mappings).map(([mappingKey, mapping]) => {
                            const duplicateGroup = getDuplicateGroupInfo(mapping)

                            return (
                                <div key={mappingKey} className={styles.mappingItem}>
                                    <div className={styles.mappingHeader}>
                                        <div className={styles.mappingInfo}>
                                            <div className={styles.mappingTitleRow}>
                                                <h3 className={styles.mappingName}>
                                                    {mapping.name}
                                                    {duplicateGroup && (
                                                        <span className={styles.duplicateBadge}>
                                                            <Users size={12} />
                                                            {duplicateGroup.current}/{duplicateGroup.total}
                                                        </span>
                                                    )}
                                                </h3>
                                                {isComboKey(mapping.keys?.[0]?.key) && (
                                                    <span className={styles.comboKeyBadge}>조합키</span>
                                                )}
                                            </div>
                                            <div className={styles.mappingDetails}>
                                                <span className={styles.startKey}>
                                                    시작키: <strong>{mapping.start_key?.toUpperCase()}</strong>
                                                </span>
                                                <span className={styles.mappingStatus}>
                                                    {mapping.enabled ? (
                                                        <span className={styles.enabled}>활성화</span>
                                                    ) : (
                                                        <span className={styles.disabled}>비활성화</span>
                                                    )}
                                                </span>
                                                {duplicateGroup && !duplicateGroup.isActive && (
                                                    <span className={styles.duplicateNote}>
                                                        중복키 (비활성화)
                                                    </span>
                                                )}
                                            </div>
                                        </div>
                                        <div className={styles.mappingActions}>
                                            <button
                                                type="button"
                                                className={`${styles.actionButton} ${styles.toggleButton}`}
                                                onClick={() => handleToggleMapping(mappingKey, mapping)}
                                                disabled={updating}
                                                title={mapping.enabled ? '비활성화' : '활성화'}
                                            >
                                                {updating ? (
                                                    <div className={styles.spinner} />
                                                ) : mapping.enabled ? (
                                                    <ToggleRight size={16} />
                                                ) : (
                                                    <ToggleLeft size={16} />
                                                )}
                                            </button>
                                            <button
                                                type="button"
                                                className={`${styles.actionButton} ${styles.editButton}`}
                                                onClick={() => handleEditMapping(mapping)}
                                                disabled={updating}
                                                title="수정"
                                            >
                                                <Edit size={16} />
                                            </button>
                                            <button
                                                type="button"
                                                className={`${styles.actionButton} ${styles.deleteButton}`}
                                                onClick={() => handleDeleteMapping(mappingKey, mapping)}
                                                disabled={updating}
                                                title="삭제"
                                            >
                                                <Trash2 size={16} />
                                            </button>
                                        </div>
                                    </div>
                                    <div className={styles.keySequence}>
                                        <span className={styles.sequenceLabel}>키 시퀀스:</span>
                                        <span className={styles.sequenceValue}>
                                            {formatKeySequence(mapping.keys)}
                                        </span>
                                    </div>
                                    <div className={styles.mappingMeta}>
                                        <span>생성: {new Date(mapping.created_at).toLocaleString()}</span>
                                        {mapping.updated_at !== mapping.created_at && (
                                            <span>수정: {new Date(mapping.updated_at).toLocaleString()}</span>
                                        )}
                                    </div>
                                </div>
                            )
                        })}
                    </div>
                )}
            </Card>

            {/* 키 맵핑 추가/수정 모달 */}
            {isModalOpen && (
                <KeyMappingModal
                    isOpen={isModalOpen}
                    onClose={() => {
                        setIsModalOpen(false)
                        setEditingMapping(null)
                    }}
                    onSave={handleSaveMapping}
                    editingMapping={editingMapping}
                    existingStartKeys={getExistingStartKeys()}
                />
            )}
        </div>
    )
}
