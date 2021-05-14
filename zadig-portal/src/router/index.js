import Vue from 'vue';
import VueRouter from 'vue-router';

// sign
import signin from '../components/entry/login.vue';

// 404
import not_found from '../components/entry/404.vue';

// setup
import setup_admin from '../components/setup/admin.vue';

// home
import onboarding_home from '../components/entry/home.vue';

// status
import workflow_status from '../components/projects/status/status.vue';

// project
import onboarding_projects_home from '../components/projects/detail_ope/home.vue';
import onboarding_projects_create from '../components/projects/detail_ope/create.vue';
import onboarding_projects_detail from '../components/projects/detail_ope/detail.vue';

// services

// service mgr
import project_service from '../components/projects/service_mgr/service.vue';

// workflow
import onboarding_projects_pipelines from '../components/projects/pipeline/workflow_list.vue';
import workflow_multi_detail from '../components/projects/pipeline/workflow_multi_detail.vue';
import workflow_multi_task from '../components/projects/pipeline/workflow_multi_task_detail.vue';

// build
import project_build_config from '../components/projects/build/config.vue';
import project_build_config_detail from '../components/projects/build/config_detail.vue';

// workflow product
import configure_pipeline_view from '../components/projects/edit_pipeline/view.vue';
import product_pipeline from '../components/projects/edit_pipeline/product_pipeline/pipeline.vue';

// env
// outside
import env_home from '../components/projects/env/outer_env/home.vue';
import env_detail from '../components/projects/env/outer_env/env_detail.vue';

// inside
import project_env_home from '../components/projects/env/inner_env/home.vue';
import project_env_detail from '../components/projects/env/inner_env/env_detail.vue';
import project_env_create from '../components/projects/env/inner_env/create_env_detail.vue';
import project_env_service_detail from '../components/projects/env/inner_env/service_detail.vue';
import project_env_service_config from '../components/projects/env/inner_env/service_config.vue';

// delivery center
import delivery_artifacts from '../components/projects/delivery/artifacts/index.vue';
import delivery_artifacts_detail from '../components/projects/delivery/artifacts/detail.vue';

/**    onboarding    */
// k8s
import onboarding_projects_info_basic from '../components/projects/guide/k8s/info_basic.vue';
import onboarding_projects_service from '../components/projects/guide/service.vue';
import onboarding_projects_runtime from '../components/projects/guide/runtime.vue';
import onboarding_projects_delivery from '../components/projects/guide/k8s/delivery.vue';

/** ---------------- */

/**   enterprise    */

import enterprise_mgr_home from '../components/enterprise_mgr/home.vue';
import enterprise_mgr_users_manage from '../components/enterprise_mgr/users/manage.vue';

/** ---------------- */

/**   system setting      */
import setting_home from '../components/setting/home.vue';

import setting_integration_manage_home from '../components/setting/integration/home.vue';
import setting_apps_manage from '../components/setting/apps/manage.vue';
import setting_registry_manage from '../components/setting/registry/manage.vue';
import setting_storage_manage from '../components/setting/storage/manage.vue';
import setting_config_manage_home from '../components/setting/config/home.vue';
import setting_config_manage_proxy from '../components/setting/config/proxy.vue';
import setting_config_manage_cache from '../components/setting/config/cache.vue';
/** ---------------- */

/**     profile      */

import profile_home from '../components/profile/home.vue';
import profile_manage from '../components/profile/manage.vue';

/** ---------------- */

//mobile
import mobile from '../mobile/index.vue';
import mobile_status from '../mobile/pipelines/status.vue';
import mobile_envs from '../mobile/envs/envs.vue';
import mobile_env_detail from '../mobile/envs/env_detail';
import mobile_env_service_detail from '../mobile/envs/service_detail';
import mobile_pipelines from '../mobile/pipelines/pipelines.vue';
import mobile_pipelines_detail from '../mobile/pipelines/detail.vue';
import mobile_pipelines_multi_task from '../mobile/pipelines/multi_task.vue';
import mobile_profile from '../mobile/profile.vue';

