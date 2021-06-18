<template>
  <div class="build-stage">
    <el-card class="box-card">
      <el-row :gutter="20"
              v-show="allTargets.length > 0"
              class="header">
        <el-col :span="2">
          <div class="index">
            序号
          </div>
        </el-col>
        <el-col :span="11">
          <div class="service">
            服务
          </div>
        </el-col>

        <el-col :span="11">
          <div class="deploy">
            部署
          </div>
        </el-col>
      </el-row>
      <div v-if="allTargets.length === 0"
           class="no-resources">

        <div>
          <img src="@assets/icons/illustration/environment.svg"
               alt="">
        </div>
        <div class="description">

          <p> 该项目不存在可构建的服务组件，请前往 项目->服务 中创建服务</p>
          <router-link :to="`/v1/projects/detail/${product_tmpl_name}/services`">
            <el-button type="primary"
                       size="small"
                       round
                       plain>服务配置</el-button>
          </router-link>
        </div>
      </div>
      <div>
        <el-row v-for="(config, _idx) of serviceConfigs"
                :key="_idx"
                :gutter="20"
                class="row">
          <el-col :span="2">
            <div class="index">
              {{ _idx+1 }}
            </div>
          </el-col>
          <el-col :span="11">
            <div class="service">
              <span class="service-link">
                <template>
                  <router-link
                               :to="`/v1/projects/detail/${config.target.product_name}/services?service_name=${config.target.service_name}`">
                    {{ `${config.target.service_name}/${config.target.service_module}`}}
                  </router-link>
                </template>
              </span>
            </div>
          </el-col>
          <el-col :span="11">
            <div class="deploy">
              <div>
                {{ `${config.target.service_name}/${config.target.service_module}`}}
              </div>
            </div>
          </el-col>
        </el-row>
      </div>
    </el-card>
  </div>
</template>

<script type="text/javascript">
import bus from '@utils/event_bus'

export default {
  computed: {
    presetMap () {
      return _.keyBy(this.presets, (i) => {
        return i.target.service_name + '/' + i.target.service_module
      })
    },
    serviceConfigs: {
      get () {
        return this.build_stage.modules
      },
      set (val) {
        this.build_stage.modules = val
      }
    },
    allTargets () {
      return this.presets.map(p => p.target)
    }
  },
  props: {
    build_stage: {
      required: true,
      type: Object
    },
    editMode: {
      required: true,
      type: Boolean
    },
    product_tmpl_name: {
      required: true,
      type: String
    },
    presets: {
      required: true,
      type: Array
    }
  },
  watch: {
    product_tmpl_name (newVal, oldVal) {
      if (oldVal) {
        this.serviceConfigs = []
      }
    },
    allTargets (newVal, oldVal) {
      this.serviceConfigs = newVal.map(tar => {
        return {
          target: tar
        }
      })
    }
  },
  created () {
    bus.$on('check-tab:buildDeploy', () => {
      bus.$emit('receive-tab-check:buildDeploy', true)
    })
  },
  beforeDestroy () {
    bus.$off('check-tab:buildDeploy')
  }
}
</script>

<style lang="less">
.build-stage {
  .header,
  .row {
    .index,
    .service,
    .deploy {
      display: inline-block;
      height: 42px;
      line-height: 42px;
      vertical-align: top;
    }

    .service {
      word-break: break-all;

      .service-link {
        a {
          color: #1989fa;
        }
      }
    }

    .deploy {
      vertical-align: initial;
      word-break: break-all;
    }
  }

  .header {
    padding: 0 5px;
    color: #7a8599;
    font-size: 16px;
    border-bottom: 1px solid #ddd;
  }

  .row {
    padding: 0 5px;
    font-size: 14px;
    transition: background 0.3s ease-out;

    &:hover {
      background-color: #f0f2f5;
    }

    .deploy-row.multi {
      height: 18px;
      line-height: 18px;
    }
  }

  .el-card {
    .el-card__body {
      padding-bottom: 50px;
    }
  }

  .no-resources {
    padding: 45px;
    border-style: hidden;
    border-collapse: collapse;

    img {
      display: block;
      width: 240px;
      height: 168px;
      margin: 10px auto;
    }

    .description {
      margin: 16px auto;
      text-align: center;

      p {
        color: #8d9199;
        font-size: 14px;
        line-height: 2;
      }
    }
  }
}
</style>
