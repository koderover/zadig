package com.neo.config;

import io.swagger.v3.oas.annotations.media.Schema;

/**
 * 通用响应对象
 */
@Schema(description = "响应对象")
public class BaseResult<T> {
    private static final int SUCCESS_CODE = 0;
    private static final String SUCCESS_MESSAGE = "成功";

    @Schema(description = "响应码", requiredMode = Schema.RequiredMode.REQUIRED, example = "" + SUCCESS_CODE)
    private int code;

    @Schema(description = "响应消息", requiredMode = Schema.RequiredMode.REQUIRED, example = SUCCESS_MESSAGE)
    private String msg;

    @Schema(description = "响应数据")
    private T data;

    private BaseResult(int code, String msg, T data) {
        this.code = code;
        this.msg = msg;
        this.data = data;
    }

    private BaseResult() {
        this(SUCCESS_CODE, SUCCESS_MESSAGE);
    }

    private BaseResult(int code, String msg) {
        this(code, msg, null);
    }

    private BaseResult(T data) {
        this(SUCCESS_CODE, SUCCESS_MESSAGE, data);
    }

    public static <T> BaseResult<T> success() {
        return new BaseResult<>();
    }

    public static <T> BaseResult<T> successWithData(T data) {
        return new BaseResult<>(data);
    }

    public static <T> BaseResult<T> failWithCodeAndMsg(int code, String msg) {
        return new BaseResult<>(code, msg, null);
    }

    public static <T> BaseResult<T> buildWithParam(ResponseParam param) {
        return new BaseResult<>(param.getCode(), param.getMsg(), null);
    }

    public int getCode() {
        return code;
    }

    public void setCode(int code) {
        this.code = code;
    }

    public String getMsg() {
        return msg;
    }

    public void setMsg(String msg) {
        this.msg = msg;
    }

    public T getData() {
        return data;
    }

    public void setData(T data) {
        this.data = data;
    }



    public static class ResponseParam {
        private int code;
        private String msg;

        private ResponseParam(int code, String msg) {
            this.code = code;
            this.msg = msg;
        }

        public static ResponseParam buildParam(int code, String msg) {
            return new ResponseParam(code, msg);
        }

        public int getCode() {
            return code;
        }

        public void setCode(int code) {
            this.code = code;
        }

        public String getMsg() {
            return msg;
        }

        public void setMsg(String msg) {
            this.msg = msg;
        }
    }
}
