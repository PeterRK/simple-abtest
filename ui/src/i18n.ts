import { ref } from 'vue'

interface MessageTree {
  [key: string]: string | MessageTree
}

export type Locale = 'zh-CN' | 'en-US'

const LOCALE_STORAGE_KEY = 'simple-abtest.locale'
const isLocale = (value: unknown): value is Locale => value === 'zh-CN' || value === 'en-US'

const loadStoredLocale = (): Locale => {
  if (typeof window === 'undefined') return 'zh-CN'
  try {
    const stored = window.localStorage.getItem(LOCALE_STORAGE_KEY)
    return isLocale(stored) ? stored : 'zh-CN'
  } catch {
    return 'zh-CN'
  }
}

const persistLocale = (nextLocale: Locale) => {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.setItem(LOCALE_STORAGE_KEY, nextLocale)
  } catch {
    // ignore storage failures and keep runtime locale working
  }
}

const messages: Record<Locale, MessageTree> = {
  'zh-CN': {
    app: {
      title: 'AB实验平台',
      experiments: '实验管理',
      verify: '在线验流',
      profile: '我的账号',
      accountSettings: '账号',
      login: '登录/注册',
      logout: '退出登录',
      language: '语言',
      langZh: '中文',
      langEn: 'English'
    },
    common: {
      id: 'ID',
      name: '名称',
      accessToken: 'Access Token',
      description: '描述',
      status: '状态',
      key: 'Key',
      create: '新增',
      update: '更新',
      delete: '删除',
      cancel: '取消',
      confirm: '确定',
      warning: '提示',
      detail: '详情',
      operation: '操作'
    },
    message: {
      failedLoadApps: '加载应用失败',
      failedLoadExperiments: '加载实验失败',
      failedLoadLayer: '加载实验层失败',
      failedLoadSegment: '加载流量段失败',
      failedLoadGroup: '加载实验组失败',
      failedLoadConfigs: '加载配置历史失败',
      failedLoadConfigContent: '加载配置内容失败',
      appCreated: '应用已创建',
      appUpdated: '应用已更新',
      appDeleted: '应用已删除',
      experimentCreated: '实验已创建',
      experimentUpdated: '实验已更新',
      experimentDeleted: '实验已删除',
      statusUpdated: '状态已更新',
      layerCreated: '实验层已创建',
      layerUpdated: '实验层已更新',
      layerDeleted: '实验层已删除',
      segmentCreated: '流量段已创建',
      segmentDeleted: '流量段已删除',
      segmentsRebalanced: '流量段比例已更新',
      groupCreated: '实验组已创建',
      groupUpdated: '实验组已更新',
      groupDeleted: '实验组已删除',
      shareUpdated: '分组流量已更新',
      segmentSeedShuffled: '流量桶已重新打散',
      shuffled: '实验流量已重新打散',
      updateFailed: '更新失败',
      createFailed: '创建失败',
      deleteFailed: '删除失败',
      operationFailed: '操作失败',
      updateFailedRefresh: '更新失败，请手动刷新后重试',
      rebalanceFailedRefresh: '调整失败，请手动刷新后重试',
      appVersionMissing: '应用版本缺失',
      appInfoMissing: '无法获取应用信息',
      appTokenMissing: '应用Access Token缺失，请刷新后重试',
      invalidTokenTTL: 'Access Token有效期必须大于 0',
      tokenIssued: 'Access Token已生成',
      issueTokenFailed: '申请Access Token失败',
      issueTokenForbidden: '仅应用管理员可以申请Access Token',
      verifyRequired: '应用ID和分流Key为必填项',
      verifyFailed: '验流失败',
      verifyContextInvalidJson: '上下文不是合法JSON',
      verifyContextKeyRequired: '上下文字段名不能为空',
      verifyContextKeyDuplicate: '上下文字段名不能重复',
      authRequired: '用户名和密码均不能为空',
      nameRequired: '名称不能为空',
      nameInvalid: '名称只能包含英文、数字、下划线和连接线',
      nameTooLong: '名称长度不能超过 {max} 个字符',
      loginSuccess: '登录成功',
      registerSuccess: '注册成功',
      userExists: '用户名已存在',
      loginFailed: '用户名或密码错误',
      inviteCodeRequired: '邀请码不能为空',
      inviteCodeInvalid: '邀请码错误',
      oldPasswordRequired: '原密码不能为空',
      passwordRequired: '新密码不能为空',
      passwordRule: '密码至少6位，且必须包含字母和数字',
      confirmPasswordRequired: '请再次输入密码',
      passwordMismatch: '两次输入的密码不一致',
      passwordUpdated: '密码已更新',
      userDeleted: '账号已注销',
      failedLoadPrivileges: '加载授权失败',
      privilegeUpdated: '授权已更新',
      privilegeUpdateForbidden: '仅应用管理员可以修改授权',
      invalidFilter: '过滤条件不合法',
      invalidExpr: '表达式结构非法',
      invalidLogicArgs: '逻辑算子参数不合法',
      invalidInArgs: 'IN算子参数不合法',
      invalidCompareArgs: '比较算子参数不合法',
      missingTargetGroup: '未找到目标组或默认组，请刷新后重试',
      invalidShareRange: '分组流量不合法，只能在 {min} ~ {max} 范围内调整',
      sumShareMust100: '流量占比总和必须为 100',
      invalidJsonFormat: '输入内容不是合法JSON，无法格式化'
    },
    confirm: {
      deleteApp: '确定删除该应用？',
      deleteExperiment: '确认删除该实验？',
      deleteLayer: '确认删除该实验层？',
      deleteSegment: '确认删除该流量段？请先确保该区间为空。',
      deleteGroup: '确认删除该实验组？',
      deleteUser: '确认注销当前账号？此操作不可恢复。',
      logout: '确认退出登录？'
    },
    list: {
      selectApp: '选择应用',
      selectAppFirst: '请先选择应用',
      createExperiment: '新增实验',
      appCreateTitle: '新增应用',
      appDetailTitle: '应用信息',
      issueAccessToken: '申请Access Token',
      experimentCreateTitle: '新增实验'
    },
    token: {
      ttlDays: '有效期天数',
      expireAt: '过期时间',
      issue: '申请'
    },
    detail: {
      expName: '实验名称',
      expDesc: '实验描述',
      filter: '过滤条件',
      createLayer: '新增实验层',
      appPrivilege: '授权',
      privilegeTitle: '应用授权管理',
      privilegeLevel: '权限',
      grantor: '授权人',
      targetUser: '用户名',
      grant: '授予',
      revoke: '撤销'
    },
    auth: {
      loginTitle: '登录',
      registerTitle: '注册',
      needLoginTip: '请先登录或注册后继续使用。',
      login: '登录',
      register: '注册',
      goRegister: '没有账号？去注册',
      goLogin: '已有账号？去登录',
      password: '密码',
      inviteCode: '邀请码',
      confirmPassword: '确认密码'
    },
    profile: {
      title: '账号信息',
      uid: '用户ID',
      changePassword: '修改密码',
      updatePassword: '更新密码',
      deleteUser: '注销账号'
    },
    settings: {
      title: '账号设置',
      oldPassword: '原密码',
      newPassword: '新密码',
      updatePassword: '修改密码',
      logout: '退出登录',
      deleteUser: '注销账号'
    },
    privilege: {
      none: '无权限',
      read: '只读',
      write: '读写',
      admin: '管理员'
    },
    verify: {
      title: '在线验流',
      application: '应用',
      key: '分流Key',
      context: '上下文',
      contextKey: '字段名',
      contextValue: '字段值',
      addContext: '新增字段',
      selectApp: '选择应用',
      keyPlaceholder: '用户ID/设备ID',
      button: '开始验流',
      result: '命中结果',
      resultEmpty: '验流结果会显示在这里'
    },
    filter: {
      empty: '无过滤条件',
      addRoot: '新增根算子',
      opType: '算子类型',
      paramType: '参数类型',
      param: '参数',
      addChild: '增加子节点',
      opAnd: '且',
      opOr: '或',
      opNot: '非',
      opIn: '属于',
      opNotIn: '不属于',
      dtypeString: '字符串',
      dtypeInt: '整数',
      dtypeFloat: '浮点数'
    },
    layer: {
      fallbackName: '实验层 {index}',
      namePlaceholder: '实验层名称',
      rename: '改名',
      addSegment: '新增流量段',
      rebalanceSegment: '调整流量段比例',
      createTitle: '新增实验层',
      rebalanceTitle: '调整流量段比例',
      percent: '占比',
      begin: '起点',
      end: '终点',
      sharePercent: '流量占比'
    },
    group: {
      rebalance: '调整分组流量',
      createGroup: '新增实验组',
      shuffle: '流量打散',
      groupName: '实验组名称',
      formatInput: '格式化JSON',
      searchConfig: '查询历史配置',
      dayAgo: '天前',
      forceHitPlaceholder: '强制命中Key，每行一个',
      configPlaceholder: '配置内容',
      configId: '配置ID',
      updateTime: '创建时间',
      createTitle: '新增实验组',
      rebalanceTitle: '调整分组流量'
    }
  },
  'en-US': {
    app: {
      title: 'A/B Test Platform',
      experiments: 'Experiments',
      verify: 'Traffic Check',
      profile: 'My Account',
      accountSettings: 'Account',
      login: 'Login/Register',
      logout: 'Log out',
      language: 'Language',
      langZh: '中文',
      langEn: 'English'
    },
    common: {
      id: 'ID',
      name: 'Name',
      accessToken: 'Access Token',
      description: 'Description',
      status: 'Status',
      key: 'Key',
      create: 'Create',
      update: 'Update',
      delete: 'Delete',
      cancel: 'Cancel',
      confirm: 'Confirm',
      warning: 'Warning',
      detail: 'Details',
      operation: 'Operation'
    },
    message: {
      failedLoadApps: 'Failed to load apps',
      failedLoadExperiments: 'Failed to load experiments',
      failedLoadLayer: 'Failed to load layer',
      failedLoadSegment: 'Failed to load segment',
      failedLoadGroup: 'Failed to load group',
      failedLoadConfigs: 'Failed to load config history',
      failedLoadConfigContent: 'Failed to load config content',
      appCreated: 'App created',
      appUpdated: 'App updated',
      appDeleted: 'App deleted',
      experimentCreated: 'Experiment created',
      experimentUpdated: 'Experiment updated',
      experimentDeleted: 'Experiment deleted',
      statusUpdated: 'Status updated',
      layerCreated: 'Layer created',
      layerUpdated: 'Layer updated',
      layerDeleted: 'Layer deleted',
      segmentCreated: 'Segment created',
      segmentDeleted: 'Segment deleted',
      segmentsRebalanced: 'Segment ratios updated',
      groupCreated: 'Group created',
      groupUpdated: 'Group updated',
      groupDeleted: 'Group deleted',
      shareUpdated: 'Traffic split updated',
      segmentSeedShuffled: 'Traffic buckets reshuffled',
      shuffled: 'Traffic reshuffled',
      updateFailed: 'Update failed',
      createFailed: 'Create failed',
      deleteFailed: 'Delete failed',
      operationFailed: 'Operation failed',
      updateFailedRefresh: 'Update failed, please refresh and retry',
      rebalanceFailedRefresh: 'Rebalance failed, please refresh and retry',
      appVersionMissing: 'App version is missing',
      appInfoMissing: 'Unable to resolve app info',
      appTokenMissing: 'App access token is missing. Refresh and retry',
      invalidTokenTTL: 'Access token TTL must be greater than 0',
      tokenIssued: 'Access token created',
      issueTokenFailed: 'Failed to issue access token',
      issueTokenForbidden: 'Only app admins can issue access tokens',
      verifyRequired: 'App ID and Key are required',
      verifyFailed: 'Traffic check failed',
      verifyContextInvalidJson: 'Context is not valid JSON',
      verifyContextKeyRequired: 'Context field name is required',
      verifyContextKeyDuplicate: 'Context field names must be unique',
      authRequired: 'Both username and password are required',
      nameRequired: 'Name is required',
      nameInvalid: 'Name may contain only letters, numbers, underscores, and hyphens',
      nameTooLong: 'Name must be at most {max} characters',
      loginSuccess: 'Login succeeded',
      registerSuccess: 'Registration succeeded',
      userExists: 'Username already exists',
      loginFailed: 'Invalid username or password',
      inviteCodeRequired: 'Invite code is required',
      inviteCodeInvalid: 'Invalid invite code',
      oldPasswordRequired: 'Current password is required',
      passwordRequired: 'New password is required',
      passwordRule: 'Password must be at least 6 characters and include letters and numbers',
      confirmPasswordRequired: 'Please confirm your password',
      passwordMismatch: 'Passwords do not match',
      passwordUpdated: 'Password updated',
      userDeleted: 'Account deleted',
      failedLoadPrivileges: 'Failed to load privileges',
      privilegeUpdated: 'Access updated',
      privilegeUpdateForbidden: 'Only app admins can modify privileges',
      invalidFilter: 'Invalid filter',
      invalidExpr: 'Invalid expression structure',
      invalidLogicArgs: 'Invalid logical operator arguments',
      invalidInArgs: 'Invalid IN operator arguments',
      invalidCompareArgs: 'Invalid compare operator arguments',
      missingTargetGroup: 'Target group or default group not found, please refresh',
      invalidShareRange: 'Invalid split. Allowed range: {min} ~ {max}',
      sumShareMust100: 'The total share percentage must be 100',
      invalidJsonFormat: 'The input is not valid JSON and cannot be formatted'
    },
    confirm: {
      deleteApp: 'Delete this app?',
      deleteExperiment: 'Delete this experiment?',
      deleteLayer: 'Delete this layer?',
      deleteSegment: 'Delete this segment? The range must be empty.',
      deleteGroup: 'Delete this group?',
      deleteUser: 'Delete your account? This action cannot be undone.',
      logout: 'Log out now?'
    },
    list: {
      selectApp: 'Select App',
      selectAppFirst: 'Please select an app first',
      createExperiment: 'New Experiment',
      appCreateTitle: 'New App',
      appDetailTitle: 'App Info',
      issueAccessToken: 'Issue Access Token',
      experimentCreateTitle: 'New Experiment'
    },
    token: {
      ttlDays: 'TTL Days',
      expireAt: 'Expires At',
      issue: 'Issue'
    },
    detail: {
      expName: 'Experiment Name',
      expDesc: 'Description',
      filter: 'Filter',
      createLayer: 'New Layer',
      appPrivilege: 'Permissions',
      privilegeTitle: 'App Permissions',
      privilegeLevel: 'Permission',
      grantor: 'Grantor',
      targetUser: 'Username',
      grant: 'Grant',
      revoke: 'Revoke'
    },
    auth: {
      loginTitle: 'Login',
      registerTitle: 'Register',
      needLoginTip: 'Please login or register first.',
      login: 'Login',
      register: 'Register',
      goRegister: 'No account? Register',
      goLogin: 'Already have an account? Login',
      password: 'Password',
      inviteCode: 'Invite Code',
      confirmPassword: 'Confirm Password'
    },
    profile: {
      title: 'Account Info',
      uid: 'User ID',
      changePassword: 'Change Password',
      updatePassword: 'Update Password',
      deleteUser: 'Delete Account'
    },
    settings: {
      title: 'Account Settings',
      oldPassword: 'Current Password',
      newPassword: 'New Password',
      updatePassword: 'Change Password',
      logout: 'Log out',
      deleteUser: 'Delete Account'
    },
    privilege: {
      none: 'No Access',
      read: 'Read-Only',
      write: 'Read/Write',
      admin: 'Admin'
    },
    verify: {
      title: 'Traffic Check',
      application: 'APP',
      key: 'Key',
      context: 'Context',
      contextKey: 'Field',
      contextValue: 'Value',
      addContext: 'Add Field',
      selectApp: 'Select',
      keyPlaceholder: 'User ID / Device ID',
      button: 'Run',
      result: 'Hit Result',
      resultEmpty: 'Results appear here'
    },
    filter: {
      empty: 'No filter condition',
      addRoot: 'Add Root Operator',
      opType: 'Operator',
      paramType: 'Data Type',
      param: 'Value',
      addChild: 'Add Child',
      opAnd: 'AND',
      opOr: 'OR',
      opNot: 'NOT',
      opIn: 'IN',
      opNotIn: 'NOT IN',
      dtypeString: 'String',
      dtypeInt: 'Int',
      dtypeFloat: 'Float'
    },
    layer: {
      fallbackName: 'Layer {index}',
      namePlaceholder: 'Layer name',
      rename: 'Rename',
      addSegment: 'New Segment',
      rebalanceSegment: 'Rebalance Segments',
      createTitle: 'New Layer',
      rebalanceTitle: 'Rebalance Segments',
      percent: 'Percent',
      begin: 'Begin',
      end: 'End',
      sharePercent: 'Share Percentage'
    },
    group: {
      rebalance: 'Adjust Split',
      createGroup: 'New Group',
      shuffle: 'Reshuffle',
      groupName: 'Group name',
      formatInput: 'Format JSON',
      searchConfig: 'View History',
      dayAgo: 'days ago',
      forceHitPlaceholder: 'Force-hit keys, one per line',
      configPlaceholder: 'Config content',
      configId: 'Config ID',
      updateTime: 'Created At',
      createTitle: 'New Group',
      rebalanceTitle: 'Adjust Split'
    }
  }
}

export const locale = ref<Locale>(loadStoredLocale())

const getByPath = (obj: MessageTree, path: string): string | MessageTree | undefined => {
  const parts = path.split('.')
  let current: string | MessageTree | undefined = obj
  for (const part of parts) {
    if (!current || typeof current === 'string') return undefined
    current = current[part]
  }
  return current
}

const formatMessage = (message: string, params?: Record<string, string | number>) => {
  if (!params) return message
  return message.replace(/\{(\w+)\}/g, (_, key: string) => String(params[key] ?? ''))
}

export const t = (key: string, params?: Record<string, string | number>) => {
  const activeMessages = messages[locale.value]
  const value = getByPath(activeMessages, key)
  if (typeof value === 'string') return formatMessage(value, params)
  return key
}

export const setLocale = (nextLocale: Locale) => {
  locale.value = nextLocale
  persistLocale(nextLocale)
}

export const useI18n = () => ({
  locale,
  setLocale,
  t
})
