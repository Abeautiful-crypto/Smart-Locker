-- 003_user_avatar.sql 用户头像（emoji 或短字符，演示用）
ALTER TABLE users ADD COLUMN avatar VARCHAR(32) NOT NULL DEFAULT '';
