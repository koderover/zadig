use `%s`;

LOCK TABLES `user` WRITE;
 ALTER TABLE `user` DROP COLUMN `api_token`;
 UNLOCK TABLES;