<script lang="ts" setup>
import { i18n } from '@/locales'
import { computed } from 'vue'

const props = defineProps({
  tilesData: <any>{},
  type: String
})

type GaugeData = {
  percent: number
  text: string
  percentage: string
}

const clampPercent = (value: number) => {
  if (!Number.isFinite(value)) return 0
  return Math.min(100, Math.max(0, value))
}

const data = computed<GaugeData>(() => {
  const d = props.tilesData
  if (!d.mem && !d.cpu && !d.disk && !d.swap) {
    return { percent: 0, text: '-', percentage: '' }
  }

  switch (props.type) {
    case 'g-cpu': {
      const percent = clampPercent(Number(d.cpu) || 0)
      return { percent, text: `${percent.toFixed(2)}%`, percentage: '' }
    }
    case 'g-mem':
      return usageGauge(d.mem)
    case 'g-disk':
      return usageGauge(d.disk)
    case 'g-swap':
      return usageGauge(d.swap)
  }

  return { percent: 0, text: '-', percentage: '' }
})

const sizeUnits = [
  { divisor: 1024 ** 5, key: 'PB' },
  { divisor: 1024 ** 4, key: 'TB' },
  { divisor: 1024 ** 3, key: 'GB' },
  { divisor: 1024 ** 2, key: 'MB' },
  { divisor: 1024, key: 'KB' },
  { divisor: 1, key: 'B' },
]

const formatUsage = (current: number, total: number) => {
  const unit = sizeUnits.find(item => total >= item.divisor) ?? sizeUnits[sizeUnits.length - 1]
  const unitName = i18n.global.t(`stats.${unit.key}`)
  return `${(current / unit.divisor).toFixed(2)} / ${(total / unit.divisor).toFixed(2)} ${unitName}`
}

const usageGauge = (usage: any): GaugeData => {
  const current = Number(usage?.current) || 0
  const total = Number(usage?.total) || 0
  if (total <= 0) return { percent: 0, text: '-', percentage: '0.00%' }

  const percent = clampPercent(current * 100 / total)
  return {
    percent,
    text: formatUsage(current, total),
    percentage: `${percent.toFixed(2)}%`,
  }
}

const cssTransformRotateValue = computed(() => {
  const percentageAsFraction = data.value.percent / 100
  const halfPercentage = percentageAsFraction / 2
  return `${halfPercentage}turn`
})

const gaugeColor = computed(() => {
  if (data.value.percent > 90) return 'error'
  if (data.value.percent > 70) return 'warning'
  return 'primary'
})
</script>

<template>
  <div class="gauge__outer">
    <div class="gauge__inner">
      <div
        class="gauge__fill"
        :style="{
          transform: `rotate(${cssTransformRotateValue})`,
          background: `rgb(var(--v-theme-${gaugeColor}))`
        }"
      ></div>
      <div class="gauge__cover">
        <span class="gauge__value" :class="{ 'gauge__value--compact': data.percentage }" dir="ltr">
          {{ data.text }}
        </span>
        <span v-if="data.percentage" class="gauge__percentage" dir="ltr">
          {{ data.percentage }}
        </span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.gauge__outer {
  width: 100%;
  max-width: 250px;
}

.gauge__inner {
  width: 100%;
  height: 0;
  padding-bottom: 50%;
  background: rgb(var(--v-theme-surface));
  position: relative;
  border-top-left-radius: 100% 200%;
  border-top-right-radius: 100% 200%;
  overflow: hidden;
}

.gauge__fill {
  position: absolute;
  top: 100%;
  left: 0;
  width: inherit;
  height: 100%;
  background: rgb(var(--v-theme-primary));
  transform-origin: center top;
  transform: rotate(0turn);
  transition: transform 0.2s ease-out;
}

.gauge__cover {
  width: 75%;
  height: 150%;
  background: rgb(var(--v-theme-background));
  position: absolute;
  top: 25%;
  left: 50%;
  transform: translateX(-50%);
  border-radius: 50%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 0 8px 25%;
  box-sizing: border-box;
  font-family: 'Lexend', sans-serif;
  font-weight: bold;
  font-size: 30px;
  line-height: 1.15;
  text-align: center;
}

.gauge__value {
  white-space: nowrap;
}

.gauge__value--compact {
  font-size: 17px;
}

.gauge__percentage {
  margin-top: 4px;
  font-size: 18px;
}
</style>
