<template>
  <div class="container">
    <div id="term-container"></div>
  </div>
</template>

<script>
import { Terminal } from "xterm";
import { FitAddon } from 'xterm-addon-fit';
import "xterm/css/xterm.css";
import _ from 'lodash';
export default {
  name: "Terminal",
  data() {
    return {
      baseLog: [],
      index: 0,
    };
  },
  methods: {
    clearCurrentTerm() {
      this.term.clear();
    }
  },
  props: {
    logs: {
      required: true,
      type: Array
    },
    stepStatus: {
      required: true,
      type: String
    },
    pipelineStatus: {
      required: true,
      type: String
    },
    currentStep: {
      required: true,
      type: Object
    }
  },
  watch: {
    logs: function (new_val, old_val) {
      if (new_val !== old_val) {
        this.term.clear();
        this.index = 0;
      }
      for (let i = this.index; i < new_val.length; i++) {
        this.term.write(new_val[i] + '\r');
      }
      this.index = new_val.length;
    },
    currentStep: function (new_val, old_val) {
    }
  },
  mounted() {
    const term = new Terminal({ fontSize: "12", rows: "30", padding: "15", fontFamily: "Monaco,monospace,Microsoft YaHei,Arial", disableStdin: true, scrollback: 9999999, cursorStyle: null });
    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.open(document.getElementById(
      "term-container"
    ));
    fitAddon.fit();
    this.term = term;

    if (this.pipelineStatus && (this.pipelineStatus !== 'pending' || this.pipelineStatus !== 'running')) {
      this.logs.forEach(element => {
        element && this.term.write(element + '\r');
      });
      this.index = this.logs.length;
    }
  }
};
</script>

<style lang="less">
.xterm {
  padding: 15px 10px;
  .xterm-viewport {
    border-radius: 5px;
  }
}
</style>