Vue.use(VueRouter);
const routes = [
  {
    path: '/v1',
    component: onboarding_home,
    meta: {},
    children: [
      {
        path: 'status',
        component: workflow_status,
        meta: {
          requiresAuth: true,
          title: '运行状态',
        },
      },
      {
        path: 'projects',
        component: onboarding_projects_home,
        meta: {
          requiresAuth: true,
          title: '项目列表',
        },
      },
      {
        path: 'envs',
        component: env_home,
        meta: {
          requiresAuth: true,
          title: '集成环境',
        },
        children: [
          {
            path: 'detail/:project_name',
            component: env_detail,
            meta: {
              requiresAuth: true,
              title: '集成环境',
            },
          },
          {
            path: 'detail/:project_name/:service_name',
            component: project_env_service_detail,
            meta: {
              requiresAuth: true,
              title: '服务详情',
            },
          },
          {
            path: 'detail/:project_name/:service_name/config',
            component: project_env_service_config,
            meta: {
              requiresAuth: true,
              title: '服务详情',
            },
          },
        ],
      },
      {
        path: 'pipelines',
        component: onboarding_projects_pipelines,
        meta: {
          requiresAuth: true,
          title: '工作流',
        },
      },
      {
        path: 'projects/create',
        component: onboarding_projects_create,
        meta: {
          requiresAuth: true,
          title: '新建项目',
        },
      },
      {
        path: 'projects/create/:project_name/basic/info',
        component: onboarding_projects_info_basic,
        meta: {
          requiresAuth: true,
          title: '新建项目',
        },
      },
      {
        path: 'projects/create/:project_name/basic/service',
        component: onboarding_projects_service,
        meta: {
          requiresAuth: true,
          title: '新建项目',
        },
      },
      {
        path: 'projects/create/:project_name/basic/runtime',
        component: onboarding_projects_runtime,
        meta: {
          requiresAuth: true,
          title: '新建项目',
        },
      },
      {
        path: 'projects/create/:project_name/basic/delivery',
        component: onboarding_projects_delivery,
        meta: {
          requiresAuth: true,
          title: '新建项目',
        },
      },
      {
        path: 'projects/edit/:project_name',
        component: onboarding_projects_create,
        meta: {
          requiresAuth: true,
          title: '修改项目',
        },
      },
      {
        path: 'projects/detail/:project_name',
        component: onboarding_projects_detail,
        meta: {
          requiresAuth: true,
          title: '项目详情',
        },
      },
      {
        path: 'projects/detail/:project_name/pipelines',
        component: onboarding_projects_pipelines,
        meta: {
          requiresAuth: true,
          title: '工作流',
        },
      },
      {
        path: 'projects/detail/:project_name/services',
        component: project_service,
        meta: {
          requiresAuth: true,
          title: '服务管理',
        },
      },
      {
        path: 'projects/detail/:project_name/builds',
        component: project_build_config,
        meta: {
          requiresAuth: true,
          title: '构建',
        },
      },
      {
        path: 'projects/detail/:project_name/builds/create',
        component: project_build_config_detail,
        meta: {
          requiresAuth: true,
          title: '新建构建',
        },
      },
      {
        path: 'projects/detail/:project_name/builds/detail/:build_name/:version',
        component: project_build_config_detail,
        meta: {
          requiresAuth: true,
          title: '构建详情',
        },
      },
      {
        path: 'projects/detail/:project_name/envs',
        component: project_env_home,
        meta: {
          requiresAuth: true,
          title: '集成环境',
        },
        children: [
          {
            path: 'create',
            component: project_env_create,
            meta: {
              requiresAuth: true,
              title: '创建环境',
            },
          },
          {
            path: 'detail',
            component: project_env_detail,
            meta: {
              requiresAuth: true,
              title: '集成环境',
            },
          },
          {
            path: 'detail/:service_name',
            component: project_env_service_detail,
            meta: {
              requiresAuth: true,
              title: '服务详情',
            },
          },
          {
            path: 'detail/:service_name/config',
            component: project_env_service_config,
            meta: {
              requiresAuth: true,
              title: '配置详情',
            },
          },
        ],
      },
      {
        path: 'projects/detail/:project_name/pipelines/multi/:workflow_name',
        component: workflow_multi_detail,
        meta: {
          requiresAuth: true,
          title: '工作流详情',
        },
      },
      {
        path: 'projects/detail/:project_name/pipelines/multi/:workflow_name/:task_id',
        component: workflow_multi_task,
        meta: {
          requiresAuth: true,
          title: '任务详情',
        },
      },
    ],
  },
  {
    path: '/setup',
    component: setup_admin,
    meta: {
      title: '系统初始化',
    },
  },
  {
    path: '/v1/delivery',
    component: onboarding_home,
    meta: {
      requiresAuth: true,
      requiresSuperAdmin: false,
      title: '交付中心',
    },
    children: [
      {
        path: 'artifacts',
        component: delivery_artifacts,
        meta: {
          requiresAuth: true,
          requiresSuperAdmin: false,
          title: '交付物追踪',
        },
      },
      {
        path: 'artifacts/detail/:id',
        component: delivery_artifacts_detail,
        meta: {
          requiresAuth: true,
          requiresSuperAdmin: false,
          title: '交付物追踪',
        },
      },
    ],
  },
  {
    path: '/productpipelines',
    component: configure_pipeline_view,
    meta: {
      requiresAuth: true,
      title: '工作流管理',
    },
    children: [
      {
        path: 'create',
        component: product_pipeline,
        meta: {
          requiresAuth: true,
          title: '新建工作流',
        },
      },
      {
        path: 'edit/:name',
        component: product_pipeline,
        meta: {
          requiresAuth: true,
          title: '编辑工作流',
        },
      },
    ],
  },
  {
    path: '/v1/enterprise',
    component: onboarding_home,
    meta: {
      requiresAuth: true,
      requiresSuperAdmin: true,
      title: '用户管理',
    },
    children: [
      {
        path: '',
        component: enterprise_mgr_home,
        meta: {
          requiresAuth: true,
          requiresSuperAdmin: true,
          title: '用户管理',
        },
      },
      {
        path: 'users/manage',
        component: enterprise_mgr_users_manage,
        meta: {
          requiresAuth: true,
          requiresSuperAdmin: true,
          title: '用户管理',
        },
      },
    ],
  },
  {
    path: '/v1/profile',
    component: onboarding_home,
    meta: {
      requiresAuth: true,
    },
    children: [
      {
        path: '',
        component: profile_home,
        meta: {
          requiresAuth: true,
          title: '用户设置',
        },
      },
      {
        path: 'info',
        component: profile_manage,
        meta: {
          requiresAuth: true,
          title: '用户设置',
        },
      },
    ],
  },
  {
    path: '/v1/system',
    component: onboarding_home,
    meta: {
      requiresAuth: true,
      requiresSuperAdmin: false,
    },
    children: [
      {
        path: '',
        component: setting_home,
        meta: {
          requiresAuth: true,
          requiresSuperAdmin: true,
          title: '系统设置',
        },
      },
      {
        path: 'apps',
        component: setting_apps_manage,
        meta: {
          requiresAuth: true,
          requiresSuperAdmin: true,
          title: '应用设置',
        },
      },
      {
        path: 'registry',
        component: setting_registry_manage,
        meta: {
          requiresAuth: true,
          requiresSuperAdmin: true,
          title: 'Registry 设置',
        },
      },
      {
        path: 'storage',
        component: setting_storage_manage,
        meta: {
          requiresAuth: true,
          requiresSuperAdmin: true,
          title: '对象存储',
        },
      },
      {
        path: 'integration',
        component: setting_integration_manage_home,
        meta: {
          requiresAuth: true,
          title: '集成管理',
        },
      },
      {
        path: 'config',
        component: setting_config_manage_home,
        meta: {
          requiresAuth: true,
          requiresSuperAdmin: true,
          title: '系统配置',
        },
        children: [
          {
            path: 'manage/proxy',
            component: setting_config_manage_proxy,
            meta: {
              requiresAuth: true,
              requiresSuperAdmin: true,
              title: '代理配置',
            },
          },
          {
            path: 'manage/cache',
            component: setting_config_manage_cache,
            meta: {
              requiresAuth: true,
              requiresSuperAdmin: true,
              title: '缓存清理',
            },
          },
        ],
      },
    ],
  },
  {
    path: '/mobile',
    component: mobile,
    meta: {
      title: 'Zadig',
    },
    children: [
      {
        path: 'status',
        component: mobile_status,
        meta: {
          requiresAuth: true,
          title: '运行状态',
        },
      },
      {
        path: 'envs',
        component: mobile_envs,
        meta: {
          requiresAuth: true,
          title: '集成环境',
        },
      },
      {
        path: 'envs/detail/:project_name',
        component: mobile_env_detail,
        meta: {
          requiresAuth: true,
          title: '集成环境',
        },
      },
      {
        path: 'envs/detail/:project_name/:service_name',
        component: mobile_env_service_detail,
        meta: {
          requiresAuth: true,
          title: '服务详情',
        },
      },
      {
        path: 'pipelines',
        component: mobile_pipelines,
        meta: {
          requiresAuth: true,
          title: '工作流',
        },
      },
      {
        path: 'pipelines/project/:project_name/multi/:workflow_name/:task_id',
        component: mobile_pipelines_multi_task,
        meta: {
          requiresAuth: true,
          title: '任务详情',
        },
      },
      {
        path: 'pipelines/project/:project_name/multi/:workflow_name',
        component: mobile_pipelines_detail,
        meta: {
          requiresAuth: true,
          title: '工作流详情',
        },
      },
      {
        path: 'profile',
        component: mobile_profile,
        meta: {
          requiresAuth: true,
          title: '用户设置',
        },
      },
    ],
  },
  {
    path: '/signin',
    component: signin,
    meta: {
      title: '登录',
    },
  },
  {
    path: '/404',
    component: not_found,
    meta: {
      title: '404 Not Found',
    },
  },
  {
    path: '/',
    component: signin,
    meta: {
      title: '登录',
    },
  },
  {
    path: '*',
    component: not_found,
    meta: {
      title: '404 Not Found',
    },
  },
];

export default new VueRouter({
  mode: 'history',
  base: __dirname,
  routes: routes,
});
