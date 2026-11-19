-- =====================================================================
-- V300231：moomoo 网页接口代理通道配置（单配置项，需求 11337）
--
-- 背景：直连 www.moomoo.com 会被按 IP 阻断（实测 TLS 直接 reset，采集报 EOF）；
--       经 Cloudflare Worker 中转可正常拿到行情（实测 15/15 成功、翻 6 页 6/6 成功）。
--       为便于随时开关与切换端点，新增**一个**系统配置项（不做两项拆分）：
--
--   20018  moomoo 网页接口代理配置
--          值格式：{enable}|{base_url}（半角竖线分隔）
--            enable   ：1/true/on = 启用代理；0/false/off/空 = 直连（默认）
--            base_url ：代理端点根地址（由运维/用户自行填写，初始发放**不预置**）
--          示例：1|{代理端点根地址}
--          留空、缺行、enable 非启用值 → 一律直连（不会因为配错地址把采集搞挂）
--
-- 只作用于网页接口（quote-api：列表 / K 线 / ETF / 期货 / 汇率 / 日 K），
-- **不作用于** moomoo OpenAPI（webapi，AppKey + Ed25519 签名）——凭据不应经第三方中转。
--
-- 幂等：已存在则不覆盖（保留运维已填的值）。
-- 回滚：DELETE FROM sys.sys_config WHERE config_no = 20018;（或把值改成 0| 关闭）
-- =====================================================================

BEGIN;

INSERT INTO sys.sys_config
    (config_no, config_name, data_type, char_value, str_value, access_level, remark)
VALUES (
    20018,
    'moomoo 网页接口代理配置',
    '3',
    '',
    '',
    1,
    '格式 {enable}|{base_url}：enable 取 1/true/on 启用、0/false/off/空 直连；'
    'base_url 为代理端点根地址（由用户自行填写，初始不预置任何地址）；'
    '留空或 enable 非启用值一律直连。仅作用于网页行情接口（quote-api），不作用于 moomoo OpenAPI（webapi）'
)
ON CONFLICT (config_no) DO NOTHING;

COMMIT;
