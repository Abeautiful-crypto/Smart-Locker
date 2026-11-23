-- 009_token_ver.sql JWT 令牌版本：退出/改角色/禁用时版本号+1，使旧 JWT 立即失效
ALTER TABLE users ADD COLUMN token_ver INT NOT NULL DEFAULT 0;
ALTER TABLE staff ADD COLUMN token_ver INT NOT NULL DEFAULT 0;
