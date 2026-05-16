<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { Back, Search } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useRoute, useRouter } from 'vue-router'
import { getExpResultData, getExpResultOptions } from '@/api'
import type { ResultDataPoint, ResultOptionLayer } from '@/types'
import { useI18n } from '@/i18n'

type ChartPoint = ResultDataPoint & {
  x: number
  y: number
}

type ChartSeries = {
  name: string
  color: string
  path: string
  points: ChartPoint[]
}

type AxisLabel = {
  text: string
  x?: number
  y?: number
}

type SampleSummary = {
  count: number
  mean: number
  variance: number
}

type PairedSamples = {
  control: SampleSummary
  treatment: SampleSummary
  differences: number[]
}

const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const chartWidth = 720
const chartHeight = 320
const chartPadding = {
  top: 22,
  right: 24,
  bottom: 44,
  left: 64
}
const plotWidth = chartWidth - chartPadding.left - chartPadding.right
const plotHeight = chartHeight - chartPadding.top - chartPadding.bottom
const seriesColors = ['#2563eb', '#16a34a', '#dc2626', '#d97706', '#7c3aed', '#0891b2', '#be123c', '#4d7c0f']
const hourMillis = 60 * 60 * 1000

const parsePositiveParam = (value: string | string[] | undefined) => {
  const raw = Array.isArray(value) ? value[0] : value
  const parsed = Number(raw)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null
}

const startOfHour = (value: number) => Math.floor(value / hourMillis) * hourMillis
const normalizeDateToHour = (value: Date | null | undefined) => {
  return value instanceof Date ? new Date(startOfHour(value.getTime())) : null
}
const disabledSubHourUnits = () => Array.from({ length: 59 }, (_, index) => index + 1)

const appId = computed(() => parsePositiveParam(route.params.appId))
const expId = computed(() => parsePositiveParam(route.params.expId))
const initialHour = startOfHour(Date.now())
const beginTime = ref<Date | null>(new Date(initialHour - 7 * 24 * hourMillis))
const endTime = ref<Date | null>(new Date(initialHour))
const optionLayers = ref<ResultOptionLayer[]>([])
const selectedLayerName = ref('')
const selectedBucketType = ref('')
const selectedMetricName = ref('')
const selectedControlGroupName = ref('')
const selectedTreatmentGroupName = ref('')
const points = ref<ResultDataPoint[]>([])
const loadingOptions = ref(false)
const loadingData = ref(false)

const selectedLayer = computed(() => optionLayers.value.find(layer => layer.name === selectedLayerName.value))
const bucketOptions = computed(() => selectedLayer.value?.bucket_types || [])
const selectedBucket = computed(() => bucketOptions.value.find(bucket => bucket.name === selectedBucketType.value))
const metricOptions = computed(() => selectedBucket.value?.metrics || [])
const hasOptions = computed(() => optionLayers.value.length > 0)
const toUnixSeconds = (value: Date | null | undefined) => value instanceof Date ? Math.floor(startOfHour(value.getTime()) / 1000) : null
const beginStamp = computed(() => toUnixSeconds(beginTime.value))
const endStamp = computed(() => toUnixSeconds(endTime.value))
const isRangeValid = computed(() => {
  const begin = beginStamp.value
  const end = endStamp.value
  return begin != null && end != null && Number.isFinite(begin) && Number.isFinite(end) && begin < end
})
const canQuery = computed(
  () => !!appId.value && !!expId.value && !!selectedLayerName.value && !!selectedBucketType.value && !!selectedMetricName.value && isRangeValid.value
)

const compareResultPointOrder = (left: ResultDataPoint, right: ResultDataPoint) =>
  left.bucket_stamp - right.bucket_stamp || left.bucket_key.localeCompare(right.bucket_key)

const sortedPoints = computed(() =>
  [...points.value].sort((left, right) =>
    compareResultPointOrder(left, right) || left.group_name.localeCompare(right.group_name)
  )
)

