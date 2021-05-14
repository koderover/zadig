<template>
  <div class="mobile-task-detail-deploy">
    <div v-if="deploys.length > 0">
      <div v-for="(deploy,index) in deploys"
           :key="index"
           class="deploy-item">
        <div class="error-wrapper">
          <van-notice-bar v-if="deploy.error"
                          color="#F56C6C"
                          background="#fef0f0">
            {{deploy.error}}
          </van-notice-bar>
        </div>
        <van-row :gutter="0">
          <van-col :span="6">
            <div class="item-title">部署状态</div>
          </van-col>
          <van-col :span="6">
            <div class="item-desc"
                 :class="$translate.calcTaskStatusColor(deploy.status)">
              {{deploy.status?$translate.translateTaskStatus(deploy.status):"未运行"}}
            </div>
          </van-col>
          <van-col v-if="deploy.status!=='running'"
                   :span="6">
            <div class="item-title">
              持续时间
            </div>
          </van-col>
          <van-col v-if="deploy.status!=='running'"
                   :span="6">
            <span class="">{{$utils.timeFormat(deploy.end_time - deploy.start_time)}}</span>
          </van-col>
        </van-row>
        <van-row :gutter="0">
          <van-col :span="6">
            <div class="item-title"> 镜像信息</div>
          </van-col>
          <van-col :span="18">
            <el-tooltip effect="dark"
                        :content="deploy.image"
                        placement="top">
              <div class=" item-desc">{{deploy.image?deploy.image.split('/')[2]:"*"}}
              </div>
            </el-tooltip>
          </van-col>
        </van-row>
        <van-row :gutter="0">
          <van-col :span="6">
            <div class="item-title">服务名称</div>
          </van-col>
          <van-col :span="18">
            <div class="item-desc">
              {{deploy.service_name}}
            </div>
          </van-col>
        </van-row>
      </div>
    </div>
  </div>
</template>

<script>
import mixin from '@utils/task_detail_mixin';
import { NoticeBar, Col, Row } from 'vant';
export default {
  components: {
    [NoticeBar.name]: NoticeBar,
    [Col.name]: Col,
    [Row.name]: Row
  },
  data() {
    return {
    };
  },

  computed: {

  },
  methods: {
  },
  props: {
    deploys: {
      type: Array,
      required: true,
    },
    pipelineName: {
      type: String,
      required: true,
    },
    taskID: {
      type: [String, Number],
      required: true,
    },
  },
  mixins: [mixin]
};
</script>

<style lang="less">
@import "~@assets/css/component/subtask.less";

.mobile-task-detail-deploy {
  color: #303133;
  a {
    color: #1989fa;
  }
}
</style>
