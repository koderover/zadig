<template>
  <div class="task-detail-test">
    <el-card v-if="!$utils.isEmpty(testingv2) && testingv2.enabled"
             class="box-card task-process"
             :body-style="{ padding: '0px', margin: '15px 0 0 0' }">
      <div class="error-wrapper">
        <el-alert v-if="testingv2.error"
                  title="错误信息"
                  :description="testingv2.error"
                  type="error"
                  close-text="知道了">
        </el-alert>
      </div>
      <div slot="header"
           class="clearfix subtask-header">
        <span>测试</span>
        <div v-if="testingv2.status==='running'"
             class="loader">
          <div class="ball-scale-multiple">
            <div></div>
            <div></div>
            <div></div>
          </div>
        </div>
      </div>
      <div class="text item">
        <el-row :gutter="0">
          <el-col :span="6">
            <div class="grid-content item-title">
              <i class="iconfont iconzhuangtai"></i> 任务状态
            </div>
          </el-col>
          <el-col :span="6">
            <div class="grid-content item-desc">
              <a href="#testv2-log"
                 :class="$translate.calcTaskStatusColor(testingv2.status,'pipeline','task')">
                {{testingv2.status?$translate.translateTaskStatus(testingv2.status):"未运行"}}
              </a>
            </div>
          </el-col>
          <el-col :span="6">
            <div class="grid-content item-title">
              <i class="iconfont icontest"></i> 测试任务
            </div>
          </el-col>
          <el-col :span="6">
            <div class="grid-content item-desc">{{ serviceName }}</div>
          </el-col>
        </el-row>
        <el-row :gutter="0"
                v-for="(build,index) in testingv2.job_ctx.builds"
                :key="index">
          <el-col :span="6">
            <div class="grid-content item-title">
              <i class="iconfont icondaima"></i> 代码库({{build.source}})
            </div>
          </el-col>
          <el-col :span="6">
            <div class="grid-content item-desc">{{build.repo_name}}
            </div>
          </el-col>
          <el-col :span="6">
            <div class="grid-content item-title">
              <i class="iconfont iconinfo"></i> 构建信息
            </div>
          </el-col>
          <el-col :span="6">
            <el-tooltip :content="`在 ${build.source} 上查看 Branch`"
                        placement="top"
                        effect="dark">
              <span v-if="build.branch"
                    class="link">
                <a v-if="build.source==='github'"
                   :href="`${build.address}/${build.repo_owner}/${build.repo_name}/tree/${build.branch}`"
                   target="_blank">{{"Branch-"+build.branch}}
                </a>
                <a v-if="build.source==='gitlab'"
                   :href="`${build.address}/${build.repo_owner}/${build.repo_name}/tree/${build.branch}`"
                   target="_blank">{{"Branch-"+build.branch}}
                </a>
                <a v-if="!build.source"
                   :href="`${build.address}/${build.repo_owner}/${build.repo_name}/tree/${build.branch}`"
                   target="_blank">{{"Branch-"+build.branch}}
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
                   target="_blank">{{"Commit-"+build.commit_id.substring(0, 10)}}
                </a>
                <span
                      v-else-if="build.source==='gerrit'&& (!build.pr || build.pr===0)">{{'Commit-'+build.commit_id.substring(0, 8)}}</span>
                <span v-else-if="build.source==='gerrit'&& build.pr && build.pr!==0"
                      class="link">
                  <a :href="`${build.address}/c/${build.repo_name}/+/${build.pr}`"
                     target="_blank">{{`Change-${build.pr}`}}
                  </a>
                  {{build.commit_id.substring(0, 8)}}
                </span>
              </span>
            </el-tooltip>
          </el-col>
        </el-row>
        <el-row :gutter="0">
          <el-col v-if="testingv2.status!=='running'"
                  :span="6">
            <div class="grid-content item-title">
              <i class="iconfont iconshijian"></i> 持续时间
            </div>
          </el-col>
          <el-col v-if="testingv2.status!=='running'"
                  :span="6">
            <span class="item-desc">{{$utils.timeFormat(testingv2.end_time -
              testingv2.start_time)}}</span>
          </el-col>
        </el-row>
      </div>
    </el-card>

    <el-card id="testv2-log"
             v-if="!$utils.isEmpty(testingv2)&&testingv2.enabled"
             class="box-card task-process"
             :body-style="{ padding: '0px', margin: '15px 0 0 0' }">
      <div class="log-container">
        <div class="log-content">

          <xterm-log :id="`${pipelineName}-${taskID}-${serviceName}`"
                     @mouseenter.native="enterLog"
                     @mouseleave.native="leaveLog"
                     :logs="testAnyLog"></xterm-log>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script>
