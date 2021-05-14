import axios from 'axios';
import qs from 'qs';
import Element from 'element-ui';
import moment from 'moment';
const specialAPIs = ['/api/directory/userss/search', '/api/aslan/delivery/artifacts'];
const ignoreErrReq = '/api/aslan/services/validateUpdate/';
const reqExps = /api\/aslan\/environment\/environments\/[a-z-0-9]+\/groups$/;
const analyticsReq = 'https://api.koderover.com/api/operation/upload';
const installationAnalysisReq = 'https://api.koderover.com/api/operation/admin/user';
const http = axios.create();
const CancelToken = axios.CancelToken;

let source = null;
export function initSource() {
  source = CancelToken.source();
  return source;
}
export function rmSource() {
  source = null;
  return;
}

class TokenSource {
  constructor({ cancelInfo }) {
    this.source = null;
    this.cancelInfo = cancelInfo || 'cancel';
  }
  get sourceToken() {
    return this.source.token;
  }
  initSource() {
    this.source = CancelToken.source();
  }
  rmSource() {
    this.source = null;
  }
  cancelRequest() {
    this.source.cancel(this.cancelInfo);
  }
}

var analyticsReqSource = new TokenSource({ cancelInfo: 'cancel analytics request' });
analyticsReqSource.initSource();

http.interceptors.request.use((config) => {
  if (source && config.method === 'get') {
    config.cancelToken = source.token;
  }
  if (config.url === analyticsReq) {
    analyticsReqSource.cancelRequest();
    analyticsReqSource.initSource();
    config.cancelToken = analyticsReqSource.sourceToken;
  }
  return config;
});

