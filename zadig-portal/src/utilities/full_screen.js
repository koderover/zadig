export const fullScreen = (id) => {
  const element = document.getElementById(id)
  const requestMethod = element.requestFullScreen || // W3C
  element.webkitRequestFullScreen || // FireFox
  element.mozRequestFullScreen || // Chrome等
  element.msRequestFullScreen // IE11
  if(requestMethod) {
    requestMethod.call(element)
  }
}