import mixin from '@utils/task_detail_mixin';
import { getWorkflowHistoryTestLogAPI } from '@api';

export default {
  data() {
    return {
      testAnyLog: [],
      wsTestDataBuffer: [],
      testLogStarted: false,
    };
  },
  computed: {
    test_running() {
      return this.testingv2 && this.testingv2.status === 'running';
    },
    test_done() {
      return this.isSubTaskDone(this.testingv2);
    },
    testName() {
      return this.testingv2 && this.testingv2.test_name;
    },
  },
  watch: {
    test_running(val, oldVal) {
      if (!oldVal && val && this.testLogStarted) {
        this.openTestLog();
      }
      if (oldVal && !val) {
        this.killTestLog();
      }
    },
    testLogStarted(val, oldVal) {

    }
  },
  methods: {
    enterLog(e) {
      let el = document.querySelector(".workflow-task-detail").style;
      el.overflow = 'hidden';
    },
    leaveLog() {
      let el = document.querySelector(".workflow-task-detail").style;
      el.overflow = 'auto';
    },
    openTestLog() {
      if (typeof window.msgServer === 'undefined') {
        window.msgServer = {};
      }
      if (typeof window.msgServer[this.serviceName] === 'undefined') {
        this.testIntervalHandle = setInterval(() => {
          if (this.hasNewTestMsg) {
            this.testAnyLog = this.testAnyLog.concat(this.wsTestDataBuffer);
            this.wsTestDataBuffer = [];
            const len = this.testAnyLog.length;
          }
          this.hasNewTestMsg = false;
        }, 500);
        const url = `/api/aslan/logs/sse/workflow/test/${this.pipelineName}/${this.taskID}/${this.testName}/999999/${this.serviceName}`;
        this.$sse(url, { format: 'plain' }).then(sse => {
          // Store SSE object at a higher scope
          window.msgServer[this.serviceName] = sse;
          sse.onError(e => {
            console.error('lost connection; giving up!', e);
            this.$message({
              message: `test日志获取失败`,
              type: 'error'
            });
            sse.close();
            this.killTestLog();
          });
          // Listen for messages without a specified event
          sse.subscribe('', data => {
            this.hasNewTestMsg = true;
            this.wsTestDataBuffer = this.wsTestDataBuffer.concat(Object.freeze(data + '\n'));
          });
        }).catch(err => {
          console.error('Failed to connect to server', err);
          delete window.msgServer;
          clearInterval(this.testIntervalHandle);
        });
      }
    },
    killTestLog() {
      this.killLog('test');
    },
    getTestLog() {
      this.testLogStarted = true;
    }
  },
  mounted() {
    if (this.test_running) {
      this.openTestLog();
    }
    if (this.test_done) {
        getWorkflowHistoryTestLogAPI(this.pipelineName, this.taskID, this.testName, this.serviceName).then(
          response => {
            this.testAnyLog = (response.split('\n')).map(element => {
              return element + '\n'
            });
          }
        );
    }
  },
  beforeDestroy() {
    this.killTestLog();
  },
  props: {
    testingv2: {
      type: Object,
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

.task-detail-test {
  .viewlog {
    font-size: 14px;
  }
}
</style>
