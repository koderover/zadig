import Vue from 'vue';
import { upperFirst, camelCase } from 'lodash';
const requireComponent = require.context('../common', false, /\.vue$/);
requireComponent.keys().forEach((fileName) => {
  const componentConfig = requireComponent(fileName);
  const componentName = upperFirst(
    camelCase(
      fileName
        .split('/')
        .pop()
        .replace(/\.\w+$/, '')
    )
  );
  Vue.component(
    componentName,
    componentConfig.default || componentConfig
  );
});