const yBounds = computed(() => {
  const values = sortedPoints.value.map(point => point.metric_value).filter(Number.isFinite)
  if (values.length === 0) return { min: 0, max: 1 }
  const min = Math.min(...values)
  const max = Math.max(...values)
  if (min === max) {
    const pad = Math.max(Math.abs(max) * 0.1, 1)
    return { min: min - pad, max: max + pad }
  }
  const pad = (max - min) * 0.08
  return { min: min - pad, max: max + pad }
})

const formatMetric = (value: number) => {
  if (!Number.isFinite(value)) return '-'
  const abs = Math.abs(value)
  if (abs >= 1000000 || (abs > 0 && abs < 0.0001)) return value.toExponential(3)
  return value.toLocaleString(undefined, { maximumFractionDigits: 6 })
}

const formatNullableMetric = (value: number | null | undefined) => {
  return value == null ? '-' : formatMetric(value)
}

const formatSignedPercent = (value: number | null) => {
  if (value == null || !Number.isFinite(value)) return '-'
  const percent = value * 100
  const prefix = percent > 0 ? '+' : ''
  return `${prefix}${percent.toLocaleString(undefined, { maximumFractionDigits: 2 })}%`
}

const formatPercent = (value: number | null) => {
  if (value == null || !Number.isFinite(value)) return '-'
  return `${(value * 100).toLocaleString(undefined, { maximumFractionDigits: 2 })}%`
}

const formatLiftWithMargin = (value: number | null, margin: number | null) => {
  const formattedValue = formatSignedPercent(value)
  if (formattedValue === '-' || margin == null || !Number.isFinite(margin)) return formattedValue
  return `${formattedValue}(±${formatPercent(Math.abs(margin))})`
}

const isLiftRangeSignificant = (value: number | null, margin: number | null) => {
  if (value == null || margin == null || !Number.isFinite(value) || !Number.isFinite(margin)) return null
  const lower = value - Math.abs(margin)
  const upper = value + Math.abs(margin)
  return lower > 0 || upper < 0
}

const summarizeSamples = (samples: number[]): SampleSummary | null => {
  const finiteSamples = samples.filter(Number.isFinite)
  if (finiteSamples.length === 0) return null
  const mean = finiteSamples.reduce((sum, value) => sum + value, 0) / finiteSamples.length
  const variance = finiteSamples.length > 1
    ? finiteSamples.reduce((sum, value) => sum + (value - mean) ** 2, 0) / (finiteSamples.length - 1)
    : 0
  return {
    count: finiteSamples.length,
    mean,
    variance
  }
}

const logGamma = (value: number): number => {
  const coefficients = [
    676.5203681218851,
    -1259.1392167224028,
    771.32342877765313,
    -176.61502916214059,
    12.507343278686905,
    -0.13857109526572012,
    9.9843695780195716e-6,
    1.5056327351493116e-7
  ]
  if (value < 0.5) {
    return Math.log(Math.PI) - Math.log(Math.sin(Math.PI * value)) - logGamma(1 - value)
  }
  let x = 0.99999999999980993
  const shifted = value - 1
  for (let i = 0; i < coefficients.length; i++) {
    x += (coefficients[i] || 0) / (shifted + i + 1)
  }
  const tValue = shifted + coefficients.length - 0.5
  return 0.5 * Math.log(2 * Math.PI) + (shifted + 0.5) * Math.log(tValue) - tValue + Math.log(x)
}

const betaContinuedFraction = (a: number, b: number, x: number): number => {
  const maxIterations = 120
  const epsilon = 3e-8
  const fpMin = 1e-30
  const qab = a + b
  const qap = a + 1
  const qam = a - 1
  let c = 1
  let d = 1 - (qab * x) / qap
  if (Math.abs(d) < fpMin) d = fpMin
  d = 1 / d
  let h = d

  for (let m = 1; m <= maxIterations; m++) {
    const m2 = 2 * m
    let aa = (m * (b - m) * x) / ((qam + m2) * (a + m2))
    d = 1 + aa * d
    if (Math.abs(d) < fpMin) d = fpMin
    c = 1 + aa / c
    if (Math.abs(c) < fpMin) c = fpMin
    d = 1 / d
    h *= d * c

    aa = -((a + m) * (qab + m) * x) / ((a + m2) * (qap + m2))
    d = 1 + aa * d
    if (Math.abs(d) < fpMin) d = fpMin
    c = 1 + aa / c
    if (Math.abs(c) < fpMin) c = fpMin
    d = 1 / d
    const delta = d * c
    h *= delta
    if (Math.abs(delta - 1) < epsilon) break
  }
  return h
}