http.interceptors.response.use(
  (response) => {
    const req = response.config.url.split(/[?#]/)[0];
    if (specialAPIs.includes(req)) {
      return response;
    } else if (reqExps.test(req)) {
      return response;
    } else {
      return response.data;
    }
  },
  (error) => {
    if (axios.isCancel(error)) {
      if (error.message === analyticsReqSource.cancelInfo) {
        return Promise.reject(analyticsReqSource.cancelInfo);
      }
      return Promise.reject('CANCEL');
    }
    if (
      error.response &&
      error.response.config.url !== analyticsReq &&
      error.response.config.url !== installationAnalysisReq &&
      !error.response.config.url.includes(ignoreErrReq)
    ) {
      // The request was made and the server responded with a status code
      // that falls out of the range of 2xx
      console.log(error.response);
      if (document.title !== '登录' && document.title !== '系统初始化') {
        if (error.response.status === 401) {
          const redirectPath = window.location.pathname + window.location.search;
          Element.Message.error('登录信息失效, 请返回重新登录');
          if (redirectPath.includes('/setup')) {
            window.location.href = `/signin`;
          } else {
            window.location.href = `/signin?redirect=${redirectPath}`;
          }
        }
        if (error.response.status === 403) {
          Element.Message.error('暂无权限');
        } else {
          if (error.response.data.code !== 6168) {
            let msg = `${error.response.status} API 请求错误`;
            if (error.response.data.errorMsg) {
              msg = `${error.response.status} : ${error.response.data.errorMsg}`;
            } else if (error.response.data.message) {
              msg = `${error.response.status} : ${error.response.data.message} ${error.response.data.description}`;
            }
            Element.Message({
              message: msg,
              type: 'error',
              dangerouslyUseHTMLString: false,
            });
          }
        }
      } else if (document.title === '登录' || document.title === '系统初始化') {
        let msg = `${error.response.status} API 请求错误`;
        if (error.response.data.errorMsg) {
          msg = `${error.response.status} : ${error.response.data.errorMsg}`;
        } else if (error.response.data.message) {
          msg = `${error.response.status} : ${error.response.data.message} ${error.response.data.description}`;
        }
        if (error.response.config.url !== '/api/directory/user/detail') {
          Element.Message({
            message: msg,
            type: 'error',
            dangerouslyUseHTMLString: false,
          });
        }
      }
    } else if (error.request) {
      // The request was made but no response was received
      // `error.request` is an instance of XMLHttpRequest in the browser and an instance of
      // http.ClientRequest in node.js
      Element.Message.error(error.request);
    } else {
      // Something happened in setting up the request that triggered an Error
      Element.Message.error(error.message);
      console.log('Error', error.message);
    }
    return Promise.reject(error);
  }
);

window.__spockEventSources = {};
function makeEventSource(basePath, config) {
  let path = basePath;
  const params = config && config.params;
  if (typeof params === 'object') {
    path = basePath + '?' + qs.stringify(params);
  }
  if (typeof params === 'string') {
    path = basePath + '?' + params;
  }

  const evtSource = new EventSource(path);
  evtSource.userCount = 0;

  const normalHandlers = [];
  const errorHandlers = [];
  evtSource.onmessage = (e) => {
    normalHandlers.forEach((h) => h({ data: JSON.parse(e.data) }));
  };
  evtSource.onerror = (e) => {
    console.log(e);
  };

  const ret = {
    then(fun1, fun2) {
      normalHandlers.push(fun1);
      if (typeof fun2 === 'function') {
        errorHandlers.push(fun2);
      }
      return ret;
    },
    catch(fun) {
      errorHandlers.push(fun);
      return ret;
    },
    close() {
      evtSource.close();
      return ret;
    },
    closeWhenDestroy(vm) {
      evtSource.userCount++;
      if (!window.__spockEventSources[vm._uid]) {
        window.__spockEventSources[vm._uid] = [];
      }
      window.__spockEventSources[vm._uid].push(evtSource);
      return ret;
    },
    isAlive() {
      // 0 — connecting
      // 1 — open
      // 2 — closed
      return evtSource.readyState !== 2;
    },
  };

  return ret;
}

// Analytics
export function analyticsRequestAPI(payload) {
  return http.post(analyticsReq, payload);
}
export function installationAnalysisRequestAPI(payload) {
  return http.post(installationAnalysisReq, payload);
}


// Status
export function taskRunningSSEAPI() {
  return makeEventSource('/api/aslan/workflow/sse/tasks/running');
}

export function taskPendingSSEAPI() {
  return makeEventSource('/api/aslan/workflow/sse/tasks/pending');
}


// Env
export function listProductAPI(envType = '', productName = '') {
  if (envType) {
    return http.get(`/api/aslan/environment/environments?envType=${envType}`);
  } else {
    return http.get(`/api/aslan/environment/environments?productName=${productName}`);
  }
}

export function listProductSSEAPI(params) {
  return makeEventSource('/api/aslan/environment/sse/products?envType=', {
    params,
  });
}

export function getProductsAPI() {
  return http.get('/api/aslan/environment/environments');
}
export function getDeployEnvByPipelineNameAPI(pipelineName) {
  return http.get(`/api/aslan/workflow/v2/tasks/pipelines/${pipelineName}/products`);
}

export function getServicePipelineAPI(projectName, envName, serviceName, serviceType) {
  return http.get(`/api/aslan/workflow/servicetask/workflows/${projectName}/${envName}/${serviceName}/${serviceType}`);
}

export function envRevisionsAPI(projectName, envName) {
  return http.get(`/api/aslan/environment/revision/products?productName=${projectName}&envName=${envName}`);
}

export function productServicesAPI(projectName, envName, searchName = '', perPage = 20, page = 1) {
  return http.get(`/api/aslan/environment/environments/${projectName}/groups?envName=${envName}&serviceName=${searchName}&perPage=${perPage}&page=${page}`);
}

export function fetchGroupsDataAPI(name, envName) {
  return http.get(`/api/aslan/environment/environments/${name}/groups?envName=${envName}`);
}

export function productEnvInfoAPI(projectName, envName) {
  return http.get(`/api/aslan/environment/environments/${projectName}?envName=${envName}`);
}


// Project
export function productTemplatesAPI() {
  return http.get('/api/aslan/project/products');
}


// Service
export function templatesAPI() {
  return http.get('/api/aslan/project/templates/info');
}

export function getServiceTemplatesAPI(projectName = '') {
  return http.get(`/api/aslan/service/services?productName=${projectName}`);
}

export function getServiceTemplatesStatAPI() {
  return http.get(`/api/aslan/service/services?requestFrom=stat`);
}

export function serviceTemplateWithConfigAPI(name, projectName) {
  return http.get(`/api/aslan/service/services/${name}?productName=${projectName}`);
}

export function serviceTemplateAPI(name, type, projectName) {
  return http.get(`/api/aslan/service/services/${name}/${type}?productName=${projectName}`);
}

export function serviceTemplateAfterRenderAPI(product_name, service_name, env_name) {
  return http.get(`/api/aslan/environment/diff/products/${product_name}/service/${service_name}?envName=${env_name}`);
}

export function saveServiceTemplateAPI(payload) {
  return http.post('/api/aslan/service/services', payload);
}

export function updateServicePermissionAPI(data) {
  return http.put('/api/aslan/service/services', data);
}

export function deleteServiceTemplateAPI(name, type, projectName, visibility) {
  return http.delete(`/api/aslan/service/services/${name}/${type}?productName=${projectName}&visibility=${visibility}`);
}

export function imagesAPI(payload, registry = '') {
  return http.post(`/api/aslan/system/registry/images?registryId=${registry}`, { names: payload });
}

export function initProductAPI(templateName, isStcov) {
  return http.get(`/api/aslan/environment/init_info/${templateName}${isStcov ? '?stcov=true' : ''}`);
}


// Build 
export function getImgListAPI(from = '') {
  return http.get(`/api/aslan/system/basicImages?image_from=${from}`);
}

export function deleteBuildConfigAPI(name, version, projectName) {
  return http.delete(`/api/aslan/build/build?name=${name}&version=${version}&productName=${projectName}`);
}

export function createBuildConfigAPI(payload) {
  return http.post(`/api/aslan/build/build`, payload);
}

export function updateBuildConfigAPI(payload) {
  return http.put(`/api/aslan/build/build`, payload);
}

export function saveBuildConfigTargetsAPI(projectName = '', payload) {
  return http.post(`/api/aslan/build/build/targets?productName=${projectName}`, payload);
}

export function getBuildConfigDetailAPI(name, version, projectName = '') {
  return http.get(`/api/aslan/build/build/${name}/${version}?productName=${projectName}`);
}

export function getRepoFilesAPI(codehostId, repoOwner, repoName, branchName, path, type, remoteName = 'origin') {
  const encodeRepoName = repoName.includes('/') ? encodeURIComponent(encodeURIComponent(repoName)) : repoName;
  if (type === 'github') {
    return http.get(`/api/aslan/code/workspace/github/${codehostId}/${encodeRepoName}/${branchName}?path=${path}`);
  } else if (type === 'gitlab') {
    return http.get(`/api/aslan/code/workspace/gitlab/${codehostId}/${encodeRepoName}/${branchName}?path=${path}&repoOwner=${repoOwner}`);
  } else if (type === 'gerrit') {
    return http.get(`/api/aslan/code/workspace/git/${codehostId}/${encodeRepoName}/${branchName}/${remoteName}?repoOwner=${repoOwner}&dir=${path}`);
  }
}

export function getPublicRepoFilesAPI(path, url) {
  return http.get(`/api/aslan/code/workspace/publicRepo?dir=${path}&url=${url}`);
}

export function getRepoFileServiceAPI(codehostId, repoOwner, repoName, branchName, path, isDir, remoteName = '') {
  const encodeRepoName = repoName.includes('/') ? encodeURIComponent(encodeURIComponent(repoName)) : repoName;
  return http.get(
    `/api/aslan/service/loader/preload/${codehostId}/${branchName}?repoOwner=${repoOwner}&repoName=${encodeRepoName}&path=${path}&isDir=${isDir}&remoteName=${remoteName}`
  );
}

export function loadRepoServiceAPI(codehostId, repoOwner, repoName, branchName, remoteName = '', payload) {
  const encodeRepoName = repoName.includes('/') ? encodeURIComponent(encodeURIComponent(repoName)) : repoName;
  return http.post(`/api/aslan/service/loader/load/${codehostId}/${branchName}?repoOwner=${repoOwner}&repoName=${encodeRepoName}&remoteName=${remoteName}`, payload);
}

export function validPreloadService(codehostId, repoOwner, repoName, branchName, path, serviceName, isDir = false, remoteName = '') {
  const encodeRepoName = repoName.includes('/') ? encodeURIComponent(encodeURIComponent(repoName)) : repoName;
  return http.get(
    `/api/aslan/service/loader/validateUpdate/${codehostId}/${branchName}?repoOwner=${repoOwner}&repoName=${encodeRepoName}&path=${path}&serviceName=${serviceName}&isDir=${isDir}&remoteName=${remoteName}`
  );
}

export function getServiceTargetsAPI(projectName = '') {
  return http.get(`/api/aslan/build/targets?productName=${projectName}`);
}

export function getTargetBuildConfigsAPI(target, project_name) {
  return http.get(`/api/aslan/build/build?targets=${target}&productName=${project_name}`);
}

export function getBuildConfigsAPI(projectName = '') {
  return http.get(`/api/aslan/build/build?productName=${projectName}`);
}


//Workflow
export function getWorkflowHistoryBuildLogAPI(workflowName, taskID, serviceName, type) {
  return http.get(`/api/aslan/logs/log/workflow/${workflowName}/tasks/${taskID}/service/${serviceName}?type=${type}`);
}

export function getWorkflowHistoryTestLogAPI(workflowName, taskID, testName, serviceName, workflowType = '') {
  return http.get(`/api/aslan/logs/log/workflow/${workflowName}/tasks/${taskID}/tests/${testName}/service/${serviceName}?workflowType=${workflowType}`); //
}

export function imageReposAPI() {
  return http.get(`/api/aslan/system/registry/release/repos`);
}

export function getArtifactWorkspaceAPI(workflowName, taskId, dir = '') {
  return http.get(`/api/aslan/workspace/workflow/${workflowName}/taskId/${taskId}?dir=${dir}`);
}

export function downloadArtifactAPI(workflowName, taskId) {
  return http.get(`/api/aslan/v2/tasks/workflow/${workflowName}/taskId/${taskId}`);
}

export function getAllBranchInfoAPI(data, param = '') {
  return http.put(`/api/aslan/code/codehost/infos?param=${param}`, data);
}

export function getWorkflowBindAPI(testName) {
  return http.get(`/api/aslan/workflow/workflow/testName/${testName}`);
}

export function listWorkflowAPI() {
  return http.get(`/api/aslan/workflow/workflow`);
}

export function setFavoriteAPI(payload) {
  return http.post(`/api/aslan/workflow/favorite`, payload);
}

export function deleteFavoriteAPI(productName, workflowName, type) {
  return http.delete(`/api/aslan/workflow/favorite/${productName}/${workflowName}/${type}`);
}
export function workflowAPI(name) {
  return http.get(`/api/aslan/workflow/workflow/find/${name}`);
}

export function workflowPresetAPI(productName) {
  return http.get(`/api/aslan/workflow/workflow/preset/${productName}`);
}

export function createWorkflowAPI(data) {
  return http.post(`/api/aslan/workflow/workflow`, data);
}

export function updateWorkflowAPI(data) {
  return http.put(`/api/aslan/workflow/workflow`, data);
}

export function deleteWorkflowAPI(name) {
  return http.delete(`/api/aslan/workflow/workflow/${name}`);
}

export function copyWorkflowAPI(oldName, newName) {
  return http.put(`/api/aslan/workflow/workflow/old/${oldName}/new/${newName}`);
}

export function precreateWorkflowTaskAPI(workflowName, envName) {
  return http.get(`/api/aslan/workflow/workflowtask/preset/${envName}/${workflowName}`);
}
export function createWorkflowTaskAPI(productName, envName) {
  return http.get(`/api/aslan/workflow/workflowtask/targets/${productName}/${envName}`);
}

export function getRegistryWhenBuildAPI() {
  return http.get(`/api/aslan/system/registry`);
}

export function getBuildTargetsAPI(productName) {
  return http.get(`/api/aslan/build/targets/${productName}`);
}

export function runWorkflowAPI(data, isArtifact = false) {
  if (isArtifact) {
    return http.put('/api/aslan/workflow/workflowtask', data);
  } else {
    return http.post('/api/aslan/workflow/workflowtask', data);
  }
}

export function restartWorkflowAPI(workflowName, taskID) {
  return http.post(`/api/aslan/workflow/workflowtask/id/${taskID}/pipelines/${workflowName}/restart`);
}

export function cancelWorkflowAPI(workflowName, taskID) {
  return http.delete(`/api/aslan/workflow/workflowtask/id/${taskID}/pipelines/${workflowName}`);
}

export function workflowTaskListAPI(name, start, max, workflowType = '') {
  return http.get(`/api/aslan/workflow/workflowtask/max/${max}/start/${start}/pipelines/${name}?workflowType=${workflowType}`);
}

export function workflowTaskDetailAPI(workflowName, taskID, workflowType = '') {
  return http.get(`/api/aslan/workflow/workflowtask/id/${taskID}/pipelines/${workflowName}?workflowType=${workflowType}`);
}

export function workflowTaskDetailSSEAPI(workflowName, taskID, workflowType = '') {
  return makeEventSource(`/api/aslan/workflow/sse/workflows/id/${taskID}/pipelines/${workflowName}?workflowType=${workflowType}`);
}


// Install
export function checkOrganizationHasSignupAPI() {
  return http.get(`/api/directory/check`);
}

export function createOrganizationInfoAPI(payload) {
  return http.post(`/api/directory/organization`, payload);
}

export function organizationInfoAPI(organization_id) {
  return http.get(`/api/directory/organization/${organization_id}`);
}

export function usersAPI(organization_id, team_id = '', page_size = 0, page_index = 0, keyword = '') {
  return http.get(`/api/directory/userss/search?orgId=${organization_id}&teamId=${team_id}&per_page=${page_size}&page=${page_index}&keyword=${keyword}`);
}


// User Management
export function addUserAPI(organization_id, payload) {
  return http.post(`/api/directory/user?orgId=${organization_id}`, payload);
}

export function editUserRoleAPI(payload) {
  return http.put(`/api/directory/roleUser`, payload);
}


export function deleteUserAPI(user_id) {
  return http.delete(`/api/directory/user/${user_id}`);
}


// ----- Syetem Setting-Integration -----

// Code
export function getCodeSourceAPI(organization_id = 1, page_size = 0, page_index = 0) {
  return http.get(`/api/aslan/code/codehost?orgId=${organization_id}`);
}

export function getCodeSourceByAdminAPI(organization_id, page_size = 0, page_index = 0) {
  return http.get(`/api/directory/codehostss/search?orgId=${organization_id}&per_page=${page_size}&page=${page_index}`);
}

export function createCodeSourceAPI(organization_id, payload) {
  return http.post(`/api/directory/codehosts?orgId=${organization_id}`, payload);
}

export function deleteCodeSourceAPI(code_source_id) {
  return http.delete(`/api/directory/codehosts/${code_source_id}`);
}

export function updateCodeSourceAPI(code_source_id, payload) {
  return http.put(`/api/directory/codehosts/${code_source_id}`, payload);
}

export function getRepoOwnerByIdAPI(id, key = '') {
  return http.get(`/api/aslan/code/codehost/${id}/namespaces?key=${key}`);
}

export function getRepoNameByIdAPI(id, type, repo_owner, key = '') {
  const repoOwner = repo_owner.includes('/') ? encodeURIComponent(encodeURIComponent(repo_owner)) : repo_owner;
  return http.get(`/api/aslan/code/codehost/${id}/namespaces/${repoOwner}/projects?type=${type}&key=${key}`);
}

export function getBranchInfoByIdAPI(id, repo_owner, repo_name) {
  const repoOwner = repo_owner.includes('/') ? encodeURIComponent(encodeURIComponent(repo_owner)) : repo_owner;
  const repoName = repo_name.includes('/') ? encodeURIComponent(encodeURIComponent(repo_name)) : repo_name;
  return http.get(`/api/aslan/code/codehost/${id}/namespaces/${repoOwner}/projects/${repoName}/branches`);
}


// GitHub App
export function getGithubAppAPI(organization_id, payload) {
  return http.get(`/api/aslan/system/githubApp?orgId=${organization_id}`, payload);
}

export function createGithubAppAPI(organization_id, payload) {
  return http.post(`/api/aslan/system/githubApp?orgId=${organization_id}`, payload);
}

export function updateGithubAppAPI(organization_id, payload) {
  return http.post(`/api/aslan/system/githubApp?orgId=${organization_id}`, payload);
}

export function deleteGithubAppAPI(id) {
  return http.delete(`/api/aslan/system/githubApp/${id}`);
}


// Jenkins
export function addJenkins(payload) {
  return http.post(`/api/aslan/system/jenkins/integration`, payload);
}
export function editJenkins(payload) {
  return http.put(`/api/aslan/system/jenkins/integration/${payload.id}`, payload);
}
export function deleteJenkins(payload) {
  return http.delete(`/api/aslan/system/jenkins/integration/${payload.id}`, payload);
}
export function queryJenkins() {
  return http.get('/api/aslan/system/jenkins/integration');
}
export function jenkinsConnection(payload) {
  return http.post(`/api/aslan/system/jenkins/user/connection`, payload);
}

export function queryJenkinsJob() {
  return http.get('/api/aslan/system/jenkins/jobNames');
}

export function queryJenkinsParams(jobName) {
  return http.get(`/api/aslan/system/jenkins/buildArgs/${jobName}`);
}


// Mail
export function getEmailHostAPI(organization_id) {
  return http.get(`/api/directory/emailHosts?orgId=${organization_id}`);
}

export function deleteEmailHostAPI(organization_id) {
  return http.delete(`/api/directory/emailHosts?orgId=${organization_id}`);
}

export function createEmailHostAPI(organization_id, payload) {
  return http.post(`/api/directory/emailHosts?orgId=${organization_id}`, payload);
}

export function getEmailServiceAPI(organization_id) {
  return http.get(`/api/directory/emailServices?orgId=${organization_id}`);
}

export function deleteEmailServiceAPI(organization_id) {
  return http.delete(`/api/directory/emailServices?orgId=${organization_id}`);
}

export function createEmailServiceAPI(organization_id, payload) {
  return http.post(`/api/directory/emailServices?orgId=${organization_id}`, payload);
}

// ----- Syetem Setting-Application -----

export function getAllAppsAPI() {
  return http.get(`/api/aslan/system/install?available=true`);
}
export function createAppAPI(data) {
  return http.post(`/api/aslan/system/install`, data);
}
export function updateAppAPI(data) {
  return http.put(`/api/aslan/system/install`, data);
}
export function deleteAppAPI(data) {
  return http.put('/api/aslan/system/install/delete', data);
}


// Proxy
export function checkProxyAPI(payload) {
  return http.post(`/api/aslan/system/proxyManage/connectionTest`, payload);
}
export function getProxyConfigAPI() {
  return http.get(`/api/aslan/system/proxyManage`);
}
export function createProxyConfigAPI(payload) {
  return http.post(`/api/aslan/system/proxyManage`, payload);
}
export function updateProxyConfigAPI(id, payload) {
  return http.put(`/api/aslan/system/proxyManage/${id}`, payload);
}

// Cache
export function cleanCacheAPI() {
  return http.post(`/api/aslan/system/cleanCache/oneClick`);
}
export function getCleanCacheStatusAPI() {
  return http.get(`/api/aslan/system/cleanCache/state`);
}

// Registry 
export function getRegistryListAPI() {
  return http.get(`/api/aslan/system/registry/namespaces`);
}

export function createRegistryAPI(payload) {
  return http.post(`/api/aslan/system/registry/namespaces`, payload);
}

export function updateRegistryAPI(id, payload) {
  return http.put(`/api/aslan/system/registry/namespaces/${id}`, payload);
}

export function deleteRegistryAPI(id) {
  return http.delete(`/api/aslan/system/registry/namespaces/${id}`);
}

// OSS
export function getStorageListAPI() {
  return http.get(`/api/aslan/system/s3storage`);
}

export function createStorageAPI(payload) {
  return http.post(`/api/aslan/system/s3storage`, payload);
}

export function updateStorageAPI(id, payload) {
  return http.put(`/api/aslan/system/s3storage/${id}`, payload);
}

export function deleteStorageAPI(id) {
  return http.delete(`/api/aslan/system/s3storage/${id}`);
}

// Delivery Center

export function getArtifactsAPI(per_page, page, name = '', type = '', image = '', repoName = '', branch = '') {
  return http.get(`/api/aslan/delivery/artifacts?per_page=${per_page}&page=${page}&name=${name}&type=${type}&image=${image}&repoName=${repoName}&branch=${branch}`);
}

export function getArtifactsDetailAPI(id) {
  return http.get(`/api/aslan/delivery/artifacts/${id}`);
}

export function addArtifactActivitiesAPI(id, payload) {
  return http.post(`/api/aslan/delivery/artifacts/${id}/activities`, payload);
}

// Project
export function getProjectsAPI(productType = '') {
  return http.get(`/api/aslan/project/products?productType=${productType}`);
}

export function getSingleProjectAPI(projectName) {
  return http.get(`/api/aslan/project/products/${projectName}/services`);
}

export function getProjectInfoAPI(projectName) {
  return http.get(`/api/aslan/project/products/${projectName}`);
}


export function updateSingleProjectAPI(projectName, payload) {
  return http.put(`/api/aslan/project/products?productName=${projectName}`, payload);
}

export function createProjectAPI(payload) {
  return http.post(`/api/aslan/project/products`, payload);
}

export function deleteProjectAPI(projectName) {
  return http.delete(`/api/aslan/project/products/${projectName}`);
}

export function downloadDevelopCLIAPI(os) {
  return http.get(`/api/aslan/kodespace/downloadUrl?os=${os}`);
}

export function getServicesTemplateWithSharedAPI(projectName) {
  return http.get(`/api/aslan/service/name?productName=${projectName}`);
}

export function updateEnvTemplateAPI(projectName, payload) {
  return http.put(`/api/aslan/project/products/${projectName}`, payload);
}

// Env and Service
export function createProductAPI(payload, envType = '') {
  return http.post('/api/aslan/environment/environments', payload);
}

export function updateServiceAPI(product, service, type, env, data, envType = '') {
  return http.put(`/api/aslan/environment/environments/${product}/services/${service}/${type}?envType=${envType}`, data, {
    params: {
      envName: env,
    },
  });
}

export function updateK8sEnvAPI(product_name, env_name, payload, envType = '') {
  return http.post(`/api/aslan/environment/environments/${product_name}?envName=${env_name}&envType=${envType}`, payload);
}

export function recycleEnvAPI(product_name, env_name, recycle_day) {
  return http.put(`/api/aslan/environment/environments/${product_name}/envRecycle?envName=${env_name}&recycleDay=${recycle_day}`);
}

export function getConfigmapAPI(query) {
  return http.get(`/api/aslan/environment/configmaps?${query}`);
}

export function updateConfigmapAPI(envType = '', payload) {
  return http.put(`/api/aslan/environment/configmaps?envType=${envType}`, payload);
}

export function rollbackConfigmapAPI(envType = '', payload) {
  return http.post(`/api/aslan/environment/configmaps?envType=${envType}`, payload);
}

export function deleteProductEnvAPI(productName, envName, envType = '') {
  return http.delete(`/api/aslan/environment/environments/${productName}?envName=${envName}&envType=${envType}`);
}

export function restartPodAPI(podName, productName, envType = '') {
  return http.delete(`/api/aslan/environment/kube/pods/${podName}?productName=${productName}&envType=${envType}`);
}

export function restartServiceOriginAPI(productName, serviceName, envName = '', envType = '') {
  return http.post(`/api/aslan/environment/environments/${productName}/services/${serviceName}/restart?envName=${envName}&envType=${envType}`);
}

export function restartServiceAPI(productName, serviceName, envName = '', scaleName, type, envType = '') {
  return http.post(`/api/aslan/environment/environments/${productName}/services/${serviceName}/restartNew?envName=${envName}&type=${type}&name=${scaleName}&envType=${envType}`);
}

export function scaleServiceAPI(productName, serviceName, envName = '', scaleName, scaleNumber, type, envType = '') {
  return http.post(
    `/api/aslan/environment/environments/${productName}/services/${serviceName}/scaleNew/${scaleNumber}?envName=${envName}&type=${type}&name=${scaleName}&envType=${envType}`
  );
}

export function scaleEventAPI(productName, scaleName, envName = '', type, envType = '') {
  return http.get(`/api/aslan/environment/kube/events?productName=${productName}&envName=${envName}&type=${type}&name=${scaleName}&envType=${envType}`);
}
export function podEventAPI(productName, podName, envName = '', envType = '') {
  return http.get(`/api/aslan/environment/kube/pods/${podName}/events?productName=${productName}&envName=${envName}&envType=${envType}`);
}
export function exportYamlAPI(productName, serviceName, envName = '', envType = '') {
  return http.get(`/api/aslan/environment/export/service?serviceName=${serviceName}&envName=${envName}&productName=${productName}&envType=${envType}`);
}

export function getProductInfo(productName, envName = '') {
  return http.get(`/api/aslan/environment/environments/${productName}?envName=${envName}`);
}

export function getServiceInfo(productName, serviceName, envName = '', envType = '') {
  return http.get(`/api/aslan/environment/environments/${productName}/services/${serviceName}?envName=${envName}&envType=${envType}`);
}

export function autoUpgradeEnvAPI(projectName, payload) {
  return http.put(`/api/aslan/environment/environments/${projectName}/autoUpdate`, payload);
}

// Login
export function userLoginAPI(organization_id, payload) {
  return http.post(`/api/directory/user/login?orgId=${organization_id}`, payload);
}
export function userLogoutAPI() {
  return http.post(`/api/directory/user/logout`);
}

// Profile
export function getCurrentUserInfoAPI() {
  return http.get(`/api/directory/user/detail`);
}

export function updateCurrentUserInfoAPI(id, payload) {
  return http.put(`/api/directory/user/${id}`, payload);
}

export function getJwtTokenAPI() {
  return http.get(`/api/aslan/token`);
}

export function getSubscribeAPI() {
  return http.get(`/api/aslan/enterprise/notification/subscribe`);
}

export function saveSubscribeAPI(payload) {
  return http.post(`/api/aslan/enterprise/notification/subscribe`, payload);
}

export function downloadPubKeyAPI() {
  return http.get(`/api/aslan/setting/user/kube/config`);
}

export function updateServiceImageAPI(payload, type, envType = '') {
  return http.post(`/api/aslan/environment/image/${type}?envType=${envType}`, payload);
}

// Notification
export function getNotificationAPI() {
  return http.get(`/api/aslan/enterprise/notification`);
}

export function deleteAnnouncementAPI(payload) {
  return http.post(`/api/aslan/enterprise/announcement/delete`, payload);
}

export function markNotiReadAPI(payload) {
  return http.put(`/api/aslan/enterprise/notification/read`, payload);
}

// Onboarding

export function saveOnboardingStatusAPI(projectName, status) {
  return http.put(`/api/aslan/project/products/${projectName}/${status}`);
}

export function validateYamlAPI(payload) {
  return http.put(`/api/aslan/service/services/yaml/validator`, payload);
}

export function generateEnvAPI(projectName, envType = '') {
  return http.post(`/api/aslan/environment/environments/${projectName}/auto?envType=${envType}`);
}

export function generatePipeAPI(projectName) {
  return http.post(`/api/aslan/workflow/workflow/${projectName}/auto`);
}

export function getProjectIngressAPI(projectName) {
  return http.get(`/api/aslan/environment/environments/${projectName}/ingressInfo`);
}
