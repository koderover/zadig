<template>
  <div class="con">
    <el-table :data="tableData" :expand-row-keys="expandRowKeys" v-loading="loading" :default-sort="defaultSort" :row-key="id" style="width: 100%">
      <el-table-column
        v-for="item in tableColumns"
        :key="item.prop"
        :prop="item.prop"
        :label="item.label"
        :width="item.width"
        :sortable="item.sortable"
        :render-header = item.renderHeader
        :align= item.align
        :type= item.type
        :sort-by = item.sortBy
      >
        <template slot-scope="scope">
          <span class="columns" v-if="item.render">
            <Render :scope="scope" :render="item.render" />
          </span>
          <span class="columns" v-else>{{ scope.row[item.prop] }}</span>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>
<script>
/*
    columns支持传组件跟jsx
    render: (scope) => {
            return (<linkUrl >12 </linkUrl>)
    }
*/

import Render from './render'

export default {
  name: 'ETable',
  components: {
    Render
  },
  props: {
    id: {
      type: String,
      default: ''
    },
    tableData: {
      type: Array,
      default: () => []
    },
    tableColumns: {
      type: Array,
      default: () => []
    },
    tableTitle: {
      type: String,
      default: ''
    },
    description: {
      type: String,
      default: ''
    },
    loading: {
      type: Boolean,
      default: false
    },
    expandRowKeys: {
      type: Array,
      default: () => []
    },
    defaultSort: Object
  },
  data () {
    return {}
  }
}
</script>
<style lang="less" scoped>
/deep/.el-table{
      font-size: 14px;
    color: #606266;
}

.con {
  width: 100%;
}
</style>
