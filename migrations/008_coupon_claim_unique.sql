-- 008_coupon_claim_unique.sql 领券防重：同一用户同一券只能领一张
-- 先清理历史重复领取记录（保留最早一张），再建唯一索引
DELETE ci FROM coupon_instances ci
JOIN coupon_instances keep ON keep.def_id = ci.def_id AND keep.user_id = ci.user_id AND keep.id < ci.id;
ALTER TABLE coupon_instances ADD UNIQUE KEY uk_coupon_user (def_id, user_id);
