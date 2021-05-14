<template>
  <div class="container">
    <div :id="id"></div>
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
    id: {
      required: true,
      type: String
    },
    fontSize: {
      required: false,
      default: "13"
    }
  },
  watch: {
    logs: function (new_val, old_val) {
      for (let i = this.index; i < new_val.length; i++) {
        this.term.write(new_val[i] + '\r');
      }
      this.index = new_val.length;
    },
    id: function (new_val, old_val) {
    }
  },
  mounted() {
    const term = new Terminal({ fontSize: this.fontSize, rows: "30", padding: "15", fontFamily: "Monaco,monospace,Microsoft YaHei,Arial", disableStdin: true, scrollback: 9999999, cursorStyle: null });
    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.open(document.getElementById(this.id));
    fitAddon.fit();
    this.term = term;
  }
};
</script>
