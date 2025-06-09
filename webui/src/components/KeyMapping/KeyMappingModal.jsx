import { useState, useEffect } from 'react'
import { X, Plus, Trash2, HelpCircle } from 'lucide-react'
import { keyMappingService } from '@services/keyMappingService'
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
        keys: [{ key: '', delay: 200 }]
    })
    const [availableKeys, setAvailableKeys] = useState({})
    const [errors, setErrors] = useState({})
    const [loading, setLoading] = useState(false)

    // 컴포넌트 마운트 시 사용 가능한 키 목록 로드
    useEffect(() => {
        if (isOpen) {
            loadAvailableKeys()

            // 편집 모드인 경우 기존 데이터 로드
            if (editingMapping) {
                setFormData({
                    name: editingMapping.name,
                    start_key: editingMapping.start_key,
                    keys: editingMapping.keys || [{ key: '', delay: 200 }]
                })
            } else {
                // 새 맵핑인 경우 초기화
                setFormData({
                    name: '',
                    start_key: '',
                    keys: [{ key: '', delay: 200 }]
                })
            }

            setErrors({})
        }
    }, [isOpen, editingMapping])

    // 사용 가능한 키 목록 로드
    const loadAvailableKeys = async () => {
        try {
            const data = await keyMappingService.getAvailableKeys()
            if (data.success) {
                setAvailableKeys(data.keys || {})
            }
        } catch (error) {
            console.error('사용 가능한 키 목록 로드 실패:', error)
        }
    }

    // 폼 입력 변경 처리
    const handleInputChange = (field, value) => {
        setFormData(prev => ({
            ...prev,
            [field]: value
        }))

        // 에러 제거
        if (errors[field]) {
            setErrors(prev => ({
                ...prev,
                [field]: null
            }))
        }
    }

    // 키 시퀀스 변경 처리
    const handleKeyChange = (index, field, value) => {
        const newKeys = [...formData.keys]
        newKeys[index] = {
            ...newKeys[index],
            [field]: field === 'delay' ? parseInt(value) || 200 : value
        }

        setFormData(prev => ({
            ...prev,
            keys: newKeys
        }))
    }

    // 키 추가
    const addKey = () => {
        setFormData(prev => ({
            ...prev,
            keys: [...prev.keys, { key: '', delay: 200 }]
        }))
    }

    // 키 제거
    const removeKey = (index) => {
        if (formData.keys.length > 1) {
            const newKeys = formData.keys.filter((_, i) => i !== index)
            setFormData(prev => ({
                ...prev,
                keys: newKeys
            }))
        }
    }

    // 유효성 검사
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
            // 중복 검사 (편집 모드가 아니거나 키가 변경된 경우)
            const isDuplicate = existingStartKeys.includes(formData.start_key) &&
                (!editingMapping || editingMapping.start_key !== formData.start_key)

            if (isDuplicate) {
                newErrors.start_key = '이미 사용 중인 시작 키입니다'
            }
        }

        // 키 시퀀스 검사
        formData.keys.forEach((keyItem, index) => {
            if (!keyItem.key.trim()) {
                newErrors[`key_${index}`] = '키를 선택해주세요'
            }

            if (keyItem.delay < 100 || keyItem.delay > 1000) {
                newErrors[`delay_${index}`] = '딜레이는 100~1000ms 사이여야 합니다'
            }
        })

        setErrors(newErrors)
        return Object.keys(newErrors).length === 0
    }

    // 저장 처리
    const handleSave = async () => {
        if (!validateForm()) {
            return
        }

        setLoading(true)

        try {
            // 키 시퀀스를 문자열로 변환
            const keySequence = formData.keys
                .map(key => `${key.key}(${key.delay})`)
                .join(',')

            const mappingData = {
                name: formData.name.trim(),
                start_key: formData.start_key.trim(),
                key_sequence: keySequence
            }

            await onSave(mappingData)
        } catch (error) {
            console.error('키 맵핑 저장 실패:', error)
        } finally {
            setLoading(false)
        }
    }

    // 모달이 열려있지 않으면 렌더링하지 않음
    if (!isOpen) return null

    return (
        <div className={styles.modalOverlay} onClick={onClose}>
            <div className={styles.modalContent} onClick={(e) => e.stopPropagation()}>
                <div className={styles.modalHeader}>
                    <h2>{editingMapping ? '키 맵핑 수정' : '새 키 맵핑 추가'}</h2>
                    <button
                        className={styles.closeButton}
                        onClick={onClose}
                        aria-label="닫기"
                    >
                        <X size={20} />
                    </button>
                </div>

                <div className={styles.modalBody}>
                    {/* 기본 정보 */}
                    <div className={styles.formGroup}>
                        <label className={styles.label}>
                            맵핑 이름 *
                        </label>
                        <input
                            type="text"
                            value={formData.name}
                            onChange={(e) => handleInputChange('name', e.target.value)}
                            placeholder="예: 도사, 천인, 마법사"
                            className={`${styles.input} ${errors.name ? styles.error : ''}`}
                        />
                        {errors.name && <span className={styles.errorText}>{errors.name}</span>}
                    </div>

                    <div className={styles.formGroup}>
                        <label className={styles.label}>
                            시작 키 *
                            <button
                                type="button"
                                className={styles.helpButton}
                                title="이 키를 누르면 키 시퀀스가 실행됩니다"
                            >
                                <HelpCircle size={14} />
                            </button>
                        </label>
                        <select
                            value={formData.start_key}
                            onChange={(e) => handleInputChange('start_key', e.target.value)}
                            className={`${styles.select} ${errors.start_key ? styles.error : ''}`}
                        >
                            <option value="">시작 키를 선택하세요</option>
                            {Object.entries(availableKeys).map(([category, keys]) => (
                                <optgroup key={category} label={category}>
                                    {keys.map(key => (
                                        <option key={key} value={key}>{key.toUpperCase()}</option>
                                    ))}
                                </optgroup>
                            ))}
                        </select>
                        {errors.start_key && <span className={styles.errorText}>{errors.start_key}</span>}
                    </div>

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
                                            {Object.entries(availableKeys).map(([category, keys]) => (
                                                <optgroup key={category} label={category}>
                                                    {keys.map(key => (
                                                        <option key={key} value={key}>{key.toUpperCase()}</option>
                                                    ))}
                                                </optgroup>
                                            ))}
                                        </select>
                                        {errors[`key_${index}`] &&
                                            <span className={styles.errorText}>{errors[`key_${index}`]}</span>
                                        }
                                    </div>

                                    <div className={styles.delayGroup}>
                                        <input
                                            type="number"
                                            min="100"
                                            max="1000"
                                            step="50"
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
                                    {index < formData.keys.length - 1 && ' → '}
                                </span>
                            ))}
                        </div>
                    </div>
                </div>

                <div className={styles.modalFooter}>
                    <Button
                        variant="outline"
                        onClick={onClose}
                        disabled={loading}
                    >
                        취소
                    </Button>
                    <Button
                        variant="primary"
                        onClick={handleSave}
                        loading={loading}
                    >
                        {editingMapping ? '수정' : '추가'}
                    </Button>
                </div>
            </div>
        </div>
    )
}