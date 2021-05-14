export const permissions = [
  {
    name: '工作流',
    subPermissions: [
      {
        name: '查看工作流',
        uuid: '30005'
      },
      {
        name: '创建工作流',
        uuid: '30003'
      },
      {
        name: '编辑工作流',
        uuid: '30002'
      },
      {
        name: '复制工作流',
        uuid: '30003'
      },
      {
        name: '删除工作流',
        uuid: '30004'
      },
      {
        name: '执行工作流',
        uuid: '30001'
      },
      {
        name: '克隆任务',
        uuid: '30007'
      }
    ]
  },
  {
    name: '集成环境',
    subPermissions: [
      {
        name: '查看集成环境',
        uuid: '40004'
      },
      {
        name: '创建集成环境',
        uuid: '40001'
      },
      {
        name: '管理集成环境',
        uuid: '40003'
      },
      {
        name: '更新镜像/重启实例',
        uuid: '40010',
      },
      {
        name: '删除集成环境',
        uuid: '40002'
      }
    ]
  }
];
