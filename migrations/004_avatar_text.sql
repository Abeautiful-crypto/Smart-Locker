-- 004_avatar_text.sql 头像支持上传本地图片（base64 data URL），需更大存储。
-- MySQL 的 TEXT 列不支持 DEFAULT，故用足够大的 VARCHAR（128x128 JPEG 约 5-15KB）。
ALTER TABLE users MODIFY COLUMN avatar VARCHAR(16000) NOT NULL DEFAULT '';