const regularizedIncompleteBeta = (x: number, a: number, b: number): number => {
  if (x <= 0) return 0
  if (x >= 1) return 1
  const front = Math.exp(logGamma(a + b) - logGamma(a) - logGamma(b) + a * Math.log(x) + b * Math.log(1 - x))
  if (x < (a + 1) / (a + b + 2)) {
    return (front * betaContinuedFraction(a, b, x)) / a
  }
  return 1 - (front * betaContinuedFraction(b, a, 1 - x)) / b
}

const studentTCdf = (tValue: number, df: number): number | null => {
  if (!Number.isFinite(tValue) || !Number.isFinite(df) || df <= 0) return null
  if (tValue === 0) return 0.5
  const x = df / (df + tValue ** 2)
  const ib = regularizedIncompleteBeta(x, df / 2, 0.5)
  return tValue > 0 ? 1 - ib / 2 : ib / 2
}

const studentTInverseCdf = (probability: number, df: number): number | null => {
  if (!Number.isFinite(probability) || !Number.isFinite(df) || probability <= 0.5 || probability >= 1 || df <= 0) return null
  let low = 0
  let high = 1
  for (let index = 0; index < 40; index++) {
    const cdf = studentTCdf(high, df)
    if (cdf == null) return null
    if (cdf >= probability) break
    high *= 2
  }
  for (let index = 0; index < 80; index++) {
    const mid = (low + high) / 2
    const cdf = studentTCdf(mid, df)
    if (cdf == null) return null
    if (cdf < probability) {
      low = mid
    } else {
      high = mid
    }
  }
  return high
}

const calcPairedLiftMargin = (differences: number[], controlMean: number, confidenceLevel = 0.95) => {
  if (!Number.isFinite(controlMean) || controlMean === 0) return null
  const summary = summarizeSamples(differences)
  if (!summary || summary.count < 2) return null
  const standardError = Math.sqrt(summary.variance / summary.count)
  if (!Number.isFinite(standardError) || standardError <= 0) return 0
  const tCritical = studentTInverseCdf((1 + confidenceLevel) / 2, summary.count - 1)
  return tCritical == null ? null : Math.abs((tCritical * standardError) / controlMean)
}

const buildBucketKeyOrder = (data: ResultDataPoint[]): string[] | null => {
  const groups = new Map<string, ResultDataPoint[]>()
  for (const point of data) {
    const groupPoints = groups.get(point.group_name)
    if (groupPoints) {
      groupPoints.push(point)
    } else {
      groups.set(point.group_name, [point])
    }
  }
  const orders = [...groups.values()].map(groupPoints => {
    const seen = new Set<string>()
    const order: string[] = []
    for (const point of [...groupPoints].sort(compareResultPointOrder)) {
      if (seen.has(point.bucket_key)) return null
      seen.add(point.bucket_key)
      order.push(point.bucket_key)
    }
    return order
  })
  const firstSeen = new Map<string, number>()
  const edges = new Map<string, Set<string>>()
  const inDegree = new Map<string, number>()

  for (const order of orders) {
    if (!order) return null
    for (const bucketKey of order) {
      if (!firstSeen.has(bucketKey)) firstSeen.set(bucketKey, firstSeen.size)
      if (!edges.has(bucketKey)) edges.set(bucketKey, new Set())
      if (!inDegree.has(bucketKey)) inDegree.set(bucketKey, 0)
    }
    for (let index = 1; index < order.length; index++) {
      const previous = order[index - 1] || ''
      const current = order[index] || ''
      const nextKeys = edges.get(previous)
      if (previous === current || !nextKeys || nextKeys.has(current)) continue
      nextKeys.add(current)
      inDegree.set(current, (inDegree.get(current) || 0) + 1)
    }
  }

  const pending = [...inDegree.entries()]
    .filter(([, degree]) => degree === 0)
    .map(([bucketKey]) => bucketKey)
    .sort((left, right) => (firstSeen.get(left) || 0) - (firstSeen.get(right) || 0))
  const merged: string[] = []
  while (pending.length > 0) {
    const bucketKey = pending.shift() || ''
    merged.push(bucketKey)
    for (const nextKey of edges.get(bucketKey) || []) {
      const nextDegree = (inDegree.get(nextKey) || 0) - 1
      inDegree.set(nextKey, nextDegree)
      if (nextDegree === 0) {
        pending.push(nextKey)
        pending.sort((left, right) => (firstSeen.get(left) || 0) - (firstSeen.get(right) || 0))
      }
    }
  }

  return merged.length === inDegree.size ? merged : null
}

