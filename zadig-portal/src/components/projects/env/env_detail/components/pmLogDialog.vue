<template>
  <el-dialog
    :visible.sync="visible"
    :close-on-click-modal="false"
    width="70%"
    title="服务日志"
    class="pm-log-dialog"
  >
    <span slot="title" class="modal-title">
      <span class="unimportant">服务名称:</span>
      {{ serviceName }}
    </span>
    <div class="service-detail-pm-service-log">
      <el-card class="box-card box-card-service">
        <div class="log-container">
          <div class="log-content">
            <xterm-log-container
              :id="serviceName"
              @closeConnection="showRealTimeLog('', '', 'close')"
              ref="log"
              :searchKey="searchKey"
              :logs="realtimeLog.data"
            ></xterm-log-container>
          </div>
          <div class="log-header">
            <div class="search-log-input">
              <el-input
                v-model="searchKey"
                size="small"
                prefix-icon="el-icon-search"
                clearable
                placeholder="请输入关键词搜索日志"
              ></el-input>
            </div>
            <template v-if="realtimeLog.status === 'connected'">
              <el-button
                @click="showRealTimeLog('', '', 'close')"
                size="small"
                icon="ion-android-checkbox-blank"
                type="primary"
              >
                停止连接
              </el-button>
              <span class="tip">接收中</span>
              <i class="el-icon-loading"></i>
            </template>
            <el-button
              v-else
              @click="refreshLog"
              size="small"
              icon="el-icon-refresh"
              type="primary"
            >
              刷新日志
            </el-button>
            <el-tooltip content="去底部" effect="light" placement="top">
              <el-button
                @click="scrollToBottom"
                class="go-to"
                type="text"
                icon="ion-arrow-down-c"
              >
              </el-button>
            </el-tooltip>
            <el-tooltip content="去顶部" effect="light" placement="top">
              <el-button
                @click="scrollToTop"
                class="go-to"
                type="text"
                icon="ion-arrow-up-c"
              >
              </el-button>
            </el-tooltip>
          </div>
        </div>
      </el-card>
    </div>
  </el-dialog>
</template>

<script>
export default {
  data () {
    return {
      currentContainer: {},
      realtimeLog: { status: '', data: [] },
      window: window,
      searchKey: '',
      visible: false,
      envName: null,
      serviceName: null
    }
  },
  methods: {
    openDialog (envName, serviceName) {
      this.visible = true
      this.envName = envName
      this.serviceName = serviceName
    },
    getLogWSUrl () {
      const hostname = window.location.hostname
      if (this.$utils.protocolCheck() === 'https') {
        return 'wss://' + hostname
      } else if (this.$utils.protocolCheck() === 'http') {
        return 'ws://' + hostname
      }
    },
    showRealTimeLog (envName, serviceName, operation) {
      const projectName = this.projectName
      if (operation === 'open') {
        if (typeof window.msgServer === 'undefined') {
          this.$sse(
            `/api/aslan/logs/sse/service/build/${serviceName}/${envName}/${projectName}?subTask=buildv2&lines=9999`
          )
            .then((sse) => {
              // Store SSE object at a higher scope
              window.msgServer = sse
              this.realtimeLog.status = 'connected'
              sse.subscribe('message', (data) => {
                this.hasNewMsg = true
                this.realtimeLog.data.push(Object.freeze(data) + '\r\n')
              })
              sse.subscribe('job-status', (data) => {
                if (data === 'completed') {
                  this.realtimeLog.status = 'closed'
                  if (typeof msgServer !== 'undefined' && msgServer) {
                    delete window.msgServer
                  }
                  sse.close()
                }
              })
            })
            .catch((err) => {
              console.error('Failed to connect to server', err)
              delete window.msgServer
              clearInterval(this.intervalHandle)
            })
        }
      }
      if (operation === 'close') {
        this.realtimeLog.status = 'closed'
        if (typeof msgServer !== 'undefined' && msgServer) {
          msgServer.close()
          delete window.msgServer
        }
        clearInterval(this.intervalHandle)
      }
    },
    openLog () {
      this.showRealTimeLog(this.envName, this.serviceName, 'open')
    },
    refreshLog () {
      this.$refs.log.clearCurrentTerm()
      this.showRealTimeLog(this.envName, this.serviceName, 'open')
      this.realtimeLog.data = []
    },
    scrollToTop () {
      this.$refs.log.scrollToTop()
    },
    scrollToBottom () {
      this.$refs.log.scrollToBottom()
    }
  },
  computed: {
    projectName () {
      return this.$route.params.project_name
    }
  },
  created () {
    this.wsDataBuffer = []
  },
  mounted () {
    // visible=true之后这个组件才被create，这个true `watch`不到
    // this.openLog()
  },
  destroyed () {
    this.realtimeLog.data = []
    this.searchKey = ''
  },
  watch: {
    visible (val) {
      if (val) {
        this.searchKey = ''
        this.realtimeLog.data = []
        this.openLog()
      } else {
        this.$refs.log.clearCurrentTerm()
        this.showRealTimeLog(this.envName, this.serviceName, 'close')
        this.realtimeLog.data = []
        this.searchKey = ''
      }
    }
  }
}
</script>

