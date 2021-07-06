function getRoute (status, type, projectName) {
  const routes = {
    basic: [
      `/v1/projects/create/${projectName}/basic/info?rightbar=step`,
      `/v1/projects/create/${projectName}/basic/service?rightbar=help`,
      `/v1/projects/create/${projectName}/basic/runtime`,
      `/v1/projects/create/${projectName}/basic/delivery`
    ],
    not_k8s: [
      `/v1/projects/create/${projectName}/not_k8s/info`,
      `/v1/projects/create/${projectName}/not_k8s/config`,
      `/v1/projects/create/${projectName}/not_k8s/deploy`,
      `/v1/projects/create/${projectName}/not_k8s/delivery`
    ],
    helm: [
      `/v1/projects/create/${projectName}/helm/info?rightbar=step`,
      `/v1/projects/create/${projectName}/helm/service?rightbar=help`,
      `/v1/projects/create/${projectName}/helm/runtime`,
      `/v1/projects/create/${projectName}/helm/delivery`
    ]
  }
  return routes[type][status]
}

export function whetherOnboarding (projectTemplate) {
  const productFeature = projectTemplate.product_feature
  const status = projectTemplate.onboarding_status
  const projectName = projectTemplate.product_name
  if (productFeature.deploy_type === 'k8s') {
    return getRoute(status - 1, 'basic', projectName)
  } else if (productFeature.deploy_type === 'helm') {
    return getRoute(status - 1, 'helm', projectName)
  } else if (productFeature.basic_facility === 'cloud_host') {
    return getRoute(status - 1, 'not_k8s', projectName)
  }
}
