<template>
  <div class="container">
    <div :id="id"></div>
  </div>
</template>

<script>
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import 'xterm/css/xterm.css'
export default {
  name: 'Terminal',
  data () {
    return {
      baseLog: [],
      index: 0
    }
  },
  methods: {
    clearCurrentTerm () {
      this.term.clear()
    },
    scroll (item) {
      const height = item.offsetHeight
      const top =
        item.getBoundingClientRect() && item.getBoundingClientRect().top
      const viewPortHeight =
        window.innerHeight ||
        document.documentElement.clientHeight ||
        document.body.clientHeight
      if (top + height <= viewPortHeight && top >= 60) {
        const el = document.querySelector('.workflow-task-detail').style
        el.overflow = 'hidden'
      }
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
      default: '13'
    }
  },
  watch: {
    logs: function (new_val, old_val) {
      for (let i = this.index; i < new_val.length; i++) {
        this.term.write(new_val[i] + '\r')
      }
      this.index = new_val.length
    }
  },
  mounted () {
    const term = new Terminal({ fontSize: this.fontSize, rows: '30', padding: '15', fontFamily: 'Monaco,monospace,Microsoft YaHei,Arial', disableStdin: true, scrollback: 9999999, cursorStyle: null })
    const fitAddon = new FitAddon()
    term.loadAddon(fitAddon)
    term.open(document.getElementById(this.id))
    fitAddon.fit()
    this.term = term
    const list = document.querySelectorAll('.xterm-viewport');
    [].forEach.call(list, (item) => {
      item.addEventListener('scroll', () => {
        this.scroll(item)
      })
    })
  }
}
</script>
