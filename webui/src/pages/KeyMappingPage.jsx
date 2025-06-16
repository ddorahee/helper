// KeyMappingPage.jsx - 테이블 형태로 개선
import React, { useState, useEffect, useCallback, useMemo, useRef } from 'react'
import { Plus, Edit, Trash2, Play, Square, ToggleLeft, ToggleRight, Copy, Users, Keyboard } from 'lucide-react'
import { useApp } from '@/contexts/AppContext'
import { keyMappingService } from '@services/keyMappingService'
import { useNotification } from '@hooks/useNotification'
import Card from '@components/Common/Card'
import Button from '@components/Common/Button'
import KeyMappingModal from '@components/KeyMapping/KeyMappingModal'
import styles from './KeyMappingPage.module.css'

// 메모화된 맵핑 아이템 컴포넌트 - 테이블 행 형태
const MappingRow = React.memo(({
    mappingID,
    mapping,
    onToggle,
    onEdit,
    onDelete,
    updating
}) => {
    // 키 시퀀스 포맷팅 (메모화)
    const formattedKeySequence = useMemo(() => {
        if (!mapping.keys || mapping.keys.length === 0) return ''
        return mapping.keys.map(key => {
            const keyDisplay = isComboKey(key.key) ? key.key.toUpperCase() : key.key.toUpperCase()
            return key.delay === 0 ? `${keyDisplay}(즉시)` : `${keyDisplay}(${key.delay}ms)`
        }).join(' → ')
    }, [mapping.keys])

    // 조합키 확인 (메모화)
    const hasComboKey = useMemo(() => {
        return mapping.keys && mapping.keys.some(key => isComboKey(key.key))
    }, [mapping.keys])

    // 이벤트 핸들러들 (useCallback으로 최적화)
    const handleToggle = useCallback(() => {
        onToggle(mappingID, mapping)
    }, [mappingID, mapping, onToggle])

    const handleEdit = useCallback(() => {
        onEdit(mapping)
    }, [mapping, onEdit])

    const handleDelete = useCallback(() => {
        onDelete(mappingID, mapping)
    }, [mappingID, mapping, onDelete])

    return (
        <tr className={`${styles.mappingRow} ${!mapping.enabled ? styles.disabled : ''}`}>
            <td className={styles.nameCell}>
                <div className={styles.nameContainer}>
                    <span className={styles.mappingName}>{mapping.name}</span>
                    <div className={styles.badges}>
                        {mapping.is_duplicate && (
                            <span className={styles.duplicateBadge}>
                                <Users size={12} />
                                {mapping.duplicate_index + 1}/{mapping.total_duplicates}
                            </span>
                        )}
                        {hasComboKey && (
                            <span className={styles.comboKeyBadge}>
                                <Keyboard size={12} />
                                조합
                            </span>
                        )}
                    </div>
                </div>
            </td>
            <td className={styles.startKeyCell}>
                <span className={styles.startKey}>
                    {mapping.start_key?.toUpperCase()}
                </span>
            </td>
            <td className={styles.sequenceCell}>
                <span className={styles.keySequence} title={formattedKeySequence}>
                    {formattedKeySequence}
                </span>
            </td>
            <td className={styles.statusCell}>
                <span className={`${styles.status} ${mapping.enabled ? styles.enabled : styles.disabled}`}>
                    {mapping.enabled ? '활성' : '비활성'}
                </span>
            </td>
            <td className={styles.actionsCell}>
                <div className={styles.actionButtons}>
                    <button
                        type="button"
                        className={`${styles.actionButton} ${styles.toggleButton} ${mapping.enabled ? styles.active : ''}`}
                        onClick={handleToggle}
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
                        onClick={handleEdit}
                        disabled={updating}
                        title="수정"
                    >
                        <Edit size={16} />
                    </button>
                    <button
                        type="button"
                        className={`${styles.actionButton} ${styles.deleteButton}`}
                        onClick={handleDelete}
                        disabled={updating}
                        title="삭제"
                    >
                        <Trash2 size={16} />
                    </button>
                </div>
            </td>
        </tr>
    )
})

MappingRow.displayName = 'MappingRow'

