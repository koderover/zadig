<template>
  <div class="function-test-summary">
    <v-chart :options="option"
             :autoresize="true" />
  </div>
</template>

<script>
import ECharts from 'vue-echarts'
import 'echarts/lib/component/title'
import 'echarts/lib/component/tooltip'
import 'echarts/lib/component/legend'
import 'echarts/lib/chart/pie'

export default {
  data () {
    return {
      option: {
        title: {
          textStyle: {
            color: '#666666'
          },
          subtextStyle: {
            color: '#999999'
          }

        },
        color: [
          '#67c23a',
          '#f56c6c',
          '#E6A23C',
          '#909399'
        ],
        tooltip: {
          trigger: 'item',
          formatter: '{a} <br/>{b}: {c} ({d}%)'
        },
        legend: {
          orient: 'vertical',
          x: 'right',
          formatter: (name) => {
            const data = this.option.series[0].data
            if (name && data.length && data[0]) {
              let total = 0
              let target
              for (let i = 0, l = data.length; i < l; i++) {
                total += data[i].value
                if (data[i].name === name) {
                  target = data[i].value
                }
              }
              const percentage = ((target / total) * 100).toFixed(2)
              return `${name} ${target} ${total !== 0 ? '(' + percentage + '%)' : ''}`
            }
            return '-'
          }
        },
        series: [
          {
            name: '测试结果',
            type: 'pie',
            radius: ['40%', '50%'],
            center: ['20%', '35%'],
            avoidLabelOverlap: false,
            stillShowZeroSum: false,
            label: {
              normal: {
                show: true,
                position: 'center',
                color: '#ccc',
                textStyle: {
                  fontSize: '30',
                  fontWeight: 'bold'
                },
                formatter: (name) => {
                  const data = this.option.series[0].data
                  if (name && data.length && data[0]) {
                    let total = 0
                    for (let i = 0, l = data.length; i < l; i++) {
                      total += data[i].value
                    }
                    return `${total}`
                  }
                  return '-'
                }
              },
              emphasis: {
                show: false,
                textStyle: {
                  fontSize: '30',
                  fontWeight: 'bold'
                }
              }
            },
            labelLine: {
              normal: {
                show: false
              }
            },
            data: [

            ]
          }
        ]
      }
    }
  },
  methods: {

  },
  computed: {

  },
  components: {
    'v-chart': ECharts
  },
  props: {
    total: {
      required: true,
      default: 0,
      type: Number
    },
    success: {
      required: true,
      default: 0,
      type: Number
    },
    failure: {
      required: true,
      default: 0,
      type: Number
    },
    skip: {
      required: true,
      default: 0,
      type: Number
    },
    error: {
      required: true,
      default: 0,
      type: Number
    },
    duration: {
      required: false,
      default: 0,
      type: Number
    }
  },
  watch: {
    total: {
      handler (val, old_val) {
        this.option.series[0].data = [
          { name: '成功', value: this.success },
          { name: '失败', value: this.failure },
          { name: '错误', value: this.error },
          { name: '未执行', value: this.skip }
        ]
      }
    }
  }
}
</script>

<style lang="less">
.function-test-summary {
  width: 100%;
  height: 100%;
}

.echarts {
  width: 100% !important;
  height: 100% !important;
}
</style>
