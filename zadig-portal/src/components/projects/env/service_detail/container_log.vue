<template>
  <div class="service-detail-container-log">

    <el-card class="box-card box-card-service">
      <div class="log-container">
        <div class="log-content">
          <xterm-log-container :id="podName"
                             @closeConnection="showRealTimeLog('','','close')"
                             @reConnect="refreshLog"
                             ref="log"
                             :searchKey="searchKey"
                             :logs="realtimeLog.data"></xterm-log-container>
        </div>
        <div class="log-header">
          <div class="search-log-input">
            <el-input v-model="searchKey"
                      size="small"
                      prefix-icon="el-icon-search"
                      clearable
                      placeholder="请输入关键词搜索日志"></el-input>
          </div>
          <template v-if="realtimeLog.status==='connected'">
            <el-button @click="showRealTimeLog('','','close')"
                       size="small"
                       icon="ion-android-checkbox-blank"
                       type="primary">
              停止连接
            </el-button>
            <span class="tip">接收中</span>
            <i class="el-icon-loading"></i>
          </template>
          <el-button v-else
                     @click="refreshLog"
                     size="small"
                     icon="el-icon-refresh"
                     type="primary">
            刷新日志
          </el-button>
          <el-tooltip content="去底部"
                      effect="light"
                      placement="top">
            <el-button @click="scrollToBottom"
                       class="go-to"
                       type="text"
                       icon="ion-arrow-down-c">
            </el-button>
          </el-tooltip>
          <el-tooltip content="去顶部"
                      effect="light"
                      placement="top">
            <el-button @click="scrollToTop"
                       class="go-to"
                       type="text"
                       icon="ion-arrow-up-c">
            </el-button>
          </el-tooltip>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script>
import { wordTranslate } from '@utils/word_translate';
export default {
  data() {
    return {
      currentContainer: {},
      realtimeLog: { status: '', data: [] },
      window: window,
      searchKey: ''
    };
  },
  methods: {
    wordTranslation(word, category) {
      return wordTranslate(word, category);
    },
    getLogWSUrl() {
      const hostname = window.location.hostname;
      if (this.$utils.protocolCheck() === 'https') {
        return 'wss://' + hostname;
      } else if (this.$utils.protocolCheck() === 'http') {
        return 'ws://' + hostname;
      }
    },
    showRealTimeLog(pod_name, container_name, operation) {
      if (operation == 'open') {
        if (typeof window.msgServer === 'undefined') {
          const ownerQ = this.$route.query.envName ? '&envName=' + this.$route.query.envName : '';
          this.$sse(`/api/aslan/logs/sse/pods/${pod_name}/containers/${container_name}?tails=1000&productName=${this.productName}` + ownerQ)
            .then(sse => {
              // Store SSE object at a higher scope
              window.msgServer = sse;
              this.realtimeLog.status = 'connected';
              sse.subscribe('', data => {
                this.hasNewMsg = true;
                this.realtimeLog.data.push(Object.freeze(data) + '\r\n');
              });
            })
            .catch(err => {
              console.error('Failed to connect to server', err);
              delete window.msgServer;
              clearInterval(this.intervalHandle);
            });
        }
      }
      if (operation == 'close') {
        this.realtimeLog.status = 'closed';
        if (typeof msgServer !== 'undefined' && msgServer) {
          msgServer.close();
          delete window.msgServer;
        }
        clearInterval(this.intervalHandle);
      }
    },
    openLog() {
      this.showRealTimeLog(this.podName, this.containerName, 'open');
    },
    refreshLog() {
      this.$refs.log.clearCurrentTerm();
      this.showRealTimeLog(this.podName, this.containerName, 'open');
      this.realtimeLog.data = [];
    },
    getStatusColor(status) {
      return _getStatusColor(status);
    },
    scrollToTop() {
      this.$refs.log.scrollToTop();
    },
    scrollToBottom() {
      this.$refs.log.scrollToBottom();
    }
  },
  computed: {
    productName() {
      return this.$route.params.project_name;
    },
    serviceName() {
      return this.$route.params.service_name;
    }
  },
  props: {
    podName: {
      required: true
    },
    containerName: {
      required: true
    },
    visible: {
      required: true
    }
  },
  mounted() {
    this.openLog();
  },
  destroyed() {
    this.realtimeLog.data = [];
    this.searchKey = '';
  },
  watch: {
    visible(val) {
      if (val) {
        this.searchKey = '';
        this.realtimeLog.data = [];
        this.openLog();
      } else {
        this.$refs.log.clearCurrentTerm();
        this.showRealTimeLog(this.podName, this.containerName, 'close');
        this.realtimeLog.data = [];
        this.searchKey = '';
      }
    }
  }
};
</script>

