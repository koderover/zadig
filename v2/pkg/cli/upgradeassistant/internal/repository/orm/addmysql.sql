use `%s`;

ALTER TABLE `user` ADD `api_token` varchar(1024) NOT NULL DEFAULT '' COMMENT 'openAPIToken';