const groupedPoints = computed(() => {
  const groups = new Map<string, ResultDataPoint[]>()
  for (const point of sortedPoints.value) {
    const group = groups.get(point.group_name)
    if (group) {
      group.push(point)
    } else {
      groups.set(point.group_name, [point])
    }
  }
  return [...groups.entries()].map(([name, groupPoints], index) => ({
    name,
    color: seriesColors[index % seriesColors.length] || '#2563eb',
    points: groupPoints
  }))
})

const groupNames = computed(() => groupedPoints.value.map(group => group.name))

const orderedBucketKeys = computed(() => {
  return buildBucketKeyOrder(points.value) || []
})

const bucketKeyIndex = computed(() => {
  const index = new Map<string, number>()
  orderedBucketKeys.value.forEach((bucketKey, idx) => index.set(bucketKey, idx))
  return index
})

const comparisonUnitKey = (point: ResultDataPoint) => point.bucket_key

const collectPairedSamples = (controlGroupName: string, treatmentGroupName: string): PairedSamples | null => {
  const controlGroup = groupedPoints.value.find(item => item.name === controlGroupName)
  const treatmentGroup = groupedPoints.value.find(item => item.name === treatmentGroupName)
  if (!controlGroup || !treatmentGroup) return null

  const controlByUnit = new Map<string, number>()
  const treatmentByUnit = new Map<string, number>()
  for (const point of controlGroup.points) {
    if (Number.isFinite(point.metric_value)) {
      controlByUnit.set(comparisonUnitKey(point), point.metric_value)
    }
  }
  for (const point of treatmentGroup.points) {
    if (Number.isFinite(point.metric_value)) {
      treatmentByUnit.set(comparisonUnitKey(point), point.metric_value)
    }
  }

  const controlValues: number[] = []
  const treatmentValues: number[] = []
  const differences: number[] = []
  for (const [unitKey, controlValue] of controlByUnit.entries()) {
    if (!treatmentByUnit.has(unitKey)) continue
    const treatmentValue = treatmentByUnit.get(unitKey)
    if (treatmentValue == null) continue
    controlValues.push(controlValue)
    treatmentValues.push(treatmentValue)
    differences.push(treatmentValue - controlValue)
  }

  const control = summarizeSamples(controlValues)
  const treatment = summarizeSamples(treatmentValues)
  return control && treatment ? { control, treatment, differences } : null
}

const pairedSamples = computed(() => {
  if (!selectedControlGroupName.value || !selectedTreatmentGroupName.value || selectedControlGroupName.value === selectedTreatmentGroupName.value) {
    return null
  }
  return collectPairedSamples(selectedControlGroupName.value, selectedTreatmentGroupName.value)
})

const selectedControlSummary = computed(() => pairedSamples.value?.control ?? null)
const selectedTreatmentSummary = computed(() => pairedSamples.value?.treatment ?? null)