<style lang="less">
.service-detail-pm-service-log {
  position: relative;
  flex: 1;
  overflow: auto;
  font-size: 13px;

  .el-breadcrumb {
    font-size: 16px;
    line-height: 1.35;

    .el-breadcrumb__item__inner a:hover,
    .el-breadcrumb__item__inner:hover {
      color: #1989fa;
      cursor: pointer;
    }
  }

  .xterm {
    padding: 10px 15px;
  }

  .text {
    font-size: 13px;
  }

  .item {
    padding: 10px 0;
    padding-left: 1px;

    .icon-color {
      color: #9ea3a9;
      cursor: pointer;

      &:hover {
        color: #1989fa;
      }
    }

    .icon-color-cancel {
      color: #ff4949;
      cursor: pointer;
    }
  }

  .clearfix::before,
  .clearfix::after {
    display: table;
    content: '';
  }

  .clearfix {
    span {
      color: #999;
      font-size: 16px;
      line-height: 20px;
    }
  }

  .clearfix::after {
    clear: both;
  }

  .alert-warning {
    position: relative;
  }

  .log-container {
    .log-header {
      margin: 0;
      padding: 0.5em 0.8em 0.4em;
      text-align: left;
      background-color: #dfe5ec;

      .tip {
        color: #999;
      }

      .go-to {
        float: right;
        margin: 0 50px 0 0;
        padding: 0;
        font-size: 26px;
      }

      .scroll-switch {
        position: relative;
        top: 6px;
        float: right;
        margin: 0 50px 0 0;

        .el-switch__label--right {
          height: 16px;
        }
      }

      .search-log-input {
        margin-bottom: 10px;
      }
    }

    .log-content {
      overflow-y: auto;

      &::-webkit-scrollbar-track {
        background-color: #f5f5f5;
        border-radius: 6px;
        box-shadow: inset 0 0 6px rgba(0, 0, 0, 0.3);
      }

      &::-webkit-scrollbar {
        width: 8px;
        background-color: #f5f5f5;
      }

      &::-webkit-scrollbar-thumb {
        background-color: #555;
        border-radius: 6px;
        box-shadow: inset 0 0 6px rgba(0, 0, 0, 0.3);
      }

      pre {
        clear: left;
        min-height: 42px;
        margin-top: 0;
        margin-bottom: 0;
        padding-top: 8px;
        color: #f1f1f1;
        font-size: 12px;
        font-family: Monaco, monospace;
        line-height: 18px;
        white-space: pre-wrap;
        word-wrap: break-word;
        background-color: #222;
        counter-reset: line-numbering;

        p {
          min-height: 16px;
          margin: 0;
          padding: 0 15px 0 16px;
          cursor: pointer;

          &:hover {
            background-color: #444 !important;
          }
        }
      }
    }
  }

  .ansi {
    .black {
      color: #4e4e4e;
    }

    .black.bold {
      color: #7c7c7c;
    }

    .red {
      color: #f55;
    }

    .red.bold {
      color: #ff9b93;
    }

    .green {
      color: #f8f8f2;
      background-color: #39aa56;
    }

    .green.bold {
      color: #b1fd79;
    }

    .yellow {
      color: #f1fa8c;
    }

    .yellow.bold {
      color: #f1fa8c;
    }

    .blue {
      color: #8be9fd;
    }

    .blue.bold {
      color: #b5dcfe;
    }

    .magenta {
      color: #ff73fd;
    }

    .magenta.bold {
      color: #ff9cfe;
    }

    .cyan {
      color: #5ff;
    }

    .white {
      color: #eee;
    }

    .white.bold {
      color: #fff;
    }

    .grey {
      color: #969696;
    }

    .bg-black {
      background-color: #4e4e4e;
    }

    .bg-red {
      background-color: #ff6c60;
    }

    .bg-green {
      background-color: #0a0;
    }

    .bg-yellow {
      background-color: #ffffb6;
    }

    .bg-blue {
      background-color: #96cbfe;
    }

    .bg-magenta {
      background-color: #ff73fd;
    }

    .bg-cyan {
      background-color: #0aa;
    }

    .bg-white {
      background-color: #eee;
    }
  }

  .box-card {
    width: 480px;
  }

  .box-card-service {
    width: 100%;
  }

  .box-card,
  .box-card-service {
    margin-top: 0;
    border: none;
    box-shadow: none;
  }

  .upper-card {
    margin-top: 0;
  }

  .el-card__header {
    padding: 8px 0;
  }

  .el-card__body {
    padding: 0;
  }

  .el-table {
    color: #445262;
  }

  .el-row {
    margin-bottom: 20px;

    &:last-child {
      margin-bottom: 0;
    }
  }

  .el-table .info-row {
    background: #c9e5f5;
  }

  .el-table .positive-row {
    background: #e2f0e4;
  }
}
</style>
