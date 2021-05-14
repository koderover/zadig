<template>
  <div v-loading="loading"
       element-loading-text="加载中..."
       element-loading-spinner="iconfont iconfont-loading icongeren"
       class="users-overview-container">
    <!--start of add user dialog-->
    <el-dialog title="新建用户"
               custom-class="create-user-dialog"
               :close-on-click-modal="false"
               :visible.sync="dialogAddUserVisible">
      <el-form :model="addUser"
               @submit.native.prevent
               :rules="addUserRule"
               ref="addUserForm">
        <el-form-item label="登录邮箱"
                      prop="email">
          <el-input size="small"
                    v-model="addUser.email"></el-input>
        </el-form-item>
        <el-form-item label="用户名"
                      prop="name">
          <el-input size="small"
                    v-model="addUser.name"></el-input>
        </el-form-item>
        <el-form-item label="密码"
                      prop="password">
          <el-input size="small"
                    v-model="addUser.password"></el-input>
        </el-form-item>
      </el-form>
      <div slot="footer"
           class="dialog-footer">
        <el-button type="primary"
                   native-type="submit"
                   size="small"
                   @click="addUserOperation"
                   class="start-create">确定</el-button>
        <el-button plain
                   native-type="submit"
                   size="small"
                   @click="dialogAddUserVisible = false">取消</el-button>
      </div>
    </el-dialog>
    <!--end of add user dialog-->

    <div class="create-team">
      <el-row :gutter="10">
        <el-col :span="6">
          <div class="search-member">
            <span class="text-title">搜索成员:</span>
            <el-button v-if="!searchInputVisible"
                       size="small"
                       @click="searchInputVisible=true"
                       plain
                       type="primary"
                       icon="el-icon-search"></el-button>
            <transition name="fade">
              <el-input v-if="searchInputVisible"
                        size="small"
                        v-model.lazy="searchUser"
                        placeholder="请输入用户名"
                        autofocus
                        clearable
                        prefix-icon="el-icon-search">
              </el-input>
            </transition>
          </div>
        </el-col>
        <el-col :span="3">
          <el-button @click="dialogAddUserVisible=true"
                     size="small"
                     plain
                     type="primary">新建用户</el-button>
        </el-col>
      </el-row>

    </div>
    <div class="users-container">
      <el-table :data="usersTableData"
                style="width: 100%">
        <el-table-column label="用户名称">
          <template slot-scope="scope">
            {{scope.row.name}}
          </template>
        </el-table-column>
        <el-table-column label="邮件">
          <template slot-scope="scope">
            {{ scope.row.email}}
          </template>
        </el-table-column>
        <el-table-column prop="lastLogin"
                         label="登录信息">
          <template slot-scope="scope">
            <span v-if="scope.row.lastLogin">{{$utils.convertTimestamp(scope.row.lastLogin)}}</span>
            <span v-else>{{'尚未登录'}}</span>
          </template>
        </el-table-column>
        <el-table-column prop="directory"
                         label="来源">
          <template slot-scope="scope">
            <span>{{scope.row.directory}}</span>
          </template>
        </el-table-column>
        <el-table-column prop="leaders"
                         label="系统角色">
          <template slot-scope="scope">
            <el-tag type="info"
                    size="small">{{scope.row.isSuperUser?'管理员':'普通用户'}}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作"
                         width="280">
          <template slot-scope="scope">
            <el-button @click="deleteUser(scope.row)"
                       type="danger"
                       size="mini"
                       plain>删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <!--start of page-divide -->
      <div class="user-table-pagination">
        <el-pagination background
                       @size-change="handleSizeChange"
                       @current-change="handleCurrentChange"
                       :current-page="currentPageList"
                       :page-sizes="[10, 20, 30, 40]"
                       :page-size="userPageSize"
                       layout="total, sizes, prev, pager, next, jumper"
                       :total="totalUser">
        </el-pagination>
      </div>
      <!--page divide-->

    </div>

  </div>
</template>

<script>

import { addUserAPI, usersAPI, deleteUserAPI } from '@api';
import bus from '@utils/event_bus';
import _ from "lodash";