const comparisonResult = computed(() => {
  const samples = pairedSamples.value
  if (!samples) return null
  const controlSummary = samples.control
  const treatmentSummary = samples.treatment
  const lift = controlSummary.mean === 0 ? null : (treatmentSummary.mean - controlSummary.mean) / controlSummary.mean
  const liftMargin = calcPairedLiftMargin(samples.differences, controlSummary.mean)
  return {
    control: controlSummary,
    treatment: treatmentSummary,
    lift,
    liftMargin,
    significant: isLiftRangeSignificant(lift, liftMargin)
  }
})

const chartSeries = computed<ChartSeries[]>(() => {
  const bucketKeys = orderedBucketKeys.value
  const bounds = yBounds.value
  const span = bounds.max - bounds.min || 1
  return groupedPoints.value.map(group => {
    const chartPoints = group.points.map(point => {
      const index = bucketKeyIndex.value.get(point.bucket_key) || 0
      const x = chartPadding.left + (bucketKeys.length <= 1 ? plotWidth / 2 : (index / (bucketKeys.length - 1)) * plotWidth)
      const y = chartPadding.top + ((bounds.max - point.metric_value) / span) * plotHeight
      return { ...point, x, y }
    })
    const path = chartPoints.map((point, index) => `${index === 0 ? 'M' : 'L'} ${point.x.toFixed(2)} ${point.y.toFixed(2)}`).join(' ')
    return {
      name: group.name,
      color: group.color,
      points: chartPoints,
      path
    }
  })
})

const xAxisLabels = computed<AxisLabel[]>(() => {
  const bucketKeys = orderedBucketKeys.value
  if (bucketKeys.length === 0) return []
  const indexes = bucketKeys.length <= 3 ? bucketKeys.map((_, index) => index) : [0, Math.floor((bucketKeys.length - 1) / 2), bucketKeys.length - 1]
  return [...new Set(indexes)].map(index => ({
    text: bucketKeys[index] || '',
    x: chartPadding.left + (bucketKeys.length <= 1 ? plotWidth / 2 : (index / (bucketKeys.length - 1)) * plotWidth)
  }))
})

const yAxisLabels = computed<AxisLabel[]>(() => {
  const bounds = yBounds.value
  return [0, 0.5, 1].map(ratio => {
    const value = bounds.max - ratio * (bounds.max - bounds.min)
    return {
      text: formatMetric(value),
      y: chartPadding.top + ratio * plotHeight
    }
  })
})

const chartTitle = computed(() => {
  const parts = [selectedLayerName.value, selectedBucketType.value, selectedMetricName.value].filter(Boolean)
  return parts.length > 0 ? parts.join(' / ') : t('result.title')
})

const applyDefaultSelection = () => {
  const firstLayer = optionLayers.value[0]
  selectedLayerName.value = firstLayer?.name || ''
  const firstBucket = firstLayer?.bucket_types?.[0]
  selectedBucketType.value = firstBucket?.name || ''
  selectedMetricName.value = ''
}

const loadData = async () => {
  if (!appId.value || !expId.value || !selectedLayerName.value || !selectedBucketType.value || !selectedMetricName.value) return
  const begin = beginStamp.value
  const end = endStamp.value
  if (begin == null || end == null || !Number.isFinite(begin) || !Number.isFinite(end) || begin >= end) {
    ElMessage.error(t('result.invalidRange'))
    return
  }
  loadingData.value = true
  try {
    const res = await getExpResultData(appId.value, expId.value, {
      layer_name: selectedLayerName.value,
      bucket_type: selectedBucketType.value,
      metric_name: selectedMetricName.value,
      begin_stamp: begin,
      end_stamp: end
    })
    const nextPoints = res.data || []
    if (buildBucketKeyOrder(nextPoints) == null) {
      points.value = []
      ElMessage.error(t('result.dataOrderMismatch'))
      return
    }
    points.value = nextPoints
  } catch {
    ElMessage.error(t('message.failedLoadResultData'))
  } finally {
    loadingData.value = false
  }
}

