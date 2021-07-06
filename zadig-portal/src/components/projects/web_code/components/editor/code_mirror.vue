<template>
  <div class="con">
     <CodeMirror  @input="changeTxt" ref='codeMirror' :value="msg" :options="options" class="code" />
  </div>
</template>

<script>
import { codemirror } from 'vue-codemirror'
import 'codemirror/lib/codemirror.css'
import 'codemirror/mode/yaml/yaml.js'
import 'codemirror/theme/neo.css'
import _ from 'lodash'

export default {
  name: 'Code',
  components: {
    CodeMirror: codemirror
  },
  props: {
    currentCode: Object,
    changeCodeTxtCache: Function,
    saveFile: Function
  },
  data () {
    return {
      msg: '',
      options: {
        tabSize: 2,
        theme: 'neo',
        mode: 'text/yaml',
        lineNumbers: true,
        line: true
      }
    }
  },
  methods: {
    changeTxt: _.debounce(function (newCode) {
      if (newCode !== this.msg) {
        this.changeCodeTxtCache(newCode)
        this.saveFile()
      }
    }, 500),
    addEvent () {
      window.addEventListener('keydown', this.saveCode, false)
    },
    removeEvent () {
      window.removeEventListener('keydown', this.saveCode)
    },
    saveCode (e) {
      if (e.keyCode === 83 && (navigator.platform.match('Mac') ? e.metaKey : e.ctrlKey)) {
        e.preventDefault()
        this.saveFile()
      }
    }
  },
  watch: {
    currentCode: {
      handler () {
        if (this.currentCode.isUpdate) {
          this.msg = this.currentCode.cacheTxt
        } else {
          this.msg = this.currentCode.txt
        }
      },
      immediate: true
    }
  },
  mounted () {
    this.addEvent()
  },
  beforeDestroy () {
    this.removeEvent()
  }
}
</script>

<style lang="less" scoped>
.con {
  width: 100%;
  height: 100%;
  font-size: 14px;

  .code {
    width: 100%;
    height: 100%;
  }
}

/deep/ .cm-s-neo .CodeMirror-linenumber {
  color: rgb(0, 108, 134);
}

/deep/ .cm-s-neo.CodeMirror {
  height: 100%;
  padding-top: 10px;
}

/deep/ .CodeMirror-sizer {
  padding-left: 10px;
}
</style>
