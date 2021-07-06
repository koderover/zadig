<template>
  <div class="content" v-if="expandFileList.length">
      <div class="item " @click="changePage(index, item)" :style="currentPage === index ? 'background-color:rgb(255, 255, 255)' : ''" :key="index" v-for="(item,index) in expandFileList">
         <i :class="item.isUpdate ? 'el-icon-edit iconColor' : 'el-icon-document iconColor'"></i> <span class="label">{{item.label}} </span><span class="right"> <i @click.stop="closeCurrentPage(item, index)" :style="currentPage === index ? 'display: block' : ''" class="el-icon-close icon"></i></span>
      </div>
  </div>
</template>
<script>
export default {
  name: 'pageNav',
  props: {
    expandFileList: Array,
    changeExpandFileList: Function,
    setData: Function,
    closePageNoSave: Function,
    saveFile: Function
  },
  data () {
    return {
      currentPage: 0
    }
  },
  methods: {
    closeCurrentPage (item, index) {
      if (item.isUpdate) {
        this.$confirm('检测到未保存的内容，是否在离开页面前保存修改？或者使用command+s保存修改', '提示', {
          distinguishCancelAndClose: true,
          confirmButtonText: '保存',
          cancelButtonText: '放弃修改',
          type: 'warning'
        }).then(() => {
          this.saveFile()
        }).catch(action => {
          if (action === 'cancel') {
            this.closePageNoSave()
            this.changeExpandFileList('del', item)
          }
        })
      } else {
        this.closePageNoSave()
        this.changeExpandFileList('del', item)
      }
    },
    changePage (index, item) {
      this.currentPage = index
      if (this.expandFileList.length > 0) {
        this.setData('currentCode', item)
      }
    }
  }
}
</script>
<style lang="less" scoped>
.content {
  position: relative;
  display: flex;
  align-items: center;
  height: 100%;
  font-size: 13px;
  background-color: rgb(245, 247, 250);

  .item {
    display: flex;
    align-items: center;
    height: 45px;
    padding-right: 15px;
    padding-left: 15px;
    color: rgb(135, 135, 135);
    line-height: 45px;
    background-color: rgba(225, 237, 250, 0.5);
    border-right: 1px solid rgb(241, 241, 241);
    cursor: pointer;

    .label {
      min-width: 40px;
      max-width: 120px;
      height: 45px;
      overflow: hidden;
      text-align: center;
    }

    .iconColor {
      margin-right: 5px;
      color: #1989fa;
    }

    .right {
      width: 20px;
      padding-left: 5px;
      text-align: center;
    }

    .icon {
      display: none;
    }
  }

  .item:hover .icon {
    display: block;
  }
}
</style>
