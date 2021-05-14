<template>
  <div class="service-detail-container-exec">

    <el-card class="box-card box-card-service">
      <div class="log-container">
        <div class="log-content">
          <div :id="id"></div>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script>
import { Terminal } from "xterm";
import { FitAddon } from 'xterm-addon-fit';
import "xterm/css/xterm.css";
import _ from 'lodash';
export default {
  name: "Exec",
  data() {
    return {
    };
  },
  methods: {
    getLogWSUrl() {
      const host = window.location.host;
      if (this.$utils.protocolCheck() === 'https') {
        return 'wss://' + host;
      } else if (this.$utils.protocolCheck() === 'http') {
        return 'ws://' + host;
      }
    },
    clearCurrentTerm() {
      this.term.clear();
    },
    getLogWSUrl() {
      const host = window.location.host;
      if (this.$utils.protocolCheck() === 'https') {
        return 'wss://' + host;
      } else if (this.$utils.protocolCheck() === 'http') {
        return 'ws://' + host;
      }
    },
    initTerm() {
      var wsLink = false;
      const hostname = this.getLogWSUrl();
      const url = `/api/podexec/${this.productName}/${this.namespace}/${this.podName}/${this.containerName}/podExec`;
      this.ws = new WebSocket(hostname + url);

      this.$nextTick(() => { 
        this.term = new Terminal({ fontSize: "12", fontFamily: "Monaco,monospace", scrollback: 9999999 });
        const fitAddon = new FitAddon();
        this.term.loadAddon(fitAddon);
        this.term.open(document.getElementById(this.id));
        this.term.writeln("****************系统信息：正在连接容器****************");
        this.term.onData(data => {
          if (wsLink) {
            this.ws.send(JSON.stringify({ "operation": "stdin", "data": data }));
          }
        });

        window.onresize = function () {
          fitAddon.fit();
        };
        this.term.onResize((size) => {
          const msg = {
            operation: "resize",
            cols: size.cols,
            rows: size.rows
          }
          if (wsLink) {
            this.ws.send(JSON.stringify(msg));
          }
        });
        this.ws.onopen = (evt) => {
          const setEnv = {
            operation: "stdin",
            data: "bash \r"
          }
          this.ws.send(JSON.stringify(setEnv));
          this.term.clear();
          this.term.writeln("\u001b[32;1m****************系统信息：容器连接已打开****************\u001b[0m");
          this.term.writeln('欢迎使用 Pod 调试功能，通过模拟终端的方式，方便快速进入容器进行调试。(注意：默认连接的 Shell 为 Bash)');
          wsLink = true;
          fitAddon.fit();
        }
        this.ws.onmessage = (evt) => {
          this.$nextTick(() => { this.term.write((JSON.parse(evt.data)['data'])); })
        }
        this.ws.onclose = (evt) => {
          wsLink = false;
          this.$nextTick(() => { this.term.writeln("\u001b[31m****************系统信息：容器连接已关闭，请关闭窗口重试！****************\u001b[0m"); });
        }
        this.ws.onerror = (evt) => {
          wsLink = false;
          this.$nextTick(() => { this.term.writeln(`\u001b[31m****************系统信息：遇到错误 ${evt.message} ！！！，请关闭窗口重试 ****************\u001b[0m`); });
        }
      })
    }
  },

  props: {
    id: {
      required: true,
      type: String
    },
    visible: {
      required: true,
      type: Boolean
    },
    podName: {
      required: true,
      type: String
    },
    productName: {
      required: true,
      type: String
    },
    containerName: {
      required: true,
      type: String
    },
    serviceName: {
      required: true,
      type: String
    },
    namespace: {
      required: true,
      type: String
    }
  },
  created() {

  },
  mounted() {
    this.initTerm();

  },
  beforeDestroy() {
    this.term.dispose();
    if (typeof this.ws !== 'undefined' && this.ws) {
        this.ws.close();
        delete this.ws;
    }
  },
  watch: {
    visible(val) {
      if (val) {
        this.initTerm();
      } else if (!val) {
        if (typeof this.ws !== 'undefined' && this.ws) {
            this.ws.close();
            delete this.ws;
        }
        this.term.dispose();

      }
    }
  }
};
</script>

<style lang="less">
.service-detail-container-exec {
  flex: 1;
  position: relative;
  overflow: auto;
  font-size: 13px;
  .xterm {
    padding: 15px 10px;
  }
  .el-breadcrumb {
    font-size: 16px;
    line-height: 1.35;
    .el-breadcrumb__item__inner a:hover,
    .el-breadcrumb__item__inner:hover {
      color: #1989fa;
      cursor: pointer;
    }
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
    }
    .log-content {
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
