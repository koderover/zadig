<template>
  <div class="service-details-container">
    <div class="info-card">
      <div class="info-header">
        <span>基本信息</span>
      </div>
      <el-row :gutter="0"
              class="info-body">
        <el-col :span="12"
                class="WAN">
          <div class="addr-title title">
            服务名称
          </div>
          <div class="addr-content">
            <div>{{serviceName}}</div>
          </div>
        </el-col>
      </el-row>
    </div>
    <div class="info-card">
      <div class="info-header">
        <span>服务状态</span>
      </div>
      <div class="info-body">
        <template>
          <el-table v-loading="getEnvLoading"
                    :data="envStatus"
                    :span-method="objectSpanMethod"
                    style="width: 100%;">
            <el-table-column label="主机资源">
              <template slot-scope="scope">
                <span>{{ scope.row.address }}</span>
              </template>
            </el-table-column>
            <el-table-column label="探活配置">
              <template >
                <div v-for="item,index in currentService.health_checks" :key="index" >
                  <span
                        v-if="item.port && item.protocol">{{`${item.protocol}:${item.port}`}}</span>
                  <span
                        v-else-if="item.protocol">{{`${item.protocol}`}}</span>
                  <span v-if="item.path">{{`${item.path}`}}</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="状态">
              <template slot-scope="scope">
                <span class="health-check"
                      :class="statusColorMap[scope.row.status]">{{statusTranslation[scope.row.status]}}</span>
              </template>
            </el-table-column>

          </el-table>
        </template>
      </div>
    </div>
  </div>
</template>

<script>
import { serviceTemplateAPI } from '@api'
import bus from '@utils/event_bus'
import { sortBy } from 'lodash'
export default {
  data () {
    return {
      currentService: {},
      getEnvLoading: false,
      envStatus: [],
      spanArr: [],
      pos: null,
      window: window,
      statusColorMap: {
        Running: 'green',
        Error: 'red'
      },
      statusTranslation: {
        Running: '健康',
        Error: '不健康'
      }
    }
  },

  computed: {
    projectName () {
      return (this.$route.params.project_name ? this.$route.params.project_name : this.$route.query.projectName)
    },
    // 共享服务需要该参数
    originProjectName () {
      return (this.$route.query.originProjectName ? this.$route.query.originProjectName : this.projectName)
    },
    clusterId () {
      return this.$route.query.clusterId
    },
    serviceName () {
      return this.$route.params.service_name
    },
    envName () {
      return this.$route.query.envName
    },
    isProd () {
      return this.$route.query.isProd === 'true'
    },
    namespace () {
      return this.$route.query.namespace
    }
  },

  methods: {
    fetchServiceData () {
      this.getEnvLoading = true
      const projectName = this.projectName
      const serviceName = this.serviceName
      serviceTemplateAPI(serviceName, 'pm', projectName).then((res) => {
        this.currentService = res
        this.getEnvLoading = false
        if (res.env_statuses) {
          this.envStatus = sortBy(res.env_statuses.filter(element => {
            return element.env_name === this.envName
          }), 'host_id')
          this.getSpanArr(this.envStatus)
        }
      })
    },

    getSpanArr (data) {
      for (let i = 0; i < data.length; i++) {
        if (i === 0) {
          this.spanArr.push(1)
          this.pos = 0
        } else {
          if (data[i].host_id === data[i - 1].host_id) {
            this.spanArr[this.pos] += 1
            this.spanArr.push(0)
          } else {
            this.spanArr.push(1)
            this.pos = i
          }
        }
      }
    },
    objectSpanMethod ({ row, column, rowIndex, columnIndex }) {
      if (columnIndex === 0) {
        const _row = this.spanArr[rowIndex]
        const _col = _row > 0 ? 1 : 0
        return {
          rowspan: _row,
          colspan: _col
        }
      }
    }
  },
  created () {
    this.fetchServiceData()
    bus.$emit(`set-topbar-title`,
      {
        title: '',
        breadcrumb: [
          { title: '项目', url: '/v1/projects' },
          { title: this.projectName, url: `/v1/projects/detail/${this.projectName}` },
          { title: '集成环境', url: `/v1/projects/detail/${this.projectName}/envs/detail` },
          { title: this.envName, url: `/v1/projects/detail/${this.projectName}/envs/detail?envName=${this.envName}` },
          { title: this.serviceName, url: `` }
        ]
      })
  }
}

</script>

<style lang="less">
@import "~@assets/css/component/service-detail.less";
</style>
