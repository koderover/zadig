import bus from '@utils/event_bus';

export default {
  methods: {
    checkCurrentTab() {
      return new Promise((resolve, reject) => {
        bus.$once(`receive-tab-check:${this.currentTab}`, pass => {
          if(pass) {
            resolve();
          } else {
            reject();
          }
        });
        bus.$emit(`check-tab:${this.currentTab}`);
      });
    }
  }
};
