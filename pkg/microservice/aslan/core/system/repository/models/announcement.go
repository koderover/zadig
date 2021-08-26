package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type Announcement struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"           json:"id,omitempty"` // 主键
	Receiver   string             `bson:"receiver"                json:"receiver"`     // 发送者
	Content    *Content           `bson:"content"                 json:"content"`      // 消息内容
	CreateTime int64              `bson:"create_time"             json:"create_time"`  // 消息创建时间
	IsRead     bool               `bson:"is_read"                 json:"is_read"`      // 是否已读
}

type Content struct {
	Title     string `bson:"title"                 json:"title"`      // 公告标题
	Priority  int    `bson:"priority"              json:"priority"`   // 公告级别
	Content   string `bson:"content"               json:"content"`    // 公告内容
	StartTime int64  `bson:"start_time"            json:"start_time"` // 公告开始时间
	EndTime   int64  `bson:"end_time"              json:"end_time"`   // 公告结束时间
}

func (Announcement) TableName() string {
	return "announcement"
}
