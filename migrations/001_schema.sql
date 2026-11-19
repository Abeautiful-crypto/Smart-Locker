-- 001_schema.sql Smart-Locker 初始结构
SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS schema_migrations (
  version VARCHAR(32) PRIMARY KEY,
  applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS users (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  phone VARCHAR(20) NOT NULL UNIQUE,
  pickup_pwd VARCHAR(64) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS staff (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  emp_no VARCHAR(32) NOT NULL UNIQUE,
  account VARCHAR(64) NOT NULL UNIQUE,
  name VARCHAR(64) NOT NULL,
  phone VARCHAR(20) NOT NULL UNIQUE,
  role ENUM('super','site_admin','service') NOT NULL,
  password_hash VARCHAR(128) NOT NULL,
  status ENUM('active','disabled') NOT NULL DEFAULT 'active',
  scope_outlet_id BIGINT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS outlets (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(64) NOT NULL,
  address VARCHAR(128) NOT NULL DEFAULT '',
  city VARCHAR(32) NOT NULL,
  status ENUM('normal','paused') NOT NULL DEFAULT 'normal',
  manager_staff_id BIGINT NULL,
  lat DOUBLE NOT NULL DEFAULT 0,
  lng DOUBLE NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS cabinet_groups (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  outlet_id BIGINT NOT NULL,
  name VARCHAR(32) NOT NULL
);

CREATE TABLE IF NOT EXISTS lockers (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  outlet_id BIGINT NOT NULL,
  group_id BIGINT NOT NULL,
  code VARCHAR(16) NOT NULL,
  size ENUM('S','M','L') NOT NULL,
  status ENUM('idle','occupied','fault') NOT NULL DEFAULT 'idle',
  fault_source ENUM('user_report','system_check') NULL,
  fault_note VARCHAR(128) NOT NULL DEFAULT '',
  current_order_id BIGINT NULL,
  UNIQUE KEY uk_group_code (group_id, code)
);

CREATE TABLE IF NOT EXISTS outlet_rules (
  outlet_id BIGINT PRIMARY KEY,
  mode ENUM('hourly','daily') NOT NULL DEFAULT 'hourly',
  price_s INT NOT NULL DEFAULT 500,
  price_m INT NOT NULL DEFAULT 700,
  price_l INT NOT NULL DEFAULT 1000,
  deposit_s INT NOT NULL DEFAULT 1000,
  deposit_m INT NOT NULL DEFAULT 2000,
  deposit_l INT NOT NULL DEFAULT 3000,
  cap_s INT NOT NULL DEFAULT 5000,
  cap_m INT NOT NULL DEFAULT 7000,
  cap_l INT NOT NULL DEFAULT 10000,
  free_minutes INT NOT NULL DEFAULT 10,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS orders (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  order_no VARCHAR(32) NOT NULL UNIQUE,
  user_id BIGINT NOT NULL,
  outlet_id BIGINT NOT NULL,
  locker_id BIGINT NOT NULL,
  size ENUM('S','M','L') NOT NULL,
  pay_mode ENUM('prepaid','deposit') NOT NULL DEFAULT 'prepaid',
  prepaid_fen INT NOT NULL DEFAULT 0,
  deposit_fen INT NOT NULL DEFAULT 0,
  coupon_instance_id BIGINT NULL,
  coupon_discount_fen INT NOT NULL DEFAULT 0,
  code CHAR(6) NOT NULL,
  status ENUM('active','finished') NOT NULL DEFAULT 'active',
  started_at DATETIME NOT NULL,
  ended_at DATETIME NULL,
  fee_fen INT NOT NULL DEFAULT 0,
  refund_fen INT NOT NULL DEFAULT 0,
  extra_fen INT NOT NULL DEFAULT 0,
  extra_paid TINYINT(1) NOT NULL DEFAULT 0,
  settle_note VARCHAR(255) NOT NULL DEFAULT '',
  pickup_method VARCHAR(16) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS coupon_defs (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(64) NOT NULL,
  ctype ENUM('discount','hour','amount') NOT NULL,
  value INT NOT NULL,
  scope VARCHAR(16) NOT NULL DEFAULT 'all',
  outlet_id BIGINT NULL,
  valid_from DATE NOT NULL,
  valid_to DATE NOT NULL,
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  total_count INT NOT NULL DEFAULT 0,
  used_count INT NOT NULL DEFAULT 0,
  note VARCHAR(255) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS coupon_instances (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  def_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  status ENUM('usable','used','expired') NOT NULL DEFAULT 'usable',
  used_order_id BIGINT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS alerts (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  title VARCHAR(64) NOT NULL,
  level ENUM('high','mid') NOT NULL DEFAULT 'mid',
  source VARCHAR(32) NOT NULL,
  outlet_id BIGINT NOT NULL,
  locker_id BIGINT NULL,
  description VARCHAR(255) NOT NULL DEFAULT '',
  status ENUM('open','handled') NOT NULL DEFAULT 'open',
  handler VARCHAR(64) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  handled_at DATETIME NULL
);

CREATE TABLE IF NOT EXISTS tickets (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  ticket_no VARCHAR(32) NOT NULL UNIQUE,
  user_id BIGINT NOT NULL,
  question VARCHAR(255) NOT NULL,
  category VARCHAR(32) NOT NULL DEFAULT '',
  status ENUM('pending','resolved') NOT NULL DEFAULT 'pending',
  reply VARCHAR(255) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  resolved_at DATETIME NULL
);

CREATE TABLE IF NOT EXISTS kb_entries (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  category VARCHAR(32) NOT NULL,
  keywords VARCHAR(255) NOT NULL,
  answer TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS chat_messages (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT NOT NULL,
  sender ENUM('user','bot','agent','system') NOT NULL,
  content VARCHAR(512) NOT NULL,
  channel VARCHAR(16) NOT NULL DEFAULT 'http',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS chat_sessions (
  user_id BIGINT PRIMARY KEY,
  state ENUM('bot','waiting','agent') NOT NULL DEFAULT 'bot',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
  token VARCHAR(64) PRIMARY KEY,
  kind ENUM('user','staff') NOT NULL,
  ref_id BIGINT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  expires_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS payments (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  order_no VARCHAR(32) NOT NULL,
  kind ENUM('prepaid','deposit','extra') NOT NULL,
  amount_fen INT NOT NULL,
  channel VARCHAR(16) NOT NULL DEFAULT 'wechat_mock',
  status ENUM('paid','refunded') NOT NULL DEFAULT 'paid',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS revenue_records (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  outlet_id BIGINT NOT NULL,
  order_id BIGINT NOT NULL,
  group_id BIGINT NOT NULL,
  kind ENUM('income','refund','extra') NOT NULL,
  amount_fen INT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
