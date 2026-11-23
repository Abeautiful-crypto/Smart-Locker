-- 010_fix_revenue_income.sql 修正历史错误记账：
-- 旧逻辑把 income 记成"实收时长费"(免费订单=0)，导致退款>收入、净收益为负。
-- 正确口径：income 应记支付时的预付金额（关联订单的 prepaid_fen）。
UPDATE revenue_records rr
JOIN orders o ON o.id = rr.order_id
SET rr.amount_fen = IF(o.pay_mode = 'deposit', o.deposit_fen, o.prepaid_fen)
WHERE rr.kind = 'income' AND rr.amount_fen <> IF(o.pay_mode = 'deposit', o.deposit_fen, o.prepaid_fen);
