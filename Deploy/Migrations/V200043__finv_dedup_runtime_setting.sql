-- 归档运行时设置（可在线修改，保存后立即生效，无需重启服务）。
-- 当前仅存放导出低峰窗口：EXPORT_WINDOW_START / EXPORT_WINDOW_END（HH:MM）。
-- 未写入记录时回退环境变量 FINV_QUOTE_DEDUB_WINDOW_START / _END。

CREATE TABLE IF NOT EXISTS dedup.finv_dedup_runtime_setting (
  setting_key   text PRIMARY KEY,
  setting_value text NOT NULL,
  gmt_update    timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE dedup.finv_dedup_runtime_setting IS '归档链路运行时设置（键值表）；写入后由服务端热加载，优先于环境变量默认值';
COMMENT ON COLUMN dedup.finv_dedup_runtime_setting.setting_key IS '设置键：EXPORT_WINDOW_START / EXPORT_WINDOW_END（扩展用）';
COMMENT ON COLUMN dedup.finv_dedup_runtime_setting.setting_value IS '设置值：时间为 HH:MM（24 小时制），支持跨零点窗口';
COMMENT ON COLUMN dedup.finv_dedup_runtime_setting.gmt_update IS '最近更新时间';