<style lang="less">
.service-detail-container-log {
  flex: 1;
  position: relative;
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
      cursor: pointer;
      color: #9ea3a9;
      &:hover {
        color: #1989fa;
      }
    }
    .icon-color-cancel {
      cursor: pointer;
      color: #ff4949;
    }
  }
  .clearfix:before,
  .clearfix:after {
    display: table;
    content: "";
  }
  .clearfix {
    span {
      line-height: 20px;
      color: #999;
      font-size: 16px;
    }
  }
  .clearfix:after {
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
        padding: 0;
        float: right;
        margin: 0 50px 0 0;
        font-size: 26px;
      }
      .scroll-switch {
        float: right;
        margin: 0 50px 0 0;
        position: relative;
        top: 6px;
        .el-switch__label--right {
          height: 16px;
        }
      }
      .search-log {
        margin-bottom: 10px;
      }
      .search-log-input {
        margin-bottom: 10px;
      }
    }
    .log-content {
      overflow-y: auto;
      &::-webkit-scrollbar-track {
        box-shadow: inset 0 0 6px rgba(0, 0, 0, 0.3);
        border-radius: 6px;
        background-color: #f5f5f5;
      }
      &::-webkit-scrollbar {
        width: 8px;
        background-color: #f5f5f5;
      }
      &::-webkit-scrollbar-thumb {
        border-radius: 6px;
        box-shadow: inset 0 0 6px rgba(0, 0, 0, 0.3);
        background-color: #555;
      }
      pre {
        clear: left;
        min-height: 42px;
        color: #f1f1f1;
        font-family: Monaco, monospace;
        font-size: 12px;
        line-height: 18px;
        white-space: pre-wrap;
        word-wrap: break-word;
        background-color: #222;
        counter-reset: line-numbering;
        margin-top: 0;
        margin-bottom: 0;
        padding-top: 8px;
        p {
          padding: 0 15px 0 16px;
          margin: 0;
          min-height: 16px;
          cursor: pointer;
          &:hover {
            background-color: #444 !important;
          }
        }
        .line-number::before {
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
      color: #ff5555;
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

  .realtime-log,
  .search-log {
    ul {
      padding: 0px;
    }
    ul > li {
      list-style: none;
      padding: 15px 0;
      font-size: 15px;
      border-top: 1px solid #e6e9f0;
      &:hover {
        background-color: #f5f5f5;
      }
    }
  }

  .value {
    font-weight: 500;
    .domain {
      margin-top: 0;
      padding: 0;
      text-decoration: none;
      list-style: none;
      color: #1989fa;
      li {
        padding-bottom: 3px;
        cursor: pointer;
      }
      li > a {
        color: #1989fa;
      }
    }
    .operation {
      padding: 0;
      margin: 0;
      list-style: none;
      li {
        float: left;
        display: inline-block;
        padding-right: 8px;
        cursor: pointer;
        i {
          color: #1989fa;
          &:hover {
            color: rgba(25, 137, 250, 0.85);
          }
        }
      }
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
    box-shadow: none;
    border: none;
  }
  .upper-card {
    margin-top: 0;
  }
  .el-card__header {
    padding: 8px 0px;
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
