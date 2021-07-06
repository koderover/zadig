<template>
  <div class="con" id="content">
    <el-tree
      ref="tree"
      node-key="id"
      class="tree"
      :data="nodeData"
      :render-content="renderContent"
      :default-expanded-keys="expandKey"
      @node-contextmenu="handleNodeClick"
      @node-click="addExpandFileList"
    ></el-tree>
  </div>
</template>
<script>
const iconConfig = {
  file: 'el-icon-document',
  folder: 'el-icon-folder',
  service: 'iconfont iconrongqifuwu'
}
export default {
  name: 'left',
  props: {
    changeModalStatus: Function,
    saveFileName: Function,
    saveNewFile: Function,
    saveNewFolder: Function,
    changeExpandFileList: Function,
    laodData: Function,
    nodeData: Array,
    expandKey: Array,
    deleteServer: Function,
    openRepoModal: Function
  },
  data () {
    return {
      serviceName: null,
      showHover: {}
    }
  },
  methods: {
    renderContent (h, { node, data, store }) {
      return (
        <span class="custom-tree-node" onMouseover={() => this.setHovered(data.id)} onMouseleave={() => this.unsetHovered(data.id)}>
          {data.add || data.updateFileName
            ? (
              <span>
                <i class={iconConfig[data.type] + ' iconColor'}></i>{' '}
                <input
                  id="folder-input"
                  class="input"
                  style="outline:none;background-color: rgb(245, 247, 250); border: none;height: 26px;width: 120px"
                  size="mini"
                  value={node.label}
                  onBlur={() => this.save(data)}
                  onKeypress={(e) => this.keyDown(e, data)}
                  type="text"
                  autocomplete="off"
                ></input>
              </span>
            )
            : (
              <span style={this.showColor(data)} >
                <i class={iconConfig[data.type] + ' iconColor'}></i> {node.label}
                {data.type === 'service' && <span><el-button
                  type="text"
                  size="mini"
                  style={this.showHover[data.id] ? 'visibility: visible' : 'visibility: hidden'}
                  class="el-icon-close icon"
                  onClick = {() => this.deleteServer(data)}
                ></el-button><el-button
                  type="text"
                  size="mini"
                  style={this.showHover[data.id] ? 'visibility: visible' : 'visibility: hidden'}
                  class="el-icon-refresh icon"
                  onClick = {() => this.openRepoModal(data)}
                ></el-button></span>}
              </span>
            )}
        </span>
      )
    },
    setHovered (name) {
      this.$nextTick(() => {
        this.$set(this.showHover, name, true)
      })
    },
    unsetHovered (name) {
      this.$nextTick(() => {
        this.$set(this.showHover, name, false)
      })
    },
    showColor (data) {
      if (data.isUpdate) {
        return 'color: rgb(193,58,61)'
      }
      if (data.updated) {
        return 'color: #1989fa'
      }
    },
    addExpandFileList (data) {
      const service_name = this.$route.query.service_name
      if (data.service_name !== service_name) {
        const params = {
          projectName: this.$route.params.project_name,
          serviceName: data.service_name
        }
        this.$router.replace({
          query: Object.assign(
            {},
            {},
            {
              service_name: data.service_name,
              service_type: this.$route.query.service_type,
              rightbar: 'var'
            })
        })
        this.$store.dispatch('queryServiceModule', params)
        this.serviceName = data.service_name
      }

      if (data.type === 'file') {
        if (!data.txt) {
          const params = {
            projectName: this.$route.params.project_name,
            service: data.service_name,
            path: data.parent,
            fileName: data.label
          }
          this.$store.dispatch('queryFileContent', params).then(res => {
            if (res) {
              data.txt = res
            } else {
              data.txt = ''
            }
            this.changeExpandFileList('add', data)
          })
        } else {
          this.changeExpandFileList('add', data)
        }
      } else if (data.children && data.children.length === 0) {
        this.laodData(data)
      }
    },
    save (data) {
      const value = document.getElementById('folder-input').value
      if (data.add) {
        if (data.children) {
          this.saveNewFolder(value)
        } else {
          this.saveNewFile(value)
        }
      } else {
        this.saveFileName(value)
      }
    },
    keyDown (e, data) {
      if (e.which === 13) {
        this.save(data)
      }
    },
    remove (data) {
      this.$refs.tree.remove(data)
    },
    onFocus () {
      document.getElementById('folder-input').focus()
    },
    addClickEvent () {
      const content = document.getElementById('content')
      content.oncontextmenu = (e) => {
        e.preventDefault()
        const position = {
          left: e.clientX,
          top: e.clientY
        }

        this.changeModalStatus(position, true, null)
      }
    },
    handleNodeClick (e, data) {
      // Todo
    }
  },
  mounted () {
    // Todo Add folder
    // this.addClickEvent()
  }
}
</script>
<style lang="less" scoped>
.con {
  position: relative;
  width: 100%;
  height: 100%;
  overflow-y: scroll;
  color: #606266;

  .tree {
    margin-left: 10px;
    font-size: 13px;
    background-color: inherit;

    /deep/ .el-tree-node {
      overflow: hidden;
    }

    /deep/ .custom-tree-node {
      display: flex;

      .icon {
        margin-left: 10px;
        color: #1989fa;
        font-size: 12px;
      }
    }

    /deep/ .iconColor {
      color: #1989fa;
    }
  }

  .input {
    outline: none;
  }
}
</style>
