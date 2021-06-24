<template>
  <div class="resize"
       ref="resize"
       :style="{width: containerWidth, height: height, resize: resize}">
    <slot></slot>
  </div>
</template>

<script>
import { debounce } from 'lodash'
export default {
  name: 'resize',
  data () {
    return {
      debounceResize: null,
      containerWidth: ''
    }
  },
  props: {
    height: {
      type: String,
      default: '250px'
    },
    width: {
      type: String,
      default: '100%'
    },
    resize: {
      type: String,
      default: 'vertical'
    }
  },
  methods: {
    observerSize () {
      const observer = new ResizeObserver(debounce((entries) => {
        entries.forEach(entry => {
          const { width, height } = entry.contentRect
          if (width) {
            this.$emit('sizeChange', { width, height })
            this.$children[0].editor && this.$children[0].editor.resize()
          }
        })
      }, 200))
      observer.observe(this.$refs.resize)
    },
    handleWindowsResize (event) {
      this.containerWidth = this.$refs.resize.parentNode.clientWidth + 'px'
      this.$children[0].editor && this.$children[0].editor.resize()
      this.$emit('sizeChange', { width: this.containerWidth, height: this.height })
    }
  },
  mounted () {
    this.containerWidth = this.width
    this.observerSize()
    if (this.resize !== 'vertical') {
      this.debounceResize = debounce(this.handleWindowsResize, 200)
      window.addEventListener('resize', this.debounceResize)
    }
  },
  beforeDestroy () {
    if (this.resize !== 'vertical') {
      window.removeEventListener('resize', this.debounceResize)
    }
  }
}
</script>

<style lang="less" scoped>
.resize {
  position: relative;
  box-sizing: border-box;
  min-width: 200px;
  max-width: 100%;
  min-height: 100px;
  padding: 6px;
  overflow: hidden;
  border: 1px solid #ccc;
  border-radius: 2px;

  &::after {
    position: absolute;
    right: -2px;
    bottom: -4px;
    width: 5px;
    height: 5px;
    border-top: 4px double transparent;
    border-bottom: 4px double transparent;
    border-left: 4px double #666;
    transform: rotate(45deg);
    content: " ";
  }
}
</style>
