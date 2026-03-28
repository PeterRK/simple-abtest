const NAME_PATTERN = /^[A-Za-z0-9_-]+$/

export type NameKind = 'app' | 'user' | 'experiment' | 'layer' | 'group'

const NAME_LIMITS: Record<NameKind, number> = {
  app: 64,
  user: 64,
  experiment: 32,
  layer: 32,
  group: 32
}

export const getNameMaxLength = (kind: NameKind) => NAME_LIMITS[kind]

export const validateName = (value: string, kind: NameKind) => {
  const trimmed = value.trim()
  const max = NAME_LIMITS[kind]
  if (!trimmed) {
    return { valid: false, normalized: trimmed, max, messageKey: 'message.nameRequired' }
  }
  if (trimmed.length > max) {
    return { valid: false, normalized: trimmed, max, messageKey: 'message.nameTooLong' }
  }
  if (!NAME_PATTERN.test(trimmed)) {
    return { valid: false, normalized: trimmed, max, messageKey: 'message.nameInvalid' }
  }
  return { valid: true, normalized: trimmed, max, messageKey: '' }
}
