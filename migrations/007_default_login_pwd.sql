-- 007_default_login_pwd.sql 演示环境：所有用户登录密码统一为 123456（SHA-256 存储）。
UPDATE users SET login_pwd = '8d969eef6ecad3c29a3a629280e686cf0c3f5d5a86aff3ca12020c923adc6c92';
