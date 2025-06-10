// webui/src/components/KeyMapping/KeyMappingModal.jsx 수정 (딜레이 관련 부분)
import { useState, useEffect } from 'react'
import { X, Plus, Trash2, HelpCircle } from 'lucide-react'
import { keyMappingService } from '@services/keyMappingService'
import { KEY_MAPPING } from '@constants/appConstants'
import Button from '@components/Common/Button'
import styles from './KeyMappingModal.module.css'

export default function KeyMappingModal({
    isOpen,
    onClose,
    onSave,
    editingMapping,
    existingStartKeys
}) {
    const [formData, setFormData] = useState({
        name: '',
        start_key: '',
        keys: [{ key: '', delay: KEY_MAPPING.DEFAULT_DELAY }] // 기본값 0ms
    })
    const [availableKeys, setAvailableKeys] = useState({})
    const [errors, setErrors] = useState({})
    const [loading, setLoading] = useState(false)

    // 시작키로 허용되는 키만 정의
    const allowedStartKeys = ['delete', 'end']

    // 컴포넌트 마운트 시 사용 가능한 키 목록 로드
    useEffect(() => {
        if (isOpen) {
            loadAvailableKeys()

            // 편집 모드인 경우 기존 데이터 로드
            if (editingMapping) {
                setFormData({
                    name: editingMapping.name,
                    start_key: editingMapping.start_key,
                    keys: editingMapping.keys || [{ key: '', delay: KEY_MAPPING.DEFAULT_DELAY }]
                })
            } else {
                // 새 맵핑인 경우 초기화
                setFormData({
                    name: '',
                    start_key: '',
                    keys: [{ key: '', delay: KEY_MAPPING.DEFAULT_DELAY }]
                })
            }

            setErrors({})
        }
    }, [isOpen, editingMapping])

    // ... 기존 함수들 그대로 유지 ...

    // 키 추가 (기본 딜레이 0ms)
    const addKey = () => {
        setFormData(prev => ({
            ...prev,
            keys: [...prev.keys, { key: '', delay: KEY_MAPPING.DEFAULT_DELAY }]
        }))
    }

    // ... 기존 함수들 그대로 유지 ...

    // 유효성 검사 (딜레이 범위 수정)
    const validateForm = () => {
        const newErrors = {}

        // 이름 검사
        if (!formData.name.trim()) {
            newErrors.name = '맵핑 이름을 입력해주세요'
        }

        // 시작 키 검사
        if (!formData.start_key.trim()) {
            newErrors.start_key = '시작 키를 선택해주세요'
        } else {
            // 허용된 시작키인지 확인
            if (!allowedStartKeys.includes(formData.start_key.toLowerCase())) {
                newErrors.start_key = '시작 키는 DELETE 또는 END만 사용할 수 있습니다'
            } else {
                // 중복 검사 (편집 모드가 아니거나 키가 변경된 경우)
                const isDuplicate = existingStartKeys.includes(formData.start_key) &&
                    (!editingMapping || editingMapping.start_key !== formData.start_key)

                if (isDuplicate) {
                    newErrors.start_key = '이미 사용 중인 시작 키입니다'
                }
            }
        }

        // 키 시퀀스 검사 (딜레이 범위 수정: 0ms~1000ms)
        formData.keys.forEach((keyItem, index) => {
            if (!keyItem.key.trim()) {
                newErrors[`key_${index}`] = '키를 선택해주세요'
            }

            // 딜레이 범위 수정: 0ms~1000ms
            if (keyItem.delay < KEY_MAPPING.MIN_DELAY || keyItem.delay > KEY_MAPPING.MAX_DELAY) {
                newErrors[`delay_${index}`] = `딜레이는 ${KEY_MAPPING.MIN_DELAY}~${KEY_MAPPING.MAX_DELAY}ms 사이여야 합니다`
            }
        })

        setErrors(newErrors)
        return Object.keys(newErrors).length === 0
    }

    // ... 기존 JSX 코드에서 딜레이 입력 부분만 수정 ...

    return (
        <div className={styles.modalOverlay} onClick={onClose}>
            <div className={styles.modalContent} onClick={(e) => e.stopPropagation()}>
                {/* ... 기존 헤더 코드 ... */}

                <div className={styles.modalBody}>
                    {/* ... 기존 폼 필드들 ... */}

                    {/* 키 시퀀스 */}
                    <div className={styles.formGroup}>
                        <label className={styles.label}>
                            키 시퀀스 *
                            <button
                                type="button"
                                className={styles.helpButton}
                                title="시작 키를 누르면 이 순서대로 키가 입력됩니다"
                            >
                                <HelpCircle size={14} />
                            </button>
                        </label>

                        <div className={styles.keySequence}>
                            {formData.keys.map((keyItem, index) => (
                                <div key={index} className={styles.keyRow}>
                                    <div className={styles.keyIndex}>{index + 1}</div>

                                    <div className={styles.keySelectGroup}>
                                        <select
                                            value={keyItem.key}
                                            onChange={(e) => handleKeyChange(index, 'key', e.target.value)}
                                            className={`${styles.keySelect} ${errors[`key_${index}`] ? styles.error : ''}`}
                                        >
                                            <option value="">키 선택</option>
                                            {Object.entries(availableKeys).map(([category, keys]) => {
                                                if (category === '시작 키') return null

                                                return (
                                                    <optgroup key={category} label={category}>
                                                        {keys.map(key => (
                                                            <option key={key} value={key}>{key.toUpperCase()}</option>
                                                        ))}
                                                    </optgroup>
                                                )
                                            })}
                                        </select>
                                        {errors[`key_${index}`] &&
                                            <span className={styles.errorText}>{errors[`key_${index}`]}</span>
                                        }
                                    </div>

                                    <div className={styles.delayGroup}>
                                        <input
                                            type="number"
                                            min={KEY_MAPPING.MIN_DELAY}      // 0ms
                                            max={KEY_MAPPING.MAX_DELAY}      // 1000ms
                                            step="10"
                                            value={keyItem.delay}
                                            onChange={(e) => handleKeyChange(index, 'delay', e.target.value)}
                                            className={`${styles.delayInput} ${errors[`delay_${index}`] ? styles.error : ''}`}
                                        />
                                        <span className={styles.delayUnit}>ms</span>
                                        {errors[`delay_${index}`] &&
                                            <span className={styles.errorText}>{errors[`delay_${index}`]}</span>
                                        }
                                    </div>

                                    <button
                                        type="button"
                                        onClick={() => removeKey(index)}
                                        disabled={formData.keys.length <= 1}
                                        className={styles.removeKeyButton}
                                        title="키 제거"
                                    >
                                        <Trash2 size={16} />
                                    </button>
                                </div>
                            ))}

                            <button
                                type="button"
                                onClick={addKey}
                                className={styles.addKeyButton}
                            >
                                <Plus size={16} />
                                키 추가
                            </button>
                        </div>

                        <small className={styles.formHelp}>
                            딜레이 범위: 0ms~1000ms (0ms = 딜레이 없음)
                        </small>
                    </div>

                    {/* 미리보기 */}
                    <div className={styles.preview}>
                        <h4>미리보기</h4>
                        <div className={styles.previewContent}>
                            <strong>{formData.start_key.toUpperCase()}</strong> 키를 누르면 → {' '}
                            {formData.keys.map((key, index) => (
                                <span key={index}>
                                    <strong>{key.key.toUpperCase()}</strong>
                                    {key.delay > 0 && <span className={styles.delay}>({key.delay}ms)</span>}
                                    {key.delay === 0 && <span className={styles.delay}>(즉시)</span>}
                                    {index < formData.keys.length - 1 && ' → '}
                                </span>
                            ))}
                        </div>
                    </div>
                </div>

                {/* ... 기존 푸터 코드 ... */}
            </div>
        </div>
    )
}