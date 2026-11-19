-- 002_seed.sql 演示数据
SET NAMES utf8mb4;

-- 员工：超管/网点管理员/客服，密码 000000（sha256 十六进制）
INSERT INTO staff (emp_no, account, name, phone, role, password_hash, scope_outlet_id)
SELECT 'E001','super','系统超管','13800000001','super',SHA2('000000',256),NULL
WHERE NOT EXISTS (SELECT 1 FROM staff WHERE account='super');

INSERT INTO outlets (name, address, city, status, lat, lng)
SELECT * FROM (SELECT '人民广场站' AS a,'上海市黄浦区人民广场地铁站B1' AS b,'上海' AS c,'normal' AS d,31.2323 AS e,121.4691 AS f UNION ALL
SELECT '虹桥枢纽站','上海市闵行区虹桥火车站到达层','上海','normal',31.1946,121.3363 UNION ALL
SELECT '二七广场站','郑州市二七区二七广场步行街','郑州','normal',34.7535,113.6654 UNION ALL
SELECT '东站立柱站','郑州市郑东新区高铁东站东广场','郑州','normal',34.7466,113.7728 UNION ALL
SELECT '福田枢纽站','深圳市福田区福田高铁站换乘层','深圳','normal',22.5392,114.0579 UNION ALL
SELECT '世界之窗站','深圳市南山区世界之窗地铁口','深圳','normal',22.5349,113.9726 UNION ALL
SELECT '西湖银泰站','杭州市上城区西湖银泰in77','杭州','normal',30.2429,120.1626 UNION ALL
SELECT '城站广场站','杭州市上城区杭州城站广场','杭州','normal',30.2435,120.1806) t
WHERE NOT EXISTS (SELECT 1 FROM outlets WHERE name=t.a);

-- 网点管理员绑定人民广场站
UPDATE outlets o JOIN staff s ON s.account='site_rm' SET o.manager_staff_id=s.id WHERE o.name='人民广场站';

INSERT INTO staff (emp_no, account, name, phone, role, password_hash, scope_outlet_id)
SELECT 'E002','site_rm','王网点','13800000002','site_admin',SHA2('000000',256),
  (SELECT id FROM outlets WHERE name='人民广场站')
WHERE NOT EXISTS (SELECT 1 FROM staff WHERE account='site_rm');

INSERT INTO staff (emp_no, account, name, phone, role, password_hash, scope_outlet_id)
SELECT 'E003','svc','李客服','13800000003','service',SHA2('000000',256),NULL
WHERE NOT EXISTS (SELECT 1 FROM staff WHERE account='svc');

-- 每网点 A/B 两个柜组，每组 18 格（S6/M8/L4）
INSERT INTO cabinet_groups (outlet_id, name)
SELECT o.id, g.n FROM outlets o JOIN (SELECT 'A组' n UNION ALL SELECT 'B组' g2) g
WHERE NOT EXISTS (SELECT 1 FROM cabinet_groups cg WHERE cg.outlet_id=o.id AND cg.name=g.n);

INSERT INTO lockers (outlet_id, group_id, code, size)
SELECT cg.outlet_id, cg.id,
  CONCAT(SUBSTRING(cg.name,1,1),'-',LPAD(seq.n,3,'0')),
  CASE WHEN seq.n<=6 THEN 'S' WHEN seq.n<=14 THEN 'M' ELSE 'L' END
FROM cabinet_groups cg
JOIN (SELECT 1 n UNION ALL SELECT 2 UNION ALL SELECT 3 UNION ALL SELECT 4 UNION ALL SELECT 5 UNION ALL SELECT 6
      UNION ALL SELECT 7 UNION ALL SELECT 8 UNION ALL SELECT 9 UNION ALL SELECT 10 UNION ALL SELECT 11 UNION ALL SELECT 12
      UNION ALL SELECT 13 UNION ALL SELECT 14 UNION ALL SELECT 15 UNION ALL SELECT 16 UNION ALL SELECT 17 UNION ALL SELECT 18) seq
