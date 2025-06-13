// KeyMappingModal.jsx - 조합키 지원 및 중복키 허용
import { useState, useEffect } from 'react'
import { X, Plus, Trash2, HelpCircle, Keyboard, AlertTriangle, Info } from 'lucide-react'
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
    const [comboKeyExamples, setComboKeyExamples] = useState({})
    const [errors, setErrors] = useState({})
    const [loading, setLoading] = useState(false)
    const [keysLoading, setKeysLoading] = useState(false)
    const [showComboKeyHelp, setShowComboKeyHelp] = useState(false)
    const [isDuplicateKey, setIsDuplicateKey] = useState(false)

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
            setIsDuplicateKey(false)
        }
    }, [isOpen, editingMapping])

    // 시작키 변경 감지하여 중복 체크
    useEffect(() => {
        if (formData.start_key && existingStartKeys.includes(formData.start_key)) {
            if (!editingMapping || editingMapping.start_key !== formData.start_key) {
                setIsDuplicateKey(true)
            } else {
                setIsDuplicateKey(false)
            }
        } else {
            setIsDuplicateKey(false)
        }
    }, [formData.start_key, existingStartKeys, editingMapping])

    // 사용 가능한 키 목록 로드
    const loadAvailableKeys = async () => {
        try {
            console.log('사용 가능한 키 목록 로드 시작...')
            setKeysLoading(true)

            const data = await keyMappingService.getAvailableKeys()
            console.log('키 목록 API 응답:', data)

            if (data && data.success && data.keys) {
                console.log('사용 가능한 키 목록:', data.keys)
                setAvailableKeys(data.keys)

                // 조합키 예시 설정
                if (data.keys['조합키 예시']) {
                    const examples = data.keys['조합키 예시']
                    setComboKeyExamples({
                        '복사/붙여넣기': examples.filter(k => k.includes('ctrl+') && ['c', 'v', 'x', 'z', 'y'].some(c => k.includes(c))),
                        '창 관리': examples.filter(k => k.includes('alt+') || k.includes('win+')),
                        '기타': examples.filter(k => !k.includes('ctrl+c') && !k.includes('ctrl+v') && !k.includes('alt+') && !k.includes('win+'))
                    })
                }
            } else {
                console.error('키 목록 로드 실패 또는 잘못된 응답:', data)
                setDefaultKeys()
            }
        } catch (error) {
            console.error('사용 가능한 키 목록 로드 실패:', error)
            setDefaultKeys()
        } finally {
            setKeysLoading(false)
        }
    }

    // 기본 키 목록 설정
    const setDefaultKeys = () => {
        const defaultKeys = {
            "숫자 키": ["1", "2", "3", "4", "5", "6", "7", "8", "9", "0"],
            "알파벳 키": ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"],
            "펑션 키": ["f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9", "f10", "f11", "f12"],
            "특수 키": ["space", "enter", "esc", "tab", "backspace", "delete"],
            "화살표 키": ["left", "up", "right", "down"],
            "수정키": ["shift", "ctrl", "alt"],
            "조합키 예시": ["ctrl+c", "ctrl+v", "ctrl+x", "alt+tab", "win+r"]
        }
        setAvailableKeys(defaultKeys)
        setComboKeyExamples({
            '복사/붙여넣기': ['ctrl+c', 'ctrl+v', 'ctrl+x'],
            '창 관리': ['alt+tab', 'win+r', 'win+d'],
            '기타': ['ctrl+z', 'ctrl+y', 'alt+f4']
        })
    }

    // 시작키용 키 목록 필터링
    const getStartKeyOptions = () => {
        return allowedStartKeys.map(key => ({
            value: key,
            label: key.toUpperCase()
        }))
    }

    // 조합키인지 확인
    const isComboKey = (key) => {
        return key && (
            key.includes('ctrl+') ||
            key.includes('shift+') ||
            key.includes('alt+') ||
            key.includes('cmd+') ||
            key.includes('win+')
        )
    }

    // 조합키 유효성 검사
    const validateComboKey = (key) => {
        if (!isComboKey(key)) return null

        const modifiers = ['ctrl+', 'shift+', 'alt+', 'cmd+', 'win+']
        let hasModifier = false
        let mainKey = key.toLowerCase()

        for (const modifier of modifiers) {
            if (mainKey.includes(modifier)) {
                hasModifier = true
                mainKey = mainKey.replace(modifier, '')
            }
        }

        if (!hasModifier) {
            return '조합키에는 최소 하나의 수정키가 필요합니다'
        }

        if (!mainKey.trim()) {
            return '조합키에는 메인키가 필요합니다'
        }

        return null
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
    // 키 시퀀스 변경 처리 (수정)
    const handleKeyChange = (index, field, value) => {
        console.log(`키 시퀀스 변경: [${index}].${field} = ${value}`)
        const newKeys = [...formData.keys]

        if (field === 'delay') {
            let delayValue = 0

            if (value === '' || value === null || value === undefined) {
                delayValue = 0
            } else if (typeof value === 'string') {
                if (value.trim() === '') {
                    delayValue = 0
                } else {
                    const parsed = parseInt(value.replace(/[^0-9]/g, ''))
                    delayValue = isNaN(parsed) ? 0 : parsed
                }
            } else {
                delayValue = parseInt(value) || 0
            }

            delayValue = Math.max(KEY_MAPPING.MIN_DELAY, Math.min(delayValue, KEY_MAPPING.MAX_DELAY))

            newKeys[index] = {
                ...newKeys[index],
                delay: delayValue
            }
        } else if (field === 'key') {
            // 키 입력 처리 개선
            const inputValue = value || ''
            const trimmedValue = inputValue.trim().toLowerCase()

            newKeys[index] = {
                ...newKeys[index],
                key: trimmedValue
            }

            // 조합키 유효성 검사
            if (isComboKey(trimmedValue) && trimmedValue !== '') {
                const comboError = validateComboKey(trimmedValue)
                if (comboError) {
                    setErrors(prev => ({
                        ...prev,
                        [`key_${index}`]: comboError
                    }))
                } else {
                    setErrors(prev => ({
                        ...prev,
                        [`key_${index}`]: null
                    }))
                }
            } else {
                // 에러 제거
                setErrors(prev => ({
                    ...prev,
                    [`key_${index}`]: null
                }))
            }
        } else {
            newKeys[index] = {
                ...newKeys[index],
                [field]: value
            }
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

    // 조합키 예시 적용
    const applyComboKeyExample = (key) => {
        const lastIndex = formData.keys.length - 1
        handleKeyChange(lastIndex, 'key', key)
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
            }
        }

        // 키 시퀀스 검사
        formData.keys.forEach((keyItem, index) => {
            if (!keyItem.key.trim()) {
                newErrors[`key_${index}`] = '키를 입력해주세요'
            } else {
                // 조합키 유효성 검사
                if (isComboKey(keyItem.key)) {
                    const comboError = validateComboKey(keyItem.key)
                    if (comboError) {
                        newErrors[`key_${index}`] = comboError
                    }
                }
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

                    {/* 기본 정보 */}
                    <div className={styles.formGroup}>
                        <label className={styles.label}>
                            맵핑 이름 *
                        </label>
                        <input
                            type="text"
                            value={formData.name}
                            onChange={(e) => handleInputChange('name', e.target.value)}
                            placeholder="예: 도사 스킬, 천인 콤보, 마법사 버프"
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

                        {/* 중복키 경고 */}
                        {isDuplicateKey && (
                            <div className={styles.duplicateWarning}>
                                <AlertTriangle size={16} />
                                <span>
                                    이미 이 시작키를 사용하는 맵핑이 있습니다.
                                    같은 시작키의 맵핑은 하나만 활성화할 수 있습니다.
                                </span>
                            </div>
                        )}

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
                                onClick={() => setShowComboKeyHelp(!showComboKeyHelp)}
                                title="조합키 사용법 보기"
                            >
                                <Keyboard size={14} />
                            </button>
                        </label>

                        {/* 조합키 도움말 */}
                        {showComboKeyHelp && (
                            <div className={styles.comboKeyHelp}>
                                <h4><Keyboard size={16} /> 조합키 사용법</h4>
                                <div className={styles.comboKeyExamples}>
                                    {Object.entries(comboKeyExamples).map(([category, examples]) => (
                                        <div key={category} className={styles.exampleCategory}>
                                            <h5>{category}</h5>
                                            <div className={styles.exampleKeys}>
                                                {examples.map(key => (
                                                    <button
                                                        key={key}
                                                        type="button"
                                                        className={styles.exampleKey}
                                                        onClick={() => applyComboKeyExample(key)}
                                                        title={`${key} 적용`}
                                                    >
                                                        {key}
                                                    </button>
                                                ))}
                                            </div>
                                        </div>
                                    ))}
                                </div>
                                <p className={styles.comboKeyNote}>
                                    <Info size={14} />
                                    조합키는 "수정키+메인키" 형태로 입력하세요.
                                    예: ctrl+c, alt+tab, win+r
                                </p>
                            </div>
                        )}

                        <div className={styles.keySequence}>
                            {formData.keys.map((keyItem, index) => (
                                <div key={index} className={`${styles.keyRow} keyRow`}>
                                    <div className={styles.keyIndex}>{index + 1}</div>

                                    <div className={styles.keyInputGroup}>
                                        <input
                                            type="text"
                                            value={keyItem.key}
                                            onChange={(e) => handleKeyChange(index, 'key', e.target.value)}
                                            placeholder="키 입력 (예: x, ctrl+c, alt+tab)"
                                            className={`${styles.keyInput} ${errors[`key_${index}`] ? styles.error : ''}`}
                                        />
                                        {isComboKey(keyItem.key) && (
                                            <div className={styles.comboKeyIndicator}>
                                                <Keyboard size={12} />
                                                조합키
                                            </div>
                                        )}
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
                                            value={keyItem.delay === 0 ? '' : keyItem.delay}
                                            onChange={(e) => handleKeyChange(index, 'delay', e.target.value)}
                                            onFocus={(e) => {
                                                setTimeout(() => e.target.select(), 10)
                                            }}
                                            onBlur={(e) => {
                                                if (e.target.value === '') {
                                                    handleKeyChange(index, 'delay', 0)
                                                }
                                            }}
                                            className={`${styles.delayInput} ${errors[`delay_${index}`] ? styles.error : ''}`}
                                            placeholder="0"
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
                            딜레이 범위: 0ms~1000ms (0ms = 딜레이 없음) |
                            조합키 지원: ctrl+키, alt+키, shift+키, win+키
                        </small>
                    </div>

                    {/* 미리보기 */}
                    <div className={styles.preview}>
                        <h4>미리보기</h4>
                        <div className={styles.previewContent}>
                            <strong>{formData.start_key.toUpperCase()}</strong> 키를 누르면 → {' '}
                            {formData.keys.map((key, index) => (
                                <span key={index} className={styles.previewKey}>
                                    <strong className={isComboKey(key.key) ? styles.comboKey : ''}>
                                        {key.key.toUpperCase()}
                                    </strong>
                                    {key.delay > 0 && <span className={styles.delay}>({key.delay}ms)</span>}
                                    {key.delay === 0 && <span className={styles.delay}>(즉시)</span>}
                                    {index < formData.keys.length - 1 && ' → '}
                                </span>
                            ))}
                        </div>

                        {/* 중복키 정보 표시 */}
                        {isDuplicateKey && (
                            <div className={styles.duplicateInfo}>
                                <Info size={14} />
                                <span>
                                    중복키 허용: 같은 시작키의 맵핑 중 하나만 활성화됩니다.
                                </span>
                            </div>
                        )}
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
