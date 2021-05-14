import bus from '@utils/event_bus';

export default {
  methods: {
    checkServiceInput() {
      return new Promise((resolve, reject) => {
        bus.$once('receive-service-input', pass => {
          if(pass) {
            resolve(pass);
          } else {
            reject('NotPass');
          }
        });
        bus.$emit('check-service-input');
      });
    }
  }
};