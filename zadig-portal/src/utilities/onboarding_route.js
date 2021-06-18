function getRoute (status, type, projectName, pipelineId = '') {
  const routes = {
    basic: [
      `/v1/projects/create/${projectName}/basic/info?rightbar=step`,
      `/v1/projects/create/${projectName}/basic/service?rightbar=help`,
      `/v1/projects/create/${projectName}/basic/runtime`,
      `/v1/projects/create/${projectName}/basic/delivery`
    ]
  }
  return routes[type][status]
}

export function whetherOnboarding (projectTemplate) {
  const status = projectTemplate.onboarding_status
  const projectName = projectTemplate.product_name
  return getRoute(status - 1, 'basic', projectName)
}
