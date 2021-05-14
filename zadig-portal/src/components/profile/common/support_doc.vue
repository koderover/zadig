<template>
  <div class='wrapper'
       :class="inlineStyle()">
    <div class='support-doc'>
      <a :href="urlAssign()"
         target="_blank"
         class="btn btn-xs btn-success btn-emphasis">帮助</a>
    </div>
  </div>
</template>
<script>
import docConfig from './config.js';
export default {
  props: {
    helpUrl: {
      type: String,
      default: ''
    },
    inline: {
      type: Boolean,
      default: false
    },
    keyword: {
      type: Object
    }
  },
  data: function () {
    return {};
  },
  methods: {
    inlineStyle: function () {
      if (this.inline) {
        return 'display-inline';
      }
    },
    urlAssign() {
      if (typeof this.keyword !== 'undefined') {
        const url = docConfig(this.keyword.location, this.keyword.key);
        return url;
      } else if (typeof this.helpUrl !== undefined) {
        const url = this.helpUrl;
        return url;
      }
    }
  }
};
</script>
<style lang="less" scoped>
.display-inline {
  display: inline-block;
}
.btn {
  font-weight: 300;
  transition: all 0.15s;
  padding: 0 15px;
  line-height: 32px;
  border-radius: 4px;
  display: inline-block;
  margin-bottom: 0;
  text-align: center;
  vertical-align: middle;
  touch-action: manipulation;
  cursor: pointer;
  border: 1px solid transparent;
  white-space: nowrap;
  user-select: none;
}
.btn-group-xs > .btn,
.btn-xs {
  padding: 0 6px;
  line-height: 16px;
  font-size: 12px;
}

.btn.btn-success.btn-emphasis {
  color: #fff;
  background-color: #67c23a;
  border-color: #67c23a;
}
.btn.btn-success {
  color: #67c23a;
  background-color: rgba(103, 194, 58, 0.04);
  border-color: rgba(103, 194, 58, 0.4);
}
</style>