const loadOptions = async () => {
  if (!appId.value || !expId.value) return
  loadingOptions.value = true
  try {
    const res = await getExpResultOptions(appId.value, expId.value)
    optionLayers.value = res.data.layers || []
    applyDefaultSelection()
  } catch {
    ElMessage.error(t('message.failedLoadResultOptions'))
  } finally {
    loadingOptions.value = false
  }
}

const normalizeBeginTime = () => {
  beginTime.value = normalizeDateToHour(beginTime.value)
}

const normalizeEndTime = () => {
  endTime.value = normalizeDateToHour(endTime.value)
}

const goBack = () => {
  if (!appId.value) {
    router.push('/')
    return
  }
  router.push({
    path: '/',
    query: {
      app_id: String(appId.value)
    }
  })
}

const syncComparisonSelection = () => {
  const names = groupNames.value
  if (names.length === 0) {
    selectedControlGroupName.value = ''
    selectedTreatmentGroupName.value = ''
    return
  }
  if (!names.includes(selectedControlGroupName.value)) {
    selectedControlGroupName.value = ''
  }
  if (!names.includes(selectedTreatmentGroupName.value) || selectedTreatmentGroupName.value === selectedControlGroupName.value) {
    selectedTreatmentGroupName.value = ''
  }
}

watch(selectedLayerName, () => {
  const firstBucket = bucketOptions.value[0]
  selectedBucketType.value = firstBucket?.name || ''
  selectedMetricName.value = ''
})

watch(selectedBucketType, () => {
  selectedMetricName.value = ''
})

watch([selectedLayerName, selectedBucketType, selectedMetricName], () => {
  points.value = []
})

watch(groupNames, () => {
  syncComparisonSelection()
})

watch(selectedControlGroupName, () => {
  if (selectedControlGroupName.value === selectedTreatmentGroupName.value) {
    selectedTreatmentGroupName.value = ''
  }
})

onMounted(async () => {
  if (!appId.value || !expId.value) {
    ElMessage.error(t('result.invalidRoute'))
    return
  }
  await loadOptions()
})
</script>

