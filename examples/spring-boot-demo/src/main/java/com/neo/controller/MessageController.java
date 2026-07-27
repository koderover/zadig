
package com.neo.controller;

import com.neo.config.BaseResult;
import com.neo.model.Message;
import com.neo.repository.MessageRepository;
import io.swagger.v3.oas.annotations.Operation;
import io.swagger.v3.oas.annotations.responses.ApiResponse;
import io.swagger.v3.oas.annotations.responses.ApiResponses;
import io.swagger.v3.oas.annotations.tags.Tag;
import org.springdoc.core.annotations.ParameterObject;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.*;

import java.util.List;

@Tag(name = "消息", description = "消息操作 API")
@RestController
@RequestMapping("/")
public class MessageController {

	@Autowired
	private  MessageRepository messageRepository;

	@Operation(summary = "消息列表", description = "完整的消息内容列表")
	@GetMapping(value = "messages")
	public List<Message> list() {
		List<Message> messages = this.messageRepository.findAll();
		return messages;
	}

	@Operation(summary = "添加消息", description = "根据参数创建消息")
	@PostMapping(value = "message")
	public Message create(@ParameterObject Message message) {
		System.out.println("message===="+message.toString());
		message = this.messageRepository.save(message);
		return message;
	}

	@Operation(summary = "修改消息", description = "根据参数修改消息")
	@PutMapping(value = "message")
	@ApiResponses({
			@ApiResponse(responseCode = "100", description = "请求参数有误"),
			@ApiResponse(responseCode = "101", description = "未授权"),
			@ApiResponse(responseCode = "103", description = "禁止访问"),
			@ApiResponse(responseCode = "104", description = "请求路径不存在"),
			@ApiResponse(responseCode = "200", description = "服务器内部错误")
	})
	public Message modify(@ParameterObject Message message) {
		Message messageResult=this.messageRepository.update(message);
		return messageResult;
	}

	@PatchMapping(value="/message/text")
	public BaseResult<Message> patch(Message message) {
		Message messageResult=this.messageRepository.updateText(message);
		return BaseResult.successWithData(messageResult);
	}

	@GetMapping(value = "message/{id}")
	public Message get(@PathVariable Long id) {
		Message message = this.messageRepository.findMessage(id);
		return message;
	}

	@DeleteMapping(value = "message/{id}")
	public void delete(@PathVariable("id") Long id) {
		this.messageRepository.deleteMessage(id);
	}



}
