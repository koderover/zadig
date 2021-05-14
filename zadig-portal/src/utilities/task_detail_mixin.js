export default {
  methods: {
    killLog(type) {
      clearInterval(this[`${type}IntervalHandle`]);
      if (typeof msgServer !== 'undefined' && msgServer) {
        for (const key in msgServer) {
          if (msgServer.hasOwnProperty(key)) {
            msgServer[key].close();
            console.info('Clean SSE '+ key);
          }
        }
        delete window.msgServer;
      } else {
        return;
      }
    },
    isSubTaskDone(subTask) {
      return (
        subTask &&
        subTask.status in
          {
            passed: 1,
            skipped: 1,
            failed: 1,
            timeout: 1,
            cancelled: 1
          }
      );
    }
  }
};
