import { ref } from 'vue'

interface MessageTree {
  [key: string]: string | MessageTree
}

export type Locale = 'zh-CN' | 'en-US'

const messages: Record<Locale, MessageTree> = {
  'zh-CN': {
    app: {
      title: 'AB 测试平台',
      experiments: '实验',
      verify: '在线验证',
      language: '语言',
      langZh: '中文',
      langEn: 'English'
    },
    common: {
      id: 'ID',
      name: '名称',
      description: '描述',
      status: '状态',
      key: 'Key',
      create: '新增',
      update: '更新',
      delete: '删除',
      cancel: '取消',
      confirm: '确定',
      warning: '提示',
      detail: '详情'
    },
    message: {
      failedLoadApps: '加载应用失败',
      failedLoadExperiments: '加载实验失败',
      appCreated: '应用已创建',
      appUpdated: '应用已更新',
      appDeleted: '应用已删除',
      experimentCreated: '实验已创建',
      experimentUpdated: '实验已更新',
      experimentDeleted: '实验已删除',
      statusUpdated: '状态已更新',
      layerCreated: 'Layer 已创建',
      layerUpdated: 'Layer 已更新',
      layerDeleted: 'Layer 已删除',
      segmentCreated: 'Segment 已创建',
      segmentDeleted: 'Segment 已删除',
      segmentsRebalanced: 'Segment 流量已调整',
      groupCreated: 'Group 已创建',
      groupUpdated: 'Group 已更新',
      groupDeleted: 'Group 已删除',
      shareUpdated: '流量已更新',
      segmentSeedShuffled: 'Segment seed 已打散',
      shuffled: '流量已打散',
      updateFailed: '更新失败',
      createFailed: '创建失败',
      deleteFailed: '删除失败',
      operationFailed: '操作失败',
      updateFailedRefresh: '更新失败，请手动刷新后重试',
      rebalanceFailedRefresh: '调整失败，请手动刷新后重试',
      appVersionMissing: '应用版本缺失',
      appInfoMissing: '无法获取应用信息',
      verifyRequired: 'App ID 和 Key 为必填项',
      verifyFailed: '验证失败或 JSON 非法',
      invalidFilter: '过滤条件不合法',
      invalidExpr: '表达式结构非法',
      invalidLogicArgs: '逻辑算子参数不合法',
      invalidInArgs: 'IN 算子参数不合法',
      invalidCompareArgs: '比较算子参数不合法',
      missingTargetGroup: '未找到目标组或默认组，请刷新后重试',
      invalidShareRange: '流量不合法，仅可在 {min} ~ {max} 范围内调整',
      sumShareMust100: '流量百分比总和需为 100'
    },
    confirm: {
      deleteApp: '确定删除该应用？',
      deleteExperiment: '确认删除该实验？',
      deleteLayer: '确认删除该 Layer？',
      deleteSegment: '确认删除该 Segment？需先确保区间为空。',
      deleteGroup: '确认删除该 Group？'
    },
    list: {
      selectApp: '选择应用',
      createExperiment: '新增实验',
      appCreateTitle: '新增应用',
      appDetailTitle: '应用详情',
      experimentCreateTitle: '新增实验'
    },
    detail: {
      expName: '实验名',
      expDesc: '实验描述',
      filter: '过滤条件',
      createLayer: '新增 Layer'
    },
    verify: {
      title: '在线验证',
      application: '应用',
      key: 'Key',
      context: '上下文',
      selectApp: '选择应用',
      keyPlaceholder: '用户 ID 或设备 ID',
      contextPlaceholder: 'JSON 格式，例如 {"country":"CN"}',
      button: '验证',
      result: '结果'
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
      fallbackName: 'Layer {index}',
      namePlaceholder: '层名',
      rename: '改名',
      addSegment: '新增 Segment',
      rebalanceSegment: '调整 Segment 流量',
      createTitle: '新增 Layer',
      rebalanceTitle: '调整 Segment 流量',
      percent: '占比',
      begin: '开始',
      end: '结束',
      sharePercent: '流量百分比'
    },
    group: {
      rebalance: '扩缩容',
      createGroup: '新增 Group',
      shuffle: '流量打散',
      groupName: '组名',
      searchConfig: '配置查找',
      dayAgo: '天前',
      forceHitPlaceholder: '强制命中 key，每行一个',
      configPlaceholder: '配置内容',
      configId: '配置 ID',
      updateTime: '更新时间',
      createTitle: '新增 Group',
      rebalanceTitle: '扩缩容'
    }
  },
  'en-US': {
    app: {
      title: 'AB Testing Platform',
      experiments: 'Experiments',
      verify: 'Verify',
      language: 'Language',
      langZh: '中文',
      langEn: 'English'
    },
    common: {
      id: 'ID',
      name: 'Name',
      description: 'Description',
      status: 'Status',
      key: 'Key',
      create: 'Create',
      update: 'Update',
      delete: 'Delete',
      cancel: 'Cancel',
      confirm: 'Confirm',
      warning: 'Warning',
      detail: 'Detail'
    },
    message: {
      failedLoadApps: 'Failed to load apps',
      failedLoadExperiments: 'Failed to load experiments',
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
      segmentsRebalanced: 'Segments rebalanced',
      groupCreated: 'Group created',
      groupUpdated: 'Group updated',
      groupDeleted: 'Group deleted',
      shareUpdated: 'Share updated',
      segmentSeedShuffled: 'Segment seed shuffled',
      shuffled: 'Shuffled',
      updateFailed: 'Update failed',
      createFailed: 'Create failed',
      deleteFailed: 'Delete failed',
      operationFailed: 'Operation failed',
      updateFailedRefresh: 'Update failed, please refresh and retry',
      rebalanceFailedRefresh: 'Rebalance failed, please refresh and retry',
      appVersionMissing: 'App version is missing',
      appInfoMissing: 'Unable to resolve app info',
      verifyRequired: 'App ID and Key are required',
      verifyFailed: 'Verification failed or invalid JSON',
      invalidFilter: 'Invalid filter condition',
      invalidExpr: 'Invalid expression structure',
      invalidLogicArgs: 'Invalid logical operator arguments',
      invalidInArgs: 'Invalid IN operator arguments',
      invalidCompareArgs: 'Invalid compare operator arguments',
      missingTargetGroup: 'Target group or default group not found, please refresh',
      invalidShareRange: 'Invalid share. Allowed range: {min} ~ {max}',
      sumShareMust100: 'The total share percentage must be 100'
    },
    confirm: {
      deleteApp: 'Delete this app?',
      deleteExperiment: 'Delete this experiment?',
      deleteLayer: 'Delete this layer?',
      deleteSegment: 'Delete this segment? The range must be empty.',
      deleteGroup: 'Delete this group?'
    },
    list: {
      selectApp: 'Select App',
      createExperiment: 'New Experiment',
      appCreateTitle: 'New App',
      appDetailTitle: 'App Detail',
      experimentCreateTitle: 'New Experiment'
    },
    detail: {
      expName: 'Experiment Name',
      expDesc: 'Experiment Description',
      filter: 'Filter',
      createLayer: 'New Layer'
    },
    verify: {
      title: 'Online Verify',
      application: 'Application',
      key: 'Key',
      context: 'Context',
      selectApp: 'Select App',
      keyPlaceholder: 'User ID or Device ID',
      contextPlaceholder: 'JSON format, e.g. {"country":"US"}',
      button: 'Verify',
      result: 'Result'
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
      rebalance: 'Rebalance',
      createGroup: 'New Group',
      shuffle: 'Shuffle',
      groupName: 'Group name',
      searchConfig: 'Find Config',
      dayAgo: 'days ago',
      forceHitPlaceholder: 'Force-hit keys, one per line',
      configPlaceholder: 'Config content',
      configId: 'Config ID',
      updateTime: 'Updated At',
      createTitle: 'New Group',
      rebalanceTitle: 'Rebalance'
    }
  }
}

export const locale = ref<Locale>('zh-CN')

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
}

export const useI18n = () => ({
  locale,
  setLocale,
  t
})
