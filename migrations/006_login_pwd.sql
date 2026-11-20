-- 006_login_pwd.sql 用户登录密码（优先使用；忘记密码可走验证码兜底）
ALTER TABLE users ADD COLUMN login_pwd VARCHAR(64) NOT NULL DEFAULT '';
