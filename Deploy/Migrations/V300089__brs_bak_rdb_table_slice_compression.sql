-- =====================================================================
-- V300089：大表分片备份的压缩口径收敛为 gzip / none（需求 10905 修订）
--
-- 背景：分片产物是 MVSV-2 文本文件（由 mvsv.WriteV2 写出 + gzip 压缩），
--   不使用 pg_dump 的 zstd；V300088 的默认值与 CHECK 沿用了小表全量的
--   ('none','gzip','zstd')，会让登记里出现 zstd 但实际按 gzip 写出的歧义。
--
-- 变更：
--   1) 列默认值 zstd → gzip；
--   2) CHECK 约束收敛为 ('none','gzip')；
--   3) 存量 zstd 行修正为 gzip（语义与实际产物一致）。
-- =====================================================================

BEGIN;

UPDATE brs.brs_bak_rdb_table_slice
   SET compression = 'gzip'
 WHERE compression = 'zstd';

ALTER TABLE brs.brs_bak_rdb_table_slice
    ALTER COLUMN compression SET DEFAULT 'gzip';

ALTER TABLE brs.brs_bak_rdb_table_slice
    DROP CONSTRAINT IF EXISTS ck_brs_bak_rdb_table_slice_compression;

ALTER TABLE brs.brs_bak_rdb_table_slice
    ADD CONSTRAINT ck_brs_bak_rdb_table_slice_compression
    CHECK (compression IN ('none', 'gzip'));

COMMENT ON COLUMN brs.brs_bak_rdb_table_slice.compression IS
    '压缩：gzip（默认，产物 .mvsv.gz）/ none（.mvsv）；分片产物为 MVSV-2 文本，不使用 zstd';

COMMIT;