WHERE NOT EXISTS (SELECT 1 FROM lockers l WHERE l.group_id=cg.id AND l.code=CONCAT(SUBSTRING(cg.name,1,1),'-',LPAD(seq.n,3,'0')));

-- 收费规则（默认按时计费，题面标准价）
INSERT INTO outlet_rules (outlet_id, mode)
SELECT o.id,'hourly' FROM outlets o
WHERE NOT EXISTS (SELECT 1 FROM outlet_rules r WHERE r.outlet_id=o.id);

-- 知识库
INSERT INTO kb_entries (category, keywords, answer)
SELECT * FROM (
SELECT '收费计费' a,'收费,计费,价格,多少钱,费用' b,'小格5元/小时、中格7元/小时、大格10元/小时，每24小时封顶分别为50元、70元、100元；按日计费为5元、7元、10元每日。' c
UNION ALL SELECT '免费时长','免费,免费时长,10分钟','每笔订单享有10分钟免费时长，寄存后10分钟内结束订单不收费、全额退款。'
UNION ALL SELECT '取件方式','取件方式,怎么取,取件','支持三种取件方式：手机扫码取件、人脸识别取件、手机号加取件密码，任一方式校验通过即可开柜。'
UNION ALL SELECT '取件码','取件码,开柜码,凭证','寄存成功后系统生成6位数字取件码，可在订单页查看，取件时输入取件码即可开柜。'
UNION ALL SELECT '退款','退款,退钱,退费','订单结束时按实际时长结算：费用未超已付金额时，剩余预付与押金原路退回，一般1至3个工作日到账。'
UNION ALL SELECT '超时补交','超时,补交,补款','费用超过已付金额时，需补交差额后方可完成取件，可在订单详情页操作。'
UNION ALL SELECT '优惠券','优惠券,卡券,折扣,抵扣','优惠券在下单时选择、结束订单时结算：8折券减时长费20%，1小时券抵扣1小时费用，1元券立减1元，均不超过订单金额。'
UNION ALL SELECT '柜门异常','柜门,异常,故障,打不开','若柜门未弹开或柜格故障，可在存件页点击"上报柜格异常"选择问题提交，我们会尽快处理并为您更换柜格。'
UNION ALL SELECT '营业时间','营业,时间,几点,营业时间','寄存柜7×24小时可用；人工客服工作时间为每日09:00至21:00，其余时间由智能客服提供服务。'
UNION ALL SELECT '押金','押金,保证金','押金标准为小格10元、中格20元、大格30元，订单结束后随余额原路退回。'
UNION ALL SELECT '发票','发票,报销','如需开具发票，请联系人工客服提供订单号与抬头信息，将在1个工作日内发送至您的邮箱。'
UNION ALL SELECT '物品安全','安全,丢失,监控','柜格具备独立电子锁并全程录像监控，如遇物品丢失请立即上报异常并联系客服，我们会调取记录核查。'
) t
WHERE NOT EXISTS (SELECT 1 FROM kb_entries);

-- 演示优惠券定义
INSERT INTO coupon_defs (name, ctype, value, scope, valid_from, valid_to, total_count, note)
SELECT * FROM (
SELECT '新人8折券' a,'discount' b,80 c,'all' d,CURDATE() e,DATE_ADD(CURDATE(),INTERVAL 30 DAY) f,100 g,'时长费用8折' h
UNION ALL SELECT '1小时免费券','hour',60,'all',CURDATE(),DATE_ADD(CURDATE(),INTERVAL 30 DAY),100,'抵扣1小时费用'
UNION ALL SELECT '1元立减券','amount',100,'all',CURDATE(),DATE_ADD(CURDATE(),INTERVAL 15 DAY),200,'结算立减1元'
) t
WHERE NOT EXISTS (SELECT 1 FROM coupon_defs WHERE name=t.a);