// 조합키 확인 함수 (외부로 분리)
const isComboKey = (key) => {
    return key && (
        key.includes('ctrl+') ||
        key.includes('shift+') ||
        key.includes('alt+') ||
        key.includes('cmd+') ||
        key.includes('win+')
    )
}

// 메인 컴포넌트
export default function KeyMappingPage() {
    const { actions } = useApp()
    const { showNotification } = useNotification()

    // 상태들
    const [mappings, setMappings] = useState({})
    const [duplicateInfo, setDuplicateInfo] = useState({})
    const [stats, setStats] = useState({})
    const [loading, setLoading] = useState(false)
    const [isModalOpen, setIsModalOpen] = useState(false)
    const [editingMapping, setEditingMapping] = useState(null)
    const [systemRunning, setSystemRunning] = useState(false)
    const [updating, setUpdating] = useState(false)

    // 중복 렌더링 방지를 위한 ref
    const loadingRef = useRef(false)
    const lastLoadTime = useRef(0)

    // 컴포넌트 마운트
    useEffect(() => {
        console.log('KeyMappingPage 마운트됨 (테이블 형태)')
        loadMappings()
        loadStatus()

        return () => {
            console.log('KeyMappingPage 언마운트됨')
        }
    }, [])

    // 키 맵핑 목록 로드 (최적화)
    const loadMappings = useCallback(async () => {
        if (loadingRef.current) {
            console.log('이미 로딩 중이므로 요청 무시')
            return
        }

        const now = Date.now()
        if (now - lastLoadTime.current < 500) {
            console.log('너무 빠른 요청, 무시')
            return
        }

        try {
            console.log('키 맵핑 목록 로드 시작...')
            loadingRef.current = true
            setLoading(true)
            lastLoadTime.current = now

            const data = await keyMappingService.getMappings()
            console.log('로드된 데이터:', Object.keys(data.mappings || {}).length, '개')

            if (data.success) {
                setMappings(data.mappings || {})
                setStats(data.stats || {})
                setDuplicateInfo(data.duplicate_info || {})

                console.log(`총 ${Object.keys(data.mappings || {}).length}개 맵핑 로드됨`)
            } else {
                showNotification('키 맵핑 목록을 불러올 수 없습니다', 'error')
            }
        } catch (error) {
            console.error('키 맵핑 로드 실패:', error)
            showNotification('키 맵핑 목록 로드 중 오류가 발생했습니다', 'error')
        } finally {
            setLoading(false)
            loadingRef.current = false
        }
    }, [showNotification])

    // 시스템 상태 로드
    const loadStatus = useCallback(async () => {
        try {
            const data = await keyMappingService.getStatus()
            if (data.success) {
                setSystemRunning(data.stats?.running || false)
            }
        } catch (error) {
            console.error('상태 로드 실패:', error)
        }
    }, [])

    // 키 맵핑 저장 핸들러 (최적화)
    const handleSaveMapping = useCallback(async (mappingData) => {
        try {
            let success = false

            if (editingMapping) {
                const updateData = {
                    id: editingMapping.id,
                    ...mappingData
                }
                console.log('키 맵핑 수정 데이터:', updateData.name)
                success = await keyMappingService.updateMapping(updateData)

                if (success) {
                    showNotification(`키 맵핑 '${mappingData.name}'이 수정되었습니다`, 'success')
                    actions.addLog(`키 맵핑 수정: ${mappingData.name}`)
                }
            } else {
                console.log('키 맵핑 추가 데이터:', mappingData.name)
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
    }, [editingMapping, showNotification, actions, loadMappings])

    // 키 맵핑 삭제 핸들러 (최적화)
    const handleDeleteMapping = useCallback(async (mappingID, mapping) => {
        if (!mapping?.id) {
            console.error('삭제 실패: ID 정보가 없음')
            showNotification('삭제할 키 맵핑 정보가 올바르지 않습니다', 'error')
            return
        }

        const displayName = `${mapping.name}${mapping.is_duplicate ? ` (${mapping.duplicate_index + 1}/${mapping.total_duplicates})` : ''}`

        if (!confirm(`키 맵핑 '${displayName}'을 삭제하시겠습니까?`)) {
            return
        }

        try {
            setUpdating(true)
            console.log('삭제 요청:', mapping.name)

            const result = await keyMappingService.deleteMappingByID(mapping.id)

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
    }, [showNotification, actions, loadMappings])

    // 키 맵핑 토글 핸들러 (최적화)
    const handleToggleMapping = useCallback(async (mappingID, mapping) => {
        if (updating || !mapping?.id) {
            return
        }

        try {
            setUpdating(true)
            console.log('토글 요청:', mapping.name)

            const success = await keyMappingService.toggleMappingByID(mapping.id)

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
    }, [updating, showNotification, actions, loadMappings])

    // 시스템 제어 핸들러
    const handleSystemControl = useCallback(async (action) => {
        try {
            setUpdating(true)
            const success = await keyMappingService.controlSystem(action)

            if (success) {
                const message = action === 'start'
                    ? '키 맵핑 시스템이 시작되었습니다'
                    : '키 맵핑 시스템이 중지되었습니다'
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
    }, [showNotification, actions, loadStatus])

    // 모달 관련 핸들러들
    const handleAddMapping = useCallback(() => {
        setEditingMapping(null)
        setIsModalOpen(true)
    }, [])

    const handleEditMapping = useCallback((mapping) => {
        console.log('수정할 맵핑:', mapping.name)
        setEditingMapping(mapping)
        setIsModalOpen(true)
    }, [])

    const handleCloseModal = useCallback(() => {
        setIsModalOpen(false)
        setEditingMapping(null)
    }, [])

    // 기존 시작키 목록 (메모화)
    const existingStartKeys = useMemo(() => {
        const startKeys = new Set()
        Object.values(mappings).forEach(mapping => {
            if (mapping.start_key) {
                startKeys.add(mapping.start_key)
            }
        })
        return Array.from(startKeys)
    }, [mappings])

    // 맵핑 목록을 배열로 변환 (메모화)
    const mappingsList = useMemo(() => {
        return Object.entries(mappings).map(([mappingID, mapping]) => ({
            mappingID,
            mapping
        })).sort((a, b) => {
            // 활성화된 맵핑을 위로, 그 다음 이름순
            if (a.mapping.enabled !== b.mapping.enabled) {
                return b.mapping.enabled - a.mapping.enabled
            }
            return a.mapping.name.localeCompare(b.mapping.name)
        })
    }, [mappings])

    // 중복키 정보 렌더링 (메모화)
    const duplicateInfoSection = useMemo(() => {
        if (Object.keys(duplicateInfo).length === 0) return null

        return (
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
        )
    }, [duplicateInfo])

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

                {duplicateInfoSection}
            </Card>

            {/* 키 맵핑 목록 카드 - 테이블 형태 */}
            <Card
                title={`키 맵핑 목록 (${Object.keys(mappings).length}개)`}
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
                ) : mappingsList.length === 0 ? (
                    <div className={styles.emptyState}>
                        <p>등록된 키 맵핑이 없습니다.</p>
                        <p>새 맵핑을 추가해서 시작해보세요!</p>
                    </div>
                ) : (
                    <div className={styles.tableContainer}>
                        <table className={styles.mappingsTable}>
                            <thead>
                                <tr>
                                    <th className={styles.nameHeader}>맵핑 이름</th>
                                    <th className={styles.startKeyHeader}>시작키</th>
                                    <th className={styles.sequenceHeader}>키 시퀀스</th>
                                    <th className={styles.statusHeader}>상태</th>
                                    <th className={styles.actionsHeader}>작업</th>
                                </tr>
                            </thead>
                            <tbody>
                                {mappingsList.map(({ mappingID, mapping }) => (
                                    <MappingRow
                                        key={mappingID}
                                        mappingID={mappingID}
                                        mapping={mapping}
                                        onToggle={handleToggleMapping}
                                        onEdit={handleEditMapping}
                                        onDelete={handleDeleteMapping}
                                        updating={updating}
                                    />
                                ))}
                            </tbody>
                        </table>
                    </div>
                )}
            </Card>

            {/* 키 맵핑 추가/수정 모달 */}
            {isModalOpen && (
                <KeyMappingModal
                    isOpen={isModalOpen}
                    onClose={handleCloseModal}
                    onSave={handleSaveMapping}
                    editingMapping={editingMapping}
                    existingStartKeys={existingStartKeys}
                />
            )}
        </div>
    )
}
