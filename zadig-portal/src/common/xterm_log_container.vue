<template>
  <div class="container">
    <div :id="id"></div>
  </div>
</template>

<script>
import { Terminal } from "xterm";
import { FitAddon } from 'xterm-addon-fit';
import { SearchAddon } from 'xterm-addon-search';
import "xterm/css/xterm.css";
import _ from 'lodash';
export default {
  name: "Terminal",
  data() {
    return {
      baseLog: [],
      index: 0
    };
  },
  methods: {
    clearCurrentTerm() {
      this.term.clear();
    },
    scrollToTop() {
      this.term.scrollToTop();
    },
    scrollToBottom() {
      this.term.scrollToBottom();
    },
    filterStrArrayByKey(key, arr) {
      const reg = new RegExp(key, "gi");
      if (!key || key === '') {
        return arr;
      }
      else {
        arr = arr.filter(item => {
          const keyLowerCase = key.toLowerCase();
          const itemLowerCase = item.toLowerCase();
          if (itemLowerCase.toString().indexOf(keyLowerCase) >= 0) {
            return true;
          }
        });
        let newArr = arr.map(item => {
          if (item.replaceAll) {
            return item.replaceAll(reg, function (match, pos, orginText) {
              return `\u001b[0;30;103m${match}\u001b[0m`;
            });
          }
          else {
            return item.replace(reg, function (match) {
              return `\u001b[0;30;103m${match}\u001b[0m`;
            });
          }
        });
        return newArr;
      }

    },
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
    searchKey: {
      required: false,
      type: String,
      default: ''
    },
  },
  watch: {
    logs: _.debounce(function (new_val, old_val) {
      this.baseLog = _.cloneDeep(new_val);
      if (!this.searchKey) {
        for (let i = this.index; i < new_val.length; i++) {
          this.term.write(new_val[i] + '\r');
        }
        this.index = new_val.length;
      }
    }, 10),
    searchKey: _.debounce(function (new_val, old_val) {
      if (new_val) {
        this.$emit('closeConnection')
        this.clearCurrentTerm();
        let filterLogs = [];
        filterLogs = this.filterStrArrayByKey(new_val, this.baseLog);
        if (filterLogs.length > 0) {
          for (let i = 0; i < filterLogs.length; i++) {
            this.term.write(filterLogs[i] + '\r');
          }
        }
      }
      else if (new_val === '') {
        this.$emit('reConnect');
        for (let i = 0; i < this.baseLog.length; i++) {
          this.term.write(this.baseLog[i] + '\r');
        }
      }

    }
      , 500)
  },
  computed: {

  },
  destroyed() {
    this.term.clear();
  },
  mounted() {
    const term = new Terminal({ fontSize: "12", padding: "15", fontFamily: "Monaco,monospace,Microsoft YaHei,Arial", disableStdin: true, scrollback: 9999999, cursorStyle: null });
    const fitAddon = new FitAddon();
    const searchAddon = new SearchAddon();
    term.loadAddon(searchAddon);
    term.loadAddon(fitAddon);
    term.open(document.getElementById(this.id));
    fitAddon.fit();
    window.onresize = () => {
      document.getElementById(this.id).style.height = window.innerHeight- 90 + 'px';
      fitAddon.fit();
    };
    this.term = term;
  }
};
</script>