<template>
  <div class="result-page">
    <div class="result-header">
      <el-button link :icon="Back" class="back-button" @click="goBack">{{ t('result.back') }}</el-button>
      <el-button type="primary" :icon="Search" :loading="loadingData" :disabled="!canQuery" @click="loadData">
        {{ t('result.query') }}
      </el-button>
    </div>

    <el-form class="result-controls" label-position="top">
      <el-form-item :label="t('result.layer')">
        <el-select v-model="selectedLayerName" :disabled="loadingOptions || !hasOptions" filterable>
          <el-option v-for="layer in optionLayers" :key="layer.name" :label="layer.name" :value="layer.name" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('result.bucketType')">
        <el-select v-model="selectedBucketType" :disabled="loadingOptions || !selectedLayerName" filterable>
          <el-option v-for="bucket in bucketOptions" :key="bucket.name" :label="bucket.name" :value="bucket.name" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('result.metric')">
        <el-select v-model="selectedMetricName" :disabled="loadingOptions || !selectedBucketType" filterable>
          <el-option v-for="metric in metricOptions" :key="metric" :label="metric" :value="metric" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('result.beginTime')">
        <el-date-picker
          v-model="beginTime"
          type="datetime"
          format="YYYY-MM-DD HH:mm"
          time-format="HH:mm"
          :disabled-minutes="disabledSubHourUnits"
          :disabled-seconds="disabledSubHourUnits"
          :editable="false"
          @change="normalizeBeginTime"
        />
      </el-form-item>
      <el-form-item :label="t('result.endTime')">
        <el-date-picker
          v-model="endTime"
          type="datetime"
          format="YYYY-MM-DD HH:mm"
          time-format="HH:mm"
          :disabled-minutes="disabledSubHourUnits"
          :disabled-seconds="disabledSubHourUnits"
          :editable="false"
          @change="normalizeEndTime"
        />
      </el-form-item>
    </el-form>

    <div v-if="!loadingOptions && !hasOptions" class="empty-state">
      {{ t('result.noOptions') }}
    </div>

    <div v-else class="result-content">
      <div class="chart-panel" v-loading="loadingOptions || loadingData">
        <div v-if="sortedPoints.length > 0" class="chart-body">
          <div class="legend-column">
            <span v-for="series in chartSeries" :key="series.name" class="legend-item">
              <span class="legend-swatch" :style="{ backgroundColor: series.color }" />
              {{ series.name }}
            </span>
          </div>

          <svg class="line-chart" :viewBox="`0 0 ${chartWidth} ${chartHeight}`" role="img" :aria-label="chartTitle">
            <line
              v-for="label in yAxisLabels"
              :key="`grid-${label.y}`"
              :x1="chartPadding.left"
              :x2="chartWidth - chartPadding.right"
              :y1="label.y"
              :y2="label.y"
              class="grid-line"
            />
            <line
              :x1="chartPadding.left"
              :x2="chartWidth - chartPadding.right"
              :y1="chartHeight - chartPadding.bottom"
              :y2="chartHeight - chartPadding.bottom"
              class="axis-line"
            />
            <line
              :x1="chartPadding.left"
              :x2="chartPadding.left"
              :y1="chartPadding.top"
              :y2="chartHeight - chartPadding.bottom"
              class="axis-line"
            />
            <text v-for="label in yAxisLabels" :key="`y-${label.y}`" :x="chartPadding.left - 10" :y="(label.y ?? 0) + 4" class="axis-text" text-anchor="end">
              {{ label.text }}
            </text>
            <text
              v-for="label in xAxisLabels"
              :key="`x-${label.x}`"
              :x="label.x"
              :y="chartHeight - chartPadding.bottom + 26"
              class="axis-text"
              text-anchor="middle"
            >
              {{ label.text }}
            </text>
            <g v-for="series in chartSeries" :key="series.name">
              <path :d="series.path" :stroke="series.color" class="series-line" />
              <circle v-for="point in series.points" :key="`${series.name}-${point.bucket_stamp}-${point.bucket_key}`" :cx="point.x" :cy="point.y" r="3.8" :fill="series.color">
                <title>{{ `${series.name} / ${point.bucket_key}: ${formatMetric(point.metric_value)}` }}</title>
              </circle>
            </g>
          </svg>

          <div class="confidence-box">
            <template v-if="groupNames.length > 0">
              <label class="confidence-field">
                <span>{{ t('result.controlGroup') }}</span>
                <el-select v-model="selectedControlGroupName" size="small">
                  <el-option
                    v-for="name in groupNames"
                    :key="`control-${name}`"
                    :label="name"
                    :value="name"
                    :disabled="name === selectedTreatmentGroupName"
                  />
                </el-select>
              </label>
              <div class="confidence-item">
                <span>{{ t('result.controlMean') }}</span>
                <strong>{{ formatNullableMetric(selectedControlSummary?.mean) }}</strong>
              </div>
              <label class="confidence-field">
                <span>{{ t('result.treatmentGroup') }}</span>
                <el-select v-model="selectedTreatmentGroupName" size="small">
                  <el-option
                    v-for="name in groupNames"
                    :key="`treatment-${name}`"
                    :label="name"
                    :value="name"
                    :disabled="name === selectedControlGroupName"
                  />
                </el-select>
              </label>
              <div class="confidence-item">
                <span>{{ t('result.treatmentMean') }}</span>
                <strong>{{ formatNullableMetric(selectedTreatmentSummary?.mean) }}</strong>
              </div>
            </template>
            <template v-else>
              <div class="confidence-item">
                <span>{{ t('result.controlMean') }}</span>
                <strong>-</strong>
              </div>
              <div class="confidence-item">
                <span>{{ t('result.treatmentMean') }}</span>
                <strong>-</strong>
              </div>
            </template>
            <div class="confidence-item">
              <span>{{ t('result.lift') }}</span>
              <strong :class="{ positive: comparisonResult && comparisonResult.lift != null && comparisonResult.lift > 0, negative: comparisonResult && comparisonResult.lift != null && comparisonResult.lift < 0 }">
                {{ formatLiftWithMargin(comparisonResult?.lift ?? null, comparisonResult?.liftMargin ?? null) }}
              </strong>
            </div>
            <div class="confidence-item">
              <span>{{ t('result.significance') }}</span>
              <strong :class="{ positive: comparisonResult?.significant === true, negative: comparisonResult?.significant === false }">
                {{ comparisonResult?.significant == null ? '-' : (comparisonResult.significant ? t('result.significant') : t('result.notSignificant')) }}
              </strong>
            </div>
          </div>

          <p class="confidence-note">{{ t('result.confidenceNote') }}</p>
        </div>

        <div v-else class="empty-state compact">
          {{ t('result.noData') }}
        </div>
      </div>

    </div>
  </div>
