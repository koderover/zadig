<template>
  <div class="content">
    <div class="menu">
      <el-button
        @click="newService()"
        size="mini"
        icon="el-icon-plus"
        plain
        circle
      >
      </el-button>
    </div>
    <el-tree class="tree" node-key="service_name" ref="tree" empty-text="" highlight-current :data="serviceList" @node-click="updateService"  >
      <span
        class="custom-tree-node"
        slot-scope="{ data }"
        @mouseover="setHovered(data.service_name)"
        @mouseleave="unsetHovered(data.service_name)"
      >
        <i class="iconfont iconwuliji"></i>
        <span class="label">{{ data.service_name }}</span>
          <el-button
            @click="removeTemplate(data)"
            type="text"
            size="mini"
            icon="el-icon-close"
            :style="
              showHover[data.service_name]
                ? 'visibility: visible'
                : 'visibility: hidden'
            "
          >
          </el-button>
      </span>
    </el-tree>
    <div class="input" v-if="showAddInput">
        <el-input v-model="serviceName"  oninput="value=value.replace(/[^a-zA-Z0-9-_]/g,'')" size="small" @blur="blur" @change="change" ref="input"></el-input>
    </div>
  </div>
</template>
<script>
import { deleteServiceTemplateAPI, getServiceTemplatesAPI } from '@api'

function compare (a, b) {
  if (a.service_name < b.service_name) {
    return -1
  }
  if (a.service_name > b.service_name) {
    return 1
  }
  return 0
}

export default {
  name: 'service_list',
  props: {
    editService: Function,
    addService: Function,
    changeShowBuild: Function
  },
  data () {
    return {
      serviceName: '',
      showAddInput: false,
      serviceList: [],
      showHover: {},
      currentNodeKey: null
    }
  },
  methods: {
    blur () {
      if (!this.serviceName) {
        this.showAddInput = false
      }
    },
    change (value) {
      if (value) {
        this.serviceList.push({ service_name: value, type: 'addService' })
        this.addService({ service_name: value, type: 'addService' })
      }
      this.showAddInput = false
      this.serviceName = ''
    },
    updateService (obj) {
      if (obj.type && obj.type === 'addService') {
        this.addService(obj)
      } else {
        this.editService(obj)
      }
    },
    newService () {
      this.showAddInput = true
      this.changeShowBuild(true)
      this.$nextTick(() => {
        this.$refs.input.focus()
      })
    },
    getServiceTemplates () {
      const projectName = this.$route.params.project_name
      const serviceName = this.$route.query.serviceName
      getServiceTemplatesAPI(projectName).then((res) => {
        this.serviceList = res.data
        this.serviceList.sort(compare)
        if (res.data.length !== 0) {
          if (serviceName) {
            const item = res.data.find(item => item.service_name === serviceName)
            this.editService(item)
            this.currentNodeKey = serviceName
          } else {
            this.editService(res.data[0])
            this.currentNodeKey = res.data[0].service_name
          }
          this.$nextTick(() => {
            this.$refs.tree.setCurrentKey(this.currentNodeKey)
          })
        } else {
          this.changeShowBuild(false)
        }
      })
    },
    removeTemplate (obj) {
      let deleteText = ''
      let title = '确认'
      title = `确定要删除 ${obj.service_name} 这个服务吗？`
      deleteText = `删除操作只会删除系统中服务的定义，如需物理删除，请登陆服务器做服务清理`
      this.$confirm(`${deleteText}`, `${title}`, {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        deleteServiceTemplateAPI(
          obj.service_name,
          obj.type,
          this.projectName,
          obj.visibility
        ).then(() => {
          this.$message({
            type: 'success',
            message: '删除成功'
          })
          this.getServiceTemplates()
        })
      })
    },
    setHovered (name) {
      this.$set(this.showHover, name, true)
    },
    unsetHovered (name) {
      this.$set(this.showHover, name, false)
    }
  },
  computed: {
    projectName () {
      return this.$route.params.project_name
    }
  },
  created () {
    this.getServiceTemplates()
  }
}
</script>
<style lang="less" scoped>
/deep/ .el-tree__empty-block {
  min-height: 0;
}

.content {
  width: 250px;
  background-color: #fff;
  border-right: 1px solid rgb(230, 230, 230);

  .menu {
    flex: 0 0 auto;
    height: 50px;
    padding-right: 8px;
    line-height: 50px;
    text-align: right;
    border-bottom: 1px solid rgb(230, 230, 230);
  }

  .tree {
    max-height: calc(~"100% - 95px") !important;
    margin-top: 10px;
    overflow: scroll;
  }

  .input {
    width: 200px;
    margin: auto;
  }
}

.label {
  display: inline-block;
  min-width: 100px;
  font-size: 13px;
}
</style>
