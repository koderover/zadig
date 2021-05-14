<template>
  <div class="task-detail-deploy">
    <el-card v-if="deploy"
             class="box-card task-process"
             :body-style="{ margin: '15px 0 0 0' }">
      <div slot="header"
           class="clearfix subtask-header">
        <span>交付物部署</span>
        <div v-if="deploy.status==='running'"
             class="loader">
          <div class="ball-scale-multiple">
            <div></div>
            <div></div>
            <div></div>
          </div>
        </div>
      </div>
      <div class="deploy-item">
        <div class="error-wrapper">
          <el-alert v-if="deploy.error"
                    title="错误信息"
                    :description="deploy.error"
                    type="error"
                    close-text="知道了">

          </el-alert>
        </div>
        <el-row :gutter="0">
          <el-col :span="6">
            <div class="grid-content item-title">
              <i class="iconfont iconzhuangtai"></i> 部署状态
            </div>
          </el-col>
          <el-col :span="6">
            <div class="grid-content item-desc"
                 :class="$translate.calcTaskStatusColor(deploy.status)">
              {{deploy.status?$translate.translateTaskStatus(deploy.status):"未运行"}}
            </div>
          </el-col>
          <el-col :span="6">
            <div class="grid-content item-title">
              <i class="iconfont iconjiqun1"></i> 部署环境
            </div>
          </el-col>
          <el-col :span="6">
            <div class="grid-content item-desc">
              <router-link class="env-link"
                           :to="`/v1/projects/detail/${deploy.product_name}/envs/detail?envName=${deploy.env_name}`">
                {{deploy.namespace}}</router-link>
            </div>
          </el-col>
        </el-row>
        <el-row :gutter="0">
          <el-col :span="6">
            <div class="grid-content item-title">
              <i class="iconfont iconSliceCopy"></i> 镜像信息
            </div>
          </el-col>
          <el-col :span="6">
            <el-tooltip effect="dark"
                        :content="deploy.image"
                        placement="top">
              <div class="grid-content item-desc">{{deploy.image?deploy.image.split('/')[2]:"*"}}
              </div>
            </el-tooltip>
          </el-col>
          <el-col :span="6">
            <div class="grid-content item-title">
              <i class="iconfont iconfuwu"></i> 服务名称
            </div>
          </el-col>
          <el-col :span="6">
            <div class="grid-content item-desc">
              <router-link class="env-link"
                           :to="`/v1/projects/detail/${deploy.product_name}/envs/detail/${deploy.service_name}?envName=${deploy.env_name}&projectName=${deploy.product_name}&namespace=${deploy.namespace}`">
                {{deploy.service_name}}</router-link>
            </div>
          </el-col>
        </el-row>
      </div>
    </el-card>

  </div>
</template>

<script>
import mixin from '@utils/task_detail_mixin';

export default {
  data() {
    return {
    };
  },
  computed: {

  },
  methods: {
  },
  props: {
    deploy: {
      type: Object,
      required: true,
    }
  },
  mixins: [mixin]
};
</script>

<style lang="less">
@import "~@assets/css/component/subtask.less";

.task-detail-deploy {
  .deploy-item {
    margin-bottom: 15px;
  }
  .env-link {
    color: #1989fa;
  }
}
</style>
