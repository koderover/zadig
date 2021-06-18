export const fullScreen = (id) => {
  const element = document.getElementById(id)
  const requestMethod =
    // W3C
    element.requestFullScreen ||
    // FireFox
    element.webkitRequestFullScreen ||
    // Chrome
    element.mozRequestFullScreen ||
    // IE11
    element.msRequestFullScreen
  if (requestMethod) {
    requestMethod.call(element)
  }
}