export default {
  data() {
    return {
      users: [],
      usersTableData: [],
      addUser: {
        "email": "",
        "password": "",
        "isSuperUser": true,
        "name": "",
        "phone": ""
      },
      editUser: {
      },
      searchUser: '',
      totalUser: 0,
      userPageSize: 10,
      currentPageList: 1,
      dialogEditRoleVisible: false,
      dialogAddUserVisible: false,
      searchInputVisible: true,
      loading: true,
      addUserRule: {
        name: [
          {
            type: 'string',
            required: true,
            message: '请输入用户名',
            trigger: 'blur'
          }
        ],
        email: [
          {
            type: 'string',
            required: true,
            message: '请输入登录邮箱',
            trigger: 'blur'
          },
          {
            type: 'email',
            message: '请输入正确的邮箱地址',
            trigger: ['blur', 'change']
          }
        ],
        password: [
          {
            type: 'string',
            required: true,
            message: '请输入密码',
            trigger: 'blur'
          }
        ],
        isSuperUser: [
          {
            type: 'boolean',
            required: true,
            message: '请选择角色',
            trigger: 'change'
          }
        ]
      }
    };
  },
  methods: {
    submit() {
      this.$refs["form"].validate((valid) => {
        if (valid) {
          this.upsertRole();
        }
      });
    },
    handleUserRoleUpdate() {
      const payload = {
        userId: this.editUser.id,
        isSuperUser: this.editUser.isSuperUser
      };
      editUserRoleAPI(payload).then((res) => {
        this.dialogEditRoleVisible = false;
        this.$message({
          type: 'success',
          message: '更改角色成功'
        });
        this.getUsers(this.searchId, this.userPageSize, this.currentPageList, this.searchUser);
      });
    },
    getUsers(team_id, page_size = 0, page_index = 0, keyword = '') {
      const orgId = this.currentOrganizationId;
      this.loading = true;
      usersAPI(orgId, team_id, page_size, page_index, keyword).then((res) => {
        this.loading = false;
        this.totalUser = Number(res.headers['x-total']);
        this.usersTableData = res.data;
      });
    },
    deleteUser(row) {
      this.$confirm(`确定删除系统用户 ${row.name}`, '提示', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        deleteUserAPI(row.id).then((res) => {
          this.$message({
            type: 'success',
            message: '删除用户成功'
          });
          this.getUsers(this.searchId, this.userPageSize, this.currentPageList, this.searchUser);
        });
      }).catch(() => {
        this.$message({
          type: 'info',
          message: '已取消删除'
        });
      });
    },
    addUserOperation() {
      this.$refs['addUserForm'].validate((valid) => {
        if (valid) {
          const payload = this.addUser;
          const orgId = this.currentOrganizationId;
          addUserAPI(orgId, payload).then((res) => {
            this.dialogAddUserVisible = false;
            this.$message({
              type: 'success',
              message: '新建用户成功'
            });
            this.getUsers(this.searchId, this.userPageSize, this.currentPageList, this.searchUser);
          });
        } else {
          return false;
        }
      });
    },
    handleSizeChange(val) {
      this.userPageSize = val;
      this.getUsers(this.searchId, this.userPageSize, this.currentPageList, this.searchUser);
    },
    handleCurrentChange(val) {
      this.currentPageList = val;
      this.getUsers(this.searchId, this.userPageSize, this.currentPageList, this.searchUser);
    },
  },
  computed: {
    currentOrganizationId() {
      return this.$store.state.login.userinfo.organization.id;
    },
    searchId() {
      return '';
    }
  },
  watch: {
    searchUser: function (val, oldVal) {
      this.getUsers('', this.userPageSize, this.currentPageList, val);
    }
  },
  created() {
    bus.$emit(`set-topbar-title`, { title: '用户管理', breadcrumb: [] });
    bus.$emit(`set-sub-sidebar-title`, {
      title: '',
      routerList: []
    });
    this.getUsers('', this.userPageSize, this.currentPageList, this.searchUser);
  }
};
</script>


<style lang="less">
.users-overview-container {
  flex: 1;
  position: relative;
  overflow: auto;
  padding: 15px 30px;
  font-size: 13px;
  .module-title h1 {
    font-weight: 200;
    font-size: 2rem;
    margin-bottom: 1.5rem;
  }
  .create-team {
    margin-top: 10px;
    margin-bottom: 15px;
    .text-title {
      margin-right: 15px;
      color: rgba(0, 0, 0, 0.65);
    }
    .team-select {
      .el-select {
        width: calc(~"100% - 60px");
      }
    }
    .search-member {
      .el-input {
        width: calc(~"100% - 80px");
      }
    }
  }
  .permission-form {
    .el-form-item__label {
      line-height: 28px;
    }
    .el-form-item {
      &:last-child {
        margin-bottom: 0;
      }
      .el-form-item__content {
        line-height: 28px;
      }
    }
    .permissions-group {
      &:last-child {
        margin-bottom: 0;
      }
      .sub-permissions {
        margin-left: 25px;

        .sub-permissions-checkbox {
          min-width: ~"calc(25% - 30px)";
          font-weight: normal;
        }
      }
    }
  }
  .users-container {
    .name-wrapper {
      font-size: 24px;
      line-height: 23px;
      .icon {
        cursor: pointer;
        color: #c0c4cc;
        margin-right: 5px;
        &:hover {
          color: #1989fa;
        }
      }
    }
    .user-table-pagination {
      margin-top: 25px;
    }
  }

  .el-table th > .cell {
    color: #97a8be;
  }
  .el-input {
    display: inline-block;
  }
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.6s;
}
.fade-enter,
.fade-leave-to {
  opacity: 0;
}

.create-user-dialog {
  width: 450px;
  .el-dialog__header {
    text-align: center;
    border-bottom: 1px solid #e4e4e4;
    padding: 15px;
    .el-dialog__close {
      font-size: 10px;
    }
  }
  .el-dialog__body {
    padding-bottom: 0px;
  }

  .el-select {
    width: 100%;
  }
  .el-input {
    display: inline-block;
  }
}
</style>
