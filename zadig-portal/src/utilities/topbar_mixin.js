import bus from '@utils/event_bus';

export default {
  methods: {
    checkCurrentTab() {
      return new Promise((resolve, reject) => {
        bus.$once(`receive-title`, pass => {
          if(pass) {
            resolve();
          } else {
            reject();
          }
        });
        bus.$emit(`set-topbar-title`,{title:'项目'});
      });
    }
  }
};
