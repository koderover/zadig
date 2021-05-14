function getRoute(status, type, projectName, pipelineId = '') {
  const routes = {
    basic: [
      `/v1/projects/create/${projectName}/basic/info?rightbar=step`,
      `/v1/projects/create/${projectName}/basic/service?rightbar=help`,
      `/v1/projects/create/${projectName}/basic/runtime`,
      `/v1/projects/create/${projectName}/basic/delivery`,
    ],
  };
  return routes[type][status];
}

export function whetherOnboarding(projectTemplate) {
  var status = projectTemplate.onboarding_status;
  var projectName = projectTemplate.product_name;
  return getRoute(status - 1, 'basic', projectName);
}
