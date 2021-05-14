export default class ValidateSubmit {
  constructor() {
    this.validateArr = []; 
    this.validateBoo = false;
    this.validateRes = [];
  }
  get validatePros() {
    return this.validateArr.map((validate) => {
      return validate.valid();
    });
  }
  getValidId(valid) {
    return this.validateArr.map((valid) => valid.name).indexOf(valid.name);
  }
  addValidate(valid) {
    this.getValidId(valid) < 0 && this.validateArr.push(valid);
  }
  deleteValidate(valid) {
    let id = this.getValidId(valid);
    id > -1 && this.validateArr.splice(id, 1);
  }
  async validateAll() {
    await Promise.all(this.validatePros)
      .then((res) => {
        this.validateBoo = true;
        this.validateRes = res;
      })
      .catch((err) => {
        console.log('check failed', err);
        this.validateBoo = false;
      });
    return [this.validateRes, this.validateBoo];
  }
}