</template>

<style scoped>
.result-page {
  display: flex;
  flex-direction: column;
  gap: 18px;
}
.result-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}
.back-button {
  padding-left: 0;
}
.result-controls {
  display: grid;
  grid-template-columns: minmax(140px, 1fr) minmax(130px, 0.8fr) minmax(140px, 1fr) minmax(180px, 1fr) minmax(180px, 1fr);
  gap: 12px;
  align-items: end;
}
.result-controls :deep(.el-form-item) {
  margin-bottom: 0;
}
.result-controls :deep(.el-select),
.result-controls :deep(.el-date-editor) {
  width: 100%;
}
.result-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.chart-panel {
  min-height: 360px;
  padding: 16px;
  border: 1px solid #dcdfe6;
  border-radius: 8px;
  background: #fff;
}
.chart-body {
  display: grid;
  grid-template-columns: minmax(96px, 140px) minmax(0, 1fr) minmax(150px, 180px);
  gap: 16px;
  align-items: start;
}
.line-chart {
  width: 100%;
  height: 300px;
  display: block;
}
.grid-line {
  stroke: #e4e7ed;
  stroke-width: 1;
}
.axis-line {
  stroke: #909399;
  stroke-width: 1;
}
.axis-text {
  fill: #606266;
  font-size: 12px;
}
.series-line {
  fill: none;
  stroke-width: 2.2;
  stroke-linecap: round;
  stroke-linejoin: round;
}
.legend-column {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 10px;
  padding-top: 8px;
}
.legend-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: #303133;
  font-size: 13px;
}
.legend-swatch {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}
.confidence-box {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 300px;
  padding-left: 16px;
  border-left: 1px solid #ebeef5;
}
.confidence-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  color: #606266;
  font-size: 12px;
}
.confidence-field :deep(.el-select) {
  width: 100%;
}
.confidence-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.confidence-item span {
  color: #606266;
  font-size: 12px;
}
.confidence-item strong {
  color: #303133;
  font-size: 16px;
  font-weight: 600;
}
.confidence-item strong.positive {
  color: #16a34a;
}
.confidence-item strong.negative {
  color: #dc2626;
}
.confidence-note {
  grid-column: 1 / -1;
  margin: -2px 0 0;
  color: #909399;
  font-size: 12px;
  line-height: 1.5;
}
.empty-state {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 220px;
  color: #909399;
  border: 1px dashed #dcdfe6;
  border-radius: 8px;
  background: #fff;
}
.empty-state.compact {
  min-height: 270px;
}
@media (max-width: 960px) {
  .result-controls {
    grid-template-columns: 1fr 1fr;
  }
  .chart-body {
    grid-template-columns: minmax(96px, 140px) minmax(0, 1fr);
  }
  .confidence-box {
    grid-column: 1 / -1;
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    min-height: 0;
    padding-top: 14px;
    padding-left: 0;
    border-top: 1px solid #ebeef5;
    border-left: 0;
  }
}
@media (max-width: 640px) {
  .result-header {
    align-items: stretch;
    flex-direction: column;
  }
  .result-controls {
    grid-template-columns: 1fr;
  }
  .chart-panel {
    padding: 12px;
  }
  .chart-body {
    grid-template-columns: 1fr;
    gap: 10px;
  }
  .legend-column {
    padding-top: 0;
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .confidence-box {
    grid-column: 1 / -1;
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
