// webui/src/components/KeyMapping/KeyMappingModal.jsx 디버깅 강화
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
        keys: [{ key: '', delay: KEY_MAPPING.DEFAULT_DELAY }]
    })
    const [availableKeys, setAvailableKeys] = useState({})
    const [errors, setErrors] = useState({})
    const [loading, setLoading] = useState(false)
    const [keysLoading, setKeysLoading] = useState(false)

    // 시작키로 허용되는 키만 정의
    const allowedStartKeys = ['delete', 'end']

    // 컴포넌트 마운트 시 사용 가능한 키 목록 로드
    useEffect(() => {
        if (isOpen) {
            console.log('키 맵핑 모달 열림, 키 목록 로드 시작')
            loadAvailableKeys()

            // 편집 모드인 경우 기존 데이터 로드
            if (editingMapping) {
                console.log('편집 모드, 기존 데이터 로드:', editingMapping)
                setFormData({
                    name: editingMapping.name,
                    start_key: editingMapping.start_key,
                    keys: editingMapping.keys || [{ key: '', delay: KEY_MAPPING.DEFAULT_DELAY }]
                })
            } else {
                // 새 맵핑인 경우 초기화
                console.log('새 맵핑 모드, 초기화')
                setFormData({
                    name: '',
                    start_key: '',
                    keys: [{ key: '', delay: KEY_MAPPING.DEFAULT_DELAY }]
                })
            }

            setErrors({})
        }
    }, [isOpen, editingMapping])

    // 사용 가능한 키 목록 로드 (디버깅 강화)
    const loadAvailableKeys = async () => {
        try {
            console.log('사용 가능한 키 목록 로드 시작...')
            setKeysLoading(true)

            const data = await keyMappingService.getAvailableKeys()
            console.log('키 목록 API 응답:', data)

            if (data && data.success && data.keys) {
                console.log('사용 가능한 키 목록:', data.keys)
                setAvailableKeys(data.keys)

                // 각 카테고리별 키 개수 로그
                Object.entries(data.keys).forEach(([category, keys]) => {
                    console.log(`카테고리 '${category}': ${keys.length}개 키`, keys)
                })
            } else {
                console.error('키 목록 로드 실패 또는 잘못된 응답:', data)

                // 기본 키 목록 설정 (백업)
                const defaultKeys = {
                    "숫자 키": ["1", "2", "3", "4", "5", "6", "7", "8", "9", "0"],
                    "알파벳 키": ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"],
                    "펑션 키": ["f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9", "f10", "f11", "f12"],
                    "특수 키": ["space", "enter", "esc", "tab"],
                    "화살표 키": ["left", "up", "right", "down"],
                    "넘패드": ["num0", "num1", "num2", "num3", "num4", "num5", "num6", "num7", "num8", "num9"],
                    "조합 키": ["shift", "ctrl", "alt"]
                }

                console.log('기본 키 목록 사용:', defaultKeys)
                setAvailableKeys(defaultKeys)
            }
        } catch (error) {
            console.error('사용 가능한 키 목록 로드 실패:', error)

            // 에러 발생 시 기본 키 목록 설정
            const defaultKeys = {
                "숫자 키": ["1", "2", "3", "4", "5", "6", "7", "8", "9", "0"],
                "알파벳 키": ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"],
                "특수 키": ["space", "enter", "esc", "tab"]
            }

            console.log('에러로 인한 기본 키 목록 사용:', defaultKeys)
            setAvailableKeys(defaultKeys)
        } finally {
            setKeysLoading(false)
        }
    }

    // 시작키용 키 목록 필터링
    const getStartKeyOptions = () => {
        return allowedStartKeys.map(key => ({
            value: key,
            label: key.toUpperCase()
        }))
    }

    // 폼 입력 변경 처리
    const handleInputChange = (field, value) => {
        console.log(`폼 입력 변경: ${field} = ${value}`)
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
        console.log(`키 시퀀스 변경: [${index}].${field} = ${value}`)
        const newKeys = [...formData.keys]
        newKeys[index] = {
            ...newKeys[index],
            [field]: field === 'delay' ? parseInt(value) || KEY_MAPPING.DEFAULT_DELAY : value
        }

        setFormData(prev => ({
            ...prev,
            keys: newKeys
        }))
    }

    // 키 추가
    const addKey = () => {
        console.log('새 키 추가')
        setFormData(prev => ({
            ...prev,
            keys: [...prev.keys, { key: '', delay: KEY_MAPPING.DEFAULT_DELAY }]
        }))
    }

    // 키 제거
    const removeKey = (index) => {
        if (formData.keys.length > 1) {
            console.log(`키 제거: index ${index}`)
            const newKeys = formData.keys.filter((_, i) => i !== index)
            setFormData(prev => ({
                ...prev,
                keys: newKeys
            }))
        }
    }

    // 유효성 검사
    const validateForm = () => {
        console.log('폼 유효성 검사 시작')
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

        // 키 시퀀스 검사
        formData.keys.forEach((keyItem, index) => {
            if (!keyItem.key.trim()) {
                newErrors[`key_${index}`] = '키를 선택해주세요'
            }

            if (keyItem.delay < KEY_MAPPING.MIN_DELAY || keyItem.delay > KEY_MAPPING.MAX_DELAY) {
                newErrors[`delay_${index}`] = `딜레이는 ${KEY_MAPPING.MIN_DELAY}~${KEY_MAPPING.MAX_DELAY}ms 사이여야 합니다`
            }
        })

        console.log('유효성 검사 결과:', newErrors)
        setErrors(newErrors)
        return Object.keys(newErrors).length === 0
    }

    // 저장 처리
    const handleSave = async () => {
        console.log('키 맵핑 저장 시작')

        if (!validateForm()) {
            console.log('유효성 검사 실패, 저장 중단')
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

            console.log('저장할 맵핑 데이터:', mappingData)

            await onSave(mappingData)
        } catch (error) {
            console.error('키 맵핑 저장 실패:', error)
        } finally {
            setLoading(false)
        }
    }

    // 키 목록 로딩 상태 디버깅
    useEffect(() => {
        console.log('사용 가능한 키 목록 상태 변경:', availableKeys)
        console.log('키 카테고리 개수:', Object.keys(availableKeys).length)
    }, [availableKeys])

    // 모달이 열려있지 않으면 렌더링하지 않음
    if (!isOpen) return null

    const startKeyOptions = getStartKeyOptions()

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
                    {/* 키 로딩 상태 표시 */}
                    {keysLoading && (
                        <div style={{ padding: '10px', textAlign: 'center', color: 'var(--text-muted)' }}>
                            사용 가능한 키 목록을 불러오는 중...
                        </div>
                    )}

                    {/* 디버그 정보 표시 */}
                    {process.env.NODE_ENV === 'development' && (
                        <div style={{ marginBottom: '10px', padding: '10px', backgroundColor: 'var(--highlight)', borderRadius: '4px', fontSize: '12px' }}>
                            <strong>디버그 정보:</strong><br />
                            키 카테고리 수: {Object.keys(availableKeys).length}<br />
                            키 로딩 중: {keysLoading ? 'Yes' : 'No'}<br />
                            총 키 개수: {Object.values(availableKeys).flat().length}
                        </div>
                    )}

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
                                title="이 키를 누르면 키 시퀀스가 실행됩니다 (DELETE 또는 END만 사용 가능)"
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
                            {startKeyOptions.map(option => (
                                <option key={option.value} value={option.value}>
                                    {option.label}
                                </option>
                            ))}
                        </select>
                        {errors.start_key && <span className={styles.errorText}>{errors.start_key}</span>}
                        <small className={styles.formHelp}>
                            안전을 위해 DELETE와 END 키만 시작키로 사용할 수 있습니다.
                        </small>
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
                                            {Object.entries(availableKeys).map(([category, keys]) => {
                                                // 시작키 카테고리는 제외
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
                                            min={KEY_MAPPING.MIN_DELAY}
                                            max={KEY_MAPPING.MAX_DELAY}
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