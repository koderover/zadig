<template>
  <div class="mobile-task-detail-build">
    <div v-if="!$utils.isEmpty(buildv2) && buildv2.enabled">
      <div class="error-wrapper">
        <van-notice-bar v-if="buildv2.error"
                        color="#F56C6C"
                        background="#fef0f0">
          {{buildv2.error}}
        </van-notice-bar>
      </div>
      <div class="text">
        <van-row :gutter="0">
          <van-col :span="6">
            <div class="item-title">
              构建状态
            </div>
          </van-col>
          <van-col :span="6">
            <div class="">
              <span :class="buildOverallColor">{{buildv2.status?buildOverallStatusZh:"未运行"}}</span>
            </div>
          </van-col>
          <van-col v-if="buildv2.status!=='running'"
                   :span="6">
            <div class="item-title">
              持续时间
            </div>
          </van-col>
          <van-col v-if="buildv2.status!=='running'"
                   :span="6">
            <span class="">{{$utils.timeFormat(buildv2.end_time - buildv2.start_time)}}</span>
          </van-col>
        </van-row>
        <van-row v-if="buildv2.job_ctx.builds">
          <div>
            <span>构建信息</span>
          </div>
        </van-row>
        <van-row :gutter="0"
                 v-for="(build,index) in buildv2.job_ctx.builds"
                 :key="index">
          <van-col :span="24">
            <span>{{build.repo_name}} ({{build.source}})</span>
            <el-tooltip :content="build.source==='gerrit'?`暂不支持在 gerrit 上查看 Release`:`在 ${build.source} 上查看 Release`"
                        placement="top"
                        effect="dark">
              <span v-if="build.tag"
                    class="link">
                <a v-if="build.source==='github'||build.source==='gitlab'"
                   :href="`${build.address}/${build.repo_owner}/${build.repo_name}/tags/${build.tag}`"
                   target="_blank">{{build.tag}}
                </a>
                <span v-if="build.source==='gerrit'">{{build.tag}}</span>
              </span>
            </el-tooltip>
            <el-tooltip :content="build.source==='gerrit'?`暂不支持在 gerrit 上查看 Branch`:`在 ${build.source} 上查看 Branch`"
                        placement="top"
                        effect="dark">
              <span v-if="build.branch && !build.tag"
                    class="link">
                <a v-if="build.source==='github'||build.source==='gitlab'"
                   :href="`${build.address}/${build.repo_owner}/${build.repo_name}/tree/${build.branch}`"
                   target="_blank">{{build.branch}}
                </a>
                <span v-if="build.source==='gerrit'">{{build.branch}}
                </span>
                <a v-if="!build.source"
                   :href="`${build.address}/${build.repo_owner}/${build.repo_name}/tree/${build.branch}`"
                   target="_blank">{{build.branch}}
                </a>
              </span>
            </el-tooltip>
            <el-tooltip :content="`在 ${build.source} 上查看 PR`"
                        placement="top"
                        effect="dark">
              <span v-if="build.pr && build.pr>0"
                    class="link">
                <a v-if="build.source==='github'"
                   :href="`${build.address}/${build.repo_owner}/${build.repo_name}/pull/${build.pr}`"
                   target="_blank">{{"PR-"+build.pr}}
                </a>
                <a v-if="build.source==='gitlab'"
                   :href="`${build.address}/${build.repo_owner}/${build.repo_name}/merge_requests/${build.pr}`"
                   target="_blank">{{"PR-"+build.pr}}
                </a>
                <a v-if="!build.source"
                   :href="`${build.address}/${build.repo_owner}/${build.repo_name}/pull/${build.pr}`"
                   target="_blank">{{"PR-"+build.pr}}
                </a>
              </span>
            </el-tooltip>
            <el-tooltip :content="`在 ${build.source} 上查看 Commit`"
                        placement="top"
                        effect="dark">
              <span v-if="build.commit_id"
                    class="link">
                <a v-if="build.source==='github'||build.source==='gitlab'"
                   :href="`${build.address}/${build.repo_owner}/${build.repo_name}/commit/${build.commit_id}`"
                   target="_blank">{{build.commit_id.substring(0, 10)}}
                </a>
                <span
                      v-else-if="build.source==='gerrit'&& (!build.pr || build.pr===0)">{{build.commit_id.substring(0, 8)}}</span>
                <span v-else-if="build.source==='gerrit'&& build.pr && build.pr!==0"
                      class="link">
                  <a :href="`${build.address}/c/${build.repo_name}/+/${build.pr}`"
                     target="_blank">{{`Change-${build.pr}`}}
                  </a>
                  {{build.commit_id.substring(0, 8)}}
                </span>
              </span>
            </el-tooltip>
          </van-col>
        </van-row>
        <template v-if="serviceType!=='pm'">
          <van-row>
            <div>
              <span>镜像信息</span>
            </div>
          </van-row>
          <van-row :gutter="0">
            <van-col :span="24">
              <el-tooltip effect="dark"
                          :content="buildv2.job_ctx.image"
                          placement="top">
                <span class="">{{buildv2.job_ctx.image.split('/').pop()}}</span>
              </el-tooltip>
            </van-col>
          </van-row>
        </template>
        <template v-if="serviceType==='pm'">
          <van-row>
            <div>
              <span>打包信息</span>
            </div>
          </van-row>
          <van-row :gutter="0">
            <van-col :span="24">
              <span class="">{{buildv2.job_ctx.package_file}}</span>
            </van-col>
          </van-row>
        </template>
        <template v-if="!$utils.isEmpty(buildv2) && buildv2.job_ctx.upload_pkg">
          <van-row>
            <div>
              <span>打包信息</span>
            </div>
          </van-row>
          <van-row :gutter="0">
            <van-col :span="24">
              <span class="">{{buildv2.job_ctx.package_file}}</span>
            </van-col>
          </van-row>
        </template>
      </div>
    </div>

    <div id="buildv2-log"
         v-if="!$utils.isEmpty(buildv2)&&buildv2.enabled"
         class="mobile-log-container">
      <xterm-log :id="buildv2.job_ctx.image"
                 :fontSize="'10'"
                 :logs="buildv2AnyLog"></xterm-log>
    </div>
  </div>
</template>

<script>
import mixin from '@utils/task_detail_mixin';
import { NoticeBar, Col, Row, Divider, Notify } from 'vant';
import { getWorkflowHistoryBuildLogAPI } from '@api';

export default {
  components: {
    [NoticeBar.name]: NoticeBar,
    [Col.name]: Col,
    [Row.name]: Row,
    [Divider.name]: Divider,
    [Notify.name]: Notify
  },
  data() {
    return {
      buildv2AnyLog: [],
      wsBuildDataBuffer: [],
      buildLogStarted: true,
    };
  },

  computed: {
    build_running() {
      return this.buildv2 && this.buildv2.status === 'running';
    },
    build_done() {
      return this.isSubTaskDone(this.buildv2);
    },
    docker_build_running() {
      return this.docker_build && this.docker_build.status === 'running';
    },
    no_docker_build() {
      return !this.docker_build;
    },

    buildOverallStatus() {
      return this.$utils.calcOverallBuildStatus(this.buildv2, this.docker_build);
    },
    buildOverallStatusZh() {
      return this.$translate.translateTaskStatus(this.buildOverallStatus);
    },
    buildOverallColor() {
      return this.$translate.calcTaskStatusColor(this.buildOverallStatus);
    },
    serviceType() {
      return this.buildv2.service_type;
    },
    envName() {
      return this.buildv2.env_name;
    }
  },
  watch: {
    build_running(val, oldVal) {
      if (!oldVal && val && this.buildLogStarted) {
        this.openBuildLog('buildv2');
      }
      if (oldVal && !val) {
        this.killLog('buildv2');
      }
    },
    docker_build_running(val, oldVal) {
      if (!oldVal && val && this.buildLogStarted) {
        this.openBuildLog('docker_build');
      }
      if (oldVal && !val) {
        this.killLog('docker_build');
      }
    },
    buildLogStarted(val, oldVal) {

    },
  },
  methods: {
    getBuildv2Log() {
      this.buildLogStarted = true;
    },
    openBuildLog(buildType) {
      let url = `/api/aslan/logs/sse/workflow/build/${this.pipelineName}/${this.taskID}/999999/${this.serviceName}?subTask=${buildType}`;
      if (this.serviceType === 'pm') {
        url = `/api/aslan/logs/sse/workflow/build/${this.pipelineName}/${this.taskID}/999999/${this.serviceName}?subTask=buildv2&envName=${this.envName}&productName=${this.projectName}`
      }
      if (typeof window.msgServer === 'undefined') {
        window.msgServer = {};
        window.msgServer[this.serviceName] = {};
      }
      this[`${buildType}IntervalHandle`] = setInterval(() => {
        if (this.hasNewMsg) {
          this.buildv2AnyLog = this.buildv2AnyLog.concat(this.wsBuildDataBuffer);
          this.wsBuildDataBuffer = [];
          const len = this.buildv2AnyLog.length;
        }
        this.hasNewMsg = false;
      }, 500);
      this.$sse(url, { format: 'plain' })
        .then(sse => {
          // Store SSE object at a higher scope
          window.msgServer[this.serviceName] = sse;
          sse.onError(e => {
            console.error('lost connection; giving up!', e);
            Notify({ type: 'danger', message: `${buildType}日志获取失败` });
            sse.close();
            this.killLog(buildType);
          });
          // Listen for messages without a specified event
          sse.subscribe('', data => {
            this.hasNewMsg = true;
            this.wsBuildDataBuffer = this.wsBuildDataBuffer.concat(Object.freeze(data + '\n'));
          });
        })
        .catch(err => {
          console.error('Failed to connect to server', err);
          Notify({ type: 'danger', message: `${buildType}日志获取失败` });
          this.killLog(buildType);
        });

    },
    getHistoryBuildLog() {
      let type = 'buildv2'
      if(this.buildv2.type === 'jenkins_build') {
        type = 'jenkins_build'
      }
      return getWorkflowHistoryBuildLogAPI(this.pipelineName, this.taskID, this.serviceName, type).then(
        response => {
          this.buildv2AnyLog = (response.split('\n')).map(element => {
            return element + '\n'
          });
        }
      );
    },
  },
  beforeDestroy() {
    this.killLog('buildv2');
    this.killLog('docker_build');
  },
  mounted() {
    if (this.build_running) {
      this.openBuildLog('buildv2');
    } else {
      if (this.build_done) {
        if (this.docker_build_running) {
          this.getHistoryBuildLog().then(() => {
            this.openBuildLog('docker_build');
          });
        } else {
          this.getHistoryBuildLog();
        }
      }
    }
  },
  props: {
    buildv2: {
      type: Object,
      required: true,
    },
    docker_build: {
      type: null,
      required: true,
    },
    isWorkflow: {
      type: Boolean,
      required: true,
    },
    serviceName: {
      type: String,
      default: '',
    },
    pipelineName: {
      type: String,
      required: true,
    },
    projectName: {
      type: String,
      default: '',
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

.mobile-task-detail-build {
  color: #303133;
  a {
    color: #1989fa;
  }
  .mobile-log-container {
    margin: 15px 0;
  }
}
</style>
