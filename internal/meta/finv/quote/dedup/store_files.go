package dedup

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// PatchTask 补丁任务（cdeu.finv_quote_cdeu_export_file_patch 一行）。
type PatchTask struct {
	PatchID      int64
	JobID        int64
	Usc          string
	BizMonth     int
	PatchLevel   string
	PatchVersion int
	SourceRows   int64
	PatchRows    int64
	Reason       string
	ArrivalStart time.Time
	ArrivalEnd   time.Time
	Status       string
	PatchFileID  *int64
}

// Alert 健康检查告警项。
type Alert struct {
	Code    string `json:"code"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

// JobProgress 导出任务批次进度（按任务状态聚合，供端到端链路的批次大节点展示）。
type JobProgress struct {
	Total     int64 `json:"total"`
	Success   int64 `json:"success"`
	Failed    int64 `json:"failed"`
	Running   int64 `json:"running"`
	Waiting   int64 `json:"waiting"`
	Paused    int64 `json:"paused"`
	Frozen    int64 `json:"frozen"`
	Loading   int64 `json:"loading"`
	Loaded    int64 `json:"loaded"`
	Exporting int64 `json:"exporting"`

	PatchTotal   int64 `json:"patch_total"`
	PatchSuccess int64 `json:"patch_success"`
	PatchFailed  int64 `json:"patch_failed"`
	PatchRunning int64 `json:"patch_running"`
	PatchPending int64 `json:"patch_pending"`
}

// Metrics 归档链路指标（M-06 / 5.2）。
type Metrics struct {
	GeneratedAt              string           `json:"generated_at"`
	JobStatus                map[string]int64 `json:"job_status"`
	JobProgress              JobProgress      `json:"job_progress"`
	UploadStatus             map[string]int64 `json:"upload_status"`
	PendingRows              int64            `json:"pending_rows"`
	StagingRows              int64            `json:"staging_rows"`
	StagingUscs              int64            `json:"staging_uscs"`
	BacklogDays              float64          `json:"backlog_days"`
	BudgetFactor             float64          `json:"budget_factor"`
	BudgetRows               int64            `json:"budget_rows"`
	BudgetConsumedRows       int64            `json:"budget_consumed_rows"`
	BudgetUsageRatio         float64          `json:"budget_usage_ratio"`
	FrozenMissingRows        int64            `json:"frozen_missing_rows"`
	FrozenExpectedRows       int64            `json:"frozen_expected_rows"`
	FrozenMissingRatio       float64          `json:"frozen_missing_ratio"`
	HeartbeatTimeouts        int64            `json:"heartbeat_timeouts"`
	UploadRetryExhausted     int64            `json:"upload_retry_exhausted"`
	PatchLast7Days           int64            `json:"patch_last_7_days"`
	HighFrequencyPatchGroups int64            `json:"high_frequency_patch_groups"`
	LocalFilesPendingCleanup int64            `json:"local_files_pending_cleanup"`
	MvsvAnomalies            int64            `json:"mvsv_anomalies"`
	Alerts                   []Alert          `json:"alerts"`
}

// NextManifestVersion 返回下一个 MANIFEST 版本号（每次登记自增，保证唯一键不冲突）。
func (s *Store) NextManifestVersion(ctx context.Context, usc string, bizMonth int) (int, error) {
	var version int
	err := s.pool.QueryRow(ctx, `
SELECT COALESCE(max(version), 0) + 1
  FROM `+tableFile+`
 WHERE usc = $1 AND biz_month = $2 AND file_type = 'MANIFEST'`, usc, bizMonth).Scan(&version)
	return version, err
}

// RegisterFile 登记物理文件（幂等：唯一键 (usc, biz_month, file_type, version)）。
//
// 相同版本重复登记时：内容 checksum 未变且已上传则保持 UPLOADED，否则回到 PENDING_UPLOAD。
func (s *Store) RegisterFile(ctx context.Context, file *File) (int64, error) {
	var uploadTarget *string
	if file.UploadTarget != nil && strings.TrimSpace(*file.UploadTarget) != "" {
		uploadTarget = file.UploadTarget
	}
	var manifestPath *string
	if file.ManifestPath != nil && strings.TrimSpace(*file.ManifestPath) != "" {
		manifestPath = file.ManifestPath
	}
	var sealDate *time.Time
	if file.SealDate != nil {
		sealDate = file.SealDate
	}
	format := file.Format
	if format == "" {
		format = MvsvFormat
	}
	compression := file.Compression
	if compression == "" {
		compression = CompressionNone
	}
	exportStatus := file.ExportStatus
	if exportStatus == "" {
		exportStatus = ExportStatusExported
	}
	uploadStatus := file.UploadStatus
	if uploadStatus == "" {
		uploadStatus = UploadStatusPending
	}

	var fileID int64
	err := s.pool.QueryRow(ctx, `
INSERT INTO `+tableFile+`
  (job_id, usc, biz_month, file_type, version, file_name, file_path, target_file_name,
   upload_target, format, compression, row_count, file_size, checksum, manifest_path,
   seal_date, export_status, upload_status, upload_attempt, remote_checksum_verified,
   network_status, cleanup_status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15,
        $16::date, $17, $18, 0, false, 'NONE', 'NONE')
ON CONFLICT (usc, biz_month, file_type, version) DO UPDATE
   SET file_name = EXCLUDED.file_name,
       file_path = EXCLUDED.file_path,
       target_file_name = EXCLUDED.target_file_name,
       upload_target = EXCLUDED.upload_target,
       format = EXCLUDED.format,
       compression = EXCLUDED.compression,
       row_count = EXCLUDED.row_count,
       file_size = EXCLUDED.file_size,
       checksum = EXCLUDED.checksum,
       manifest_path = EXCLUDED.manifest_path,
       seal_date = EXCLUDED.seal_date,
       export_status = EXCLUDED.export_status,
       upload_status = CASE
           WHEN `+tableFile+`.checksum = EXCLUDED.checksum
                AND (`+tableFile+`.upload_status = 'UPLOADED'
                     -- MANIFEST 按业务约定不上传（保持 SKIPPED），避免重导出后又进待传队列
                     OR (`+tableFile+`.upload_status = 'SKIPPED'
                         AND EXCLUDED.file_type = 'MANIFEST'))
               THEN `+tableFile+`.upload_status
           ELSE 'PENDING_UPLOAD' END,
       remote_checksum_verified = (`+tableFile+`.checksum = EXCLUDED.checksum
                                   AND `+tableFile+`.remote_checksum_verified),
       gmt_next_retry = now(),
       gmt_update = now()
RETURNING file_id`,
		file.JobID, file.Usc, file.BizMonth, file.FileType, file.Version,
		file.FileName, file.FilePath, file.TargetFileName, uploadTarget, format, compression,
		file.RowCount, file.FileSize, file.Checksum, manifestPath, sealDate,
		exportStatus, uploadStatus).Scan(&fileID)
	return fileID, err
}

// GetFile 读取单个文件登记。
func (s *Store) GetFile(ctx context.Context, fileID int64) (*File, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+fileColumns+` FROM `+tableFile+` WHERE file_id = $1`, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	file, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[File])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &file, nil
}

// LatestBaseFile 读取任务最新生效的 BASE 文件。
func (s *Store) LatestBaseFile(ctx context.Context, jobID int64) (*File, error) {
	rows, err := s.pool.Query(ctx, `
SELECT `+fileColumns+`
  FROM `+tableFile+`
 WHERE job_id = $1 AND file_type = 'BASE' AND export_status = 'EXPORTED'
 ORDER BY version DESC
 LIMIT 1`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	file, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[File])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &file, nil
}

// BaseAndPatchFiles 读取任务下全部生效的 BASE / PATCH 文件（Manifest 组装用）。
func (s *Store) BaseAndPatchFiles(ctx context.Context, jobID int64) ([]File, error) {
	rows, err := s.pool.Query(ctx, `
SELECT `+fileColumns+`
  FROM `+tableFile+`
 WHERE job_id = $1 AND file_type IN ('BASE', 'PATCH') AND export_status = 'EXPORTED'
 ORDER BY file_type, version`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[File])
}

// SupersedeFile 把被覆盖的旧 base 标记为 SUPERSEDED（未上传的直接跳过上传）。
func (s *Store) SupersedeFile(ctx context.Context, fileID int64) error {
	_, err := s.pool.Exec(ctx, `
UPDATE `+tableFile+`
   SET export_status = 'SUPERSEDED',
       upload_status = CASE WHEN upload_status = 'UPLOADED' THEN 'UPLOADED' ELSE 'SKIPPED' END,
       gmt_update = now()
 WHERE file_id = $1`, fileID)
	return err
}

// SkipManifestUploads 把 MANIFEST 文件标记为 SKIPPED（按业务约定不上传）。
//
// 业务约定：只上传 .mvsv 数据文件（BASE / PATCH）；xx_..._Manifest.json 仅留在
// 本地用于校验，不推送到远端仓库。标记为 SKIPPED 而不是继续留在 PENDING_UPLOAD，
// 是为了让上传动态作业生成、本地清理与监控指标都能把 MANIFEST 视为已终结状态。
//
// 返回被跳过的文件描述（含尝试次数与目标文件名），供调用方写上传审计日志。
type SkippedManifest struct {
	FileID     int64
	Attempt    int
	TargetName string
}

func (s *Store) SkipManifestUploads(ctx context.Context) ([]SkippedManifest, error) {
	rows, err := s.pool.Query(ctx, `
UPDATE `+tableFile+`
   SET upload_status = 'SKIPPED',
       gmt_next_retry = NULL,
       gmt_update = now()
 WHERE file_type = 'MANIFEST'
   AND upload_status IN ('PENDING_UPLOAD', 'UPLOAD_FAILED')
RETURNING file_id, upload_attempt, target_file_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	skipped := make([]SkippedManifest, 0, 8)
	for rows.Next() {
		var item SkippedManifest
		if err := rows.Scan(&item.FileID, &item.Attempt, &item.TargetName); err != nil {
			return nil, err
		}
		skipped = append(skipped, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return skipped, nil
}

// PickUploadFile 领取一个待上传文件（行锁 + SKIP LOCKED）。
//
// 规则（G-08 / G-14 / 4.5）：只领取 PENDING_UPLOAD / UPLOAD_FAILED 且 gmt_next_retry 到期的记录；
// **仅上传 .mvsv 数据文件（BASE / PATCH）**——MANIFEST（xx_..._Manifest.json）
// 只留本地用于校验，不推送远端仓库，故此处直接排除（其状态由
// SkipManifestUploads 统一标记为 SKIPPED）。
func (s *Store) PickUploadFile(ctx context.Context) (*File, error) {
	rows, err := s.pool.Query(ctx, `
WITH picked AS (
  SELECT f.file_id
    FROM `+tableFile+` f
   WHERE f.upload_status IN ('PENDING_UPLOAD', 'UPLOAD_FAILED')
     AND (f.gmt_next_retry IS NULL OR f.gmt_next_retry <= now())
     AND f.export_status = 'EXPORTED'
     AND f.file_type <> 'MANIFEST'
   ORDER BY f.gmt_create, f.file_id
   LIMIT 1
   FOR UPDATE SKIP LOCKED
)
UPDATE `+tableFile+` f
   SET upload_status = 'UPLOADING',
       upload_attempt = f.upload_attempt + 1,
       gmt_upload_start = now(),
       gmt_update = now()
  FROM picked
 WHERE f.file_id = picked.file_id
RETURNING `+fileColumnsAliased)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	file, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[File])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &file, nil
}

// FinishUpload 落定上传结果。
func (s *Store) FinishUpload(ctx context.Context, fileID int64, status string, verified bool, targetPath *string, nextRetry *time.Time) error {
	_, err := s.pool.Exec(ctx, `
UPDATE `+tableFile+`
   SET upload_status = $2,
       remote_checksum_verified = $3,
       upload_target = COALESCE($4, upload_target),
       gmt_next_retry = $5,
       gmt_upload_finish = now(),
       gmt_update = now()
 WHERE file_id = $1`, fileID, status, verified, targetPath, nextRetry)
	return err
}

// InsertUploadLog 写一条上传尝试审计记录（G-13）。
func (s *Store) InsertUploadLog(ctx context.Context, fileID int64, attempt int, targetPath string, started time.Time, status string, uploadedBytes int64, errText string) error {
	var errArg *string
	if errText != "" {
		errArg = &errText
	}
	_, err := s.pool.Exec(ctx, `
INSERT INTO `+tableUploadLog+`
  (file_id, attempt, target_path, gmt_start, gmt_finish, status, uploaded_bytes, error)
VALUES ($1, $2, $3, $4, now(), $5, $6, $7)`,
		fileID, attempt, targetPath, started, status, uploadedBytes, errArg)
	return err
}

// ListFiles 分页查询文件登记。
func (s *Store) ListFiles(ctx context.Context, filter FileFilter) (Page[File], error) {
	conditions := make([]string, 0, 4)
	args := make([]any, 0, 6)
	if filter.UploadStatus != "" {
		args = append(args, filter.UploadStatus)
		conditions = append(conditions, fmt.Sprintf("upload_status = $%d", len(args)))
	}
	if filter.FileType != "" {
		args = append(args, filter.FileType)
		conditions = append(conditions, fmt.Sprintf("file_type = $%d", len(args)))
	}
	if filter.Usc != "" {
		args = append(args, filter.Usc)
		conditions = append(conditions, fmt.Sprintf("usc = $%d", len(args)))
	}
	if filter.BizMonth > 0 {
		args = append(args, filter.BizMonth)
		conditions = append(conditions, fmt.Sprintf("biz_month = $%d", len(args)))
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	var page Page[File]
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM `+tableFile+where, args...).Scan(&page.Total); err != nil {
		return page, err
	}
	if page.Total == 0 {
		page.List = []File{}
		return page, nil
	}

	pageSize := normalizePageSize(filter.PageSize)
	offset := (normalizePage(filter.Page) - 1) * pageSize
	args = append(args, pageSize, offset)
	rows, err := s.pool.Query(ctx, fmt.Sprintf(
		`SELECT %s FROM %s%s ORDER BY file_id DESC LIMIT $%d OFFSET $%d`,
		fileColumns, tableFile, where, len(args)-1, len(args)), args...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[File])
	if err != nil {
		return page, err
	}
	page.List = list
	return page, nil
}

// ResetFileForRetry 人工重试上传（仅 UPLOAD_FAILED 可重置）。
func (s *Store) ResetFileForRetry(ctx context.Context, fileID int64) error {
	tag, err := s.pool.Exec(ctx, `
UPDATE `+tableFile+`
   SET upload_status = 'PENDING_UPLOAD',
       gmt_next_retry = now(),
       upload_attempt = 0,
       gmt_update = now()
 WHERE file_id = $1 AND upload_status = 'UPLOAD_FAILED'`, fileID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("文件 %d 不存在或状态不是 UPLOAD_FAILED", fileID)
	}
	return nil
}

// RecoverTimeoutUploads 把超时未完成的 UPLOADING 记录回收为 UPLOAD_FAILED。
func (s *Store) RecoverTimeoutUploads(ctx context.Context, timeout time.Duration) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
UPDATE `+tableFile+`
   SET upload_status = 'UPLOAD_FAILED',
       gmt_next_retry = now(),
       gmt_upload_finish = now(),
       gmt_update = now()
 WHERE upload_status = 'UPLOADING'
   AND (gmt_upload_start IS NULL OR gmt_upload_start < now() - make_interval(secs => $1::double precision))`,
		timeout.Seconds())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// MarkNetworkArchived 标记文件已归档到网盘（由外部网盘 Job 或运维调用，L-02）。
func (s *Store) MarkNetworkArchived(ctx context.Context, fileID int64) error {
	_, err := s.pool.Exec(ctx, `
UPDATE `+tableFile+` SET network_status = 'ARCHIVED', gmt_update = now() WHERE file_id = $1`, fileID)
	return err
}

// MarkLocalDeleted 标记本地文件已清理。
func (s *Store) MarkLocalDeleted(ctx context.Context, fileID int64) error {
	_, err := s.pool.Exec(ctx, `
UPDATE `+tableFile+` SET cleanup_status = 'LOCAL_DELETED', gmt_update = now() WHERE file_id = $1`, fileID)
	return err
}

// LocalFileToPurge 待清理的本地导出文件（需求 10744）。
type LocalFileToPurge struct {
	FileID   int64
	Usc      string
	BizMonth int
	FileType string
	Version  int
	FilePath string
}

// LocalFilesForSettledMonths 返回「该证券月全部文件已交付且已过保留窗口」的本地文件。
//
// 判据（全部满足）：
//  1. 该 (usc, biz_month) 的 BASE 与全部 PATCH 均已 upload_status='UPLOADED'；
//  2. 上述 BASE/PATCH 的 remote_checksum_verified 全部为 true；
//  3. 最后一个文件的上传完成时间早于 now() - delay（默认 24 小时，留人工干预窗口）；
//  4. 该月没有 PENDING/RUNNING 状态的补丁记录；
//  5. 文件尚未标记 LOCAL_DELETED 且 file_path 非空。
//
// 返回值包含该月的 MANIFEST（.json），它不上传远端但同属本次导出产物，需要一起释放。
func (s *Store) LocalFilesForSettledMonths(ctx context.Context, delay time.Duration, limit int) ([]LocalFileToPurge, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := s.pool.Query(ctx, `
WITH settled AS (
    SELECT f.usc AS usc,
           f.biz_month AS biz_month,
           max(f.gmt_upload_finish) AS last_upload_at,
           bool_and(f.upload_status = 'UPLOADED') AS all_uploaded,
           bool_and(CASE WHEN f.file_type IN ('BASE','PATCH')
                         THEN f.remote_checksum_verified ELSE true END) AS all_verified,
           bool_or(f.file_type = 'BASE') AS has_base
      FROM `+tableFile+` f
     WHERE f.file_type IN ('BASE','PATCH')
     GROUP BY f.usc, f.biz_month
)
SELECT f.file_id, f.usc, f.biz_month, f.file_type, f.version, f.file_path
  FROM `+tableFile+` f
  JOIN settled s ON s.usc = f.usc AND s.biz_month = f.biz_month
 WHERE s.all_uploaded
   AND s.all_verified
   AND s.has_base
   AND s.last_upload_at IS NOT NULL
   AND s.last_upload_at < now() - make_interval(secs => $1::double precision)
   AND f.cleanup_status <> 'LOCAL_DELETED'
   AND f.file_path <> ''
   AND NOT EXISTS (
       SELECT 1 FROM `+tablePatch+` p
        WHERE p.usc = f.usc AND p.biz_month = f.biz_month
          AND p.status IN ('PENDING','RUNNING'))
 ORDER BY s.last_upload_at, f.file_id
 LIMIT $2`, delay.Seconds(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]LocalFileToPurge, 0, limit)
	for rows.Next() {
		var item LocalFileToPurge
		if err := rows.Scan(&item.FileID, &item.Usc, &item.BizMonth, &item.FileType,
			&item.Version, &item.FilePath); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// LocalCleanupCandidates 返回满足本地清理条件的文件：
// 上传成功 + 远端校验通过 + 网盘归档完成 + 无待处理补丁 + 保留期到达（L-02）。
//
// 例外：MANIFEST 按约定不上传（upload_status=SKIPPED），无需远端校验，
// 只要网盘归档完成且保留期到达即可清理，避免本地残留少量 json 永不释放。
func (s *Store) LocalCleanupCandidates(ctx context.Context, retention time.Duration, limit int) ([]File, error) {
	rows, err := s.pool.Query(ctx, `
SELECT `+fileColumns+`
  FROM `+tableFile+` f
 WHERE f.cleanup_status = 'NONE'
   AND (
        (f.upload_status = 'UPLOADED' AND f.remote_checksum_verified)
        OR (f.file_type = 'MANIFEST' AND f.upload_status IN ('UPLOADED', 'SKIPPED'))
       )
   AND f.network_status = 'ARCHIVED'
   AND f.gmt_create < now() - make_interval(secs => $1::double precision)
   AND NOT EXISTS (
       SELECT 1 FROM `+tablePatch+` p
        WHERE p.usc = f.usc AND p.biz_month = f.biz_month
          AND p.status IN ('PENDING', 'RUNNING'))
 ORDER BY f.file_id
 LIMIT $2`, retention.Seconds(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[File])
}

// PurgeMainCandidates 返回可清理主表旧数据的月份（全链路成功确认，L-04）。
func (s *Store) PurgeMainCandidates(ctx context.Context, beforeBizMonth int) ([]int, error) {
	rows, err := s.pool.Query(ctx, `
SELECT j.biz_month
  FROM `+tableJob+` j
 WHERE j.biz_month < $1
 GROUP BY j.biz_month
HAVING bool_and(
    j.status = 'EXPORTED'
    AND EXISTS (
        SELECT 1 FROM `+tableFile+` b
         WHERE b.job_id = j.job_id AND b.file_type = 'BASE'
           AND b.upload_status = 'UPLOADED' AND b.remote_checksum_verified
           AND b.network_status = 'ARCHIVED' AND b.cleanup_status = 'LOCAL_DELETED')
    AND NOT EXISTS (
        SELECT 1 FROM `+tablePatch+` p
         WHERE p.job_id = j.job_id AND p.status IN ('PENDING', 'RUNNING')))`, beforeBizMonth)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	months := make([]int, 0, 8)
	for rows.Next() {
		var month int
		if err := rows.Scan(&month); err != nil {
			return nil, err
		}
		months = append(months, month)
	}
	return months, rows.Err()
}

// PurgeMainMonth 按批删除主表某月数据，返回删除行数。
func (s *Store) PurgeMainMonth(ctx context.Context, bizMonth int, limit int) (int64, error) {
	from, to := MonthRange(bizMonth)
	tag, err := s.pool.Exec(ctx, `
DELETE FROM `+tableMain+`
 WHERE ctid IN (
   SELECT ctid FROM `+tableMain+`
    WHERE date BETWEEN $1 AND $2
    LIMIT $3)`, from, to, limit)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// PurgeMainSecurityMonth 删除指定证券某月的全部主表行情数据（20007=1 上传成功后调用）。
//
// 只删 dedup schema 的主表；quote schema 是回测/查询专用的独立副本，不参与本操作。
// 分批删除（ctid 子查询 + LIMIT），避免长事务与大量 WAL。
func (s *Store) PurgeMainSecurityMonth(ctx context.Context, usc string, bizMonth, limit int) (int64, error) {
	if limit <= 0 {
		limit = 50000
	}
	from, to := MonthRange(bizMonth)
	tag, err := s.pool.Exec(ctx, `
DELETE FROM `+tableMain+`
 WHERE ctid IN (
   SELECT ctid FROM `+tableMain+`
    WHERE usc = $1 AND date BETWEEN $2 AND $3
    LIMIT $4)`, usc, from, to, limit)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// SettledSecurityMonth 一个「已沉淀」的证券月：已导出并已上传，且导出后数据未再变化。
type SettledSecurityMonth struct {
	Usc      string
	BizMonth int
	FileID   int64
}

// SettledSecurityMonths 返回可按 20007 释放主表数据的证券月列表。
//
// 判定条件（全部满足）：
//  1. 存在 BASE 文件且 export_status='EXPORTED'、upload_status='UPLOADED'；
//  2. 主表该月仍有数据；
//  3. BASE 文件生成之后，该月主表没有任何新增/更新
//     （gmt_create > BASE.gmt_create 或 gmt_update > BASE.gmt_create）。
//
// 条件 3 用于避免误删：只要有行在文件生成后被写入或修改，就说明远端 BASE 不包含
// 这部分内容，必须先走 INCR 补丁流程，不能直接删除。
func (s *Store) SettledSecurityMonths(ctx context.Context, limit int) ([]SettledSecurityMonth, error) {
	if limit <= 0 {
		limit = 500
	}
	// 判据（需求 10720）：该证券月的 **BASE 与全部 PATCH** 都已 UPLOADED，
	// 且主表仍留有该月数据、且数据在最后一个文件生成后没有再次变化。
	//
	// 相比旧实现（只看 BASE）多了一层「全部 PATCH 已上传」约束，避免把尚未
	// 交付增量补丁的月份当作已沉淀而提前删除。
	rows, err := s.pool.Query(ctx, `
WITH settled AS (
    SELECT f.usc AS usc,
           f.biz_month AS biz_month,
           min(f.file_id) AS base_file_id,
           max(f.gmt_create) AS last_file_at,
           bool_and(f.upload_status = 'UPLOADED') AS all_uploaded,
           bool_or(f.file_type = 'BASE') AS has_base
      FROM `+tableFile+` f
     WHERE f.export_status = 'EXPORTED'
       AND f.file_type IN ('BASE','PATCH')
     GROUP BY f.usc, f.biz_month
)
SELECT s.base_file_id, s.usc, s.biz_month
  FROM settled s
 WHERE s.all_uploaded
   AND s.has_base
   AND EXISTS (
       SELECT 1 FROM `+tableMain+` m
        WHERE m.usc = s.usc
          AND m.date BETWEEN s.biz_month * 100 + 1 AND s.biz_month * 100 + 31)
   AND NOT EXISTS (
       SELECT 1 FROM `+tableMain+` m
        WHERE m.usc = s.usc
          AND m.date BETWEEN s.biz_month * 100 + 1 AND s.biz_month * 100 + 31
          AND (m.gmt_create > s.last_file_at OR m.gmt_update > s.last_file_at))
 ORDER BY s.biz_month, s.usc
 LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]SettledSecurityMonth, 0, limit)
	for rows.Next() {
		var item SettledSecurityMonth
		if err := rows.Scan(&item.FileID, &item.Usc, &item.BizMonth); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// PatchCandidates 返回需要检查补丁的已导出证券月。
func (s *Store) PatchCandidates(ctx context.Context, batchNo string, limit int) ([]PatchCandidate, error) {
	if batchNo != "" {
		rows, err := s.pool.Query(ctx, `
SELECT j.job_id, j.usc, j.biz_month, j.seal_date, j.base_version, j.patch_count
  FROM `+tableBatch+` b
  JOIN `+tableJob+` j ON j.job_id = b.source_job_id
 WHERE b.batch_no = $1 AND b.export_type = 'INCR'
   AND j.status = 'EXPORTED'
   AND EXISTS (
       SELECT 1 FROM `+tableFile+` f
        WHERE f.job_id = j.job_id AND f.file_type = 'BASE' AND f.export_status = 'EXPORTED')
 ORDER BY j.gmt_update, j.job_id
 LIMIT $2`, batchNo, limit)
		if err != nil {
			return nil, err
		}
		return scanPatchCandidates(rows)
	}
	rows, err := s.pool.Query(ctx, `
SELECT j.job_id, j.usc, j.biz_month, j.seal_date, j.base_version, j.patch_count
  FROM `+tableJob+` j
 WHERE j.status = 'EXPORTED'
   AND EXISTS (
       SELECT 1 FROM `+tableFile+` f
        WHERE f.job_id = j.job_id AND f.file_type = 'BASE' AND f.export_status = 'EXPORTED')
 ORDER BY j.gmt_update, j.job_id
 LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	return scanPatchCandidates(rows)
}

func scanPatchCandidates(rows pgx.Rows) ([]PatchCandidate, error) {
	defer rows.Close()
	candidates := make([]PatchCandidate, 0, 16)
	for rows.Next() {
		var item PatchCandidate
		if err := rows.Scan(&item.JobID, &item.Usc, &item.BizMonth, &item.SealDate,
			&item.BaseVersion, &item.PatchCount); err != nil {
			return nil, err
		}
		candidates = append(candidates, item)
	}
	return candidates, rows.Err()
}

// NextPatchVersion 返回下一个补丁序号 n（对应 ..._Incr_{n}.mvsv）。
func (s *Store) NextPatchVersion(ctx context.Context, jobID int64) (int, error) {
	var version int
	err := s.pool.QueryRow(ctx, `
SELECT COALESCE(max(patch_version), 0) + 1
  FROM `+tablePatch+` WHERE job_id = $1`, jobID).Scan(&version)
	return version, err
}

// InsertPatchTask 登记补丁任务。
func (s *Store) InsertPatchTask(ctx context.Context, task *PatchTask) (int64, error) {
	var patchFileID *int64
	if task.PatchFileID != nil {
		patchFileID = task.PatchFileID
	}
	var patchID int64
	err := s.pool.QueryRow(ctx, `
INSERT INTO `+tablePatch+`
  (job_id, usc, biz_month, patch_level, patch_version, source_rows, patch_rows, reason,
   arrival_window, status, patch_file_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, tstzrange($9, $10), $11, $12)
RETURNING patch_id`,
		task.JobID, task.Usc, task.BizMonth, task.PatchLevel, task.PatchVersion,
		task.SourceRows, task.PatchRows, task.Reason, task.ArrivalStart, task.ArrivalEnd,
		task.Status, patchFileID).Scan(&patchID)
	return patchID, err
}

// FinishPatchTask 落定补丁任务结果。
func (s *Store) FinishPatchTask(ctx context.Context, patchID int64, status string, patchRows int64, patchFileID *int64, errText string) error {
	var errArg *string
	if errText != "" {
		errArg = &errText
	}
	_, err := s.pool.Exec(ctx, `
UPDATE `+tablePatch+`
   SET status = $2,
       patch_rows = $3,
       patch_file_id = COALESCE($4, patch_file_id),
       gmt_finish = now(),
       last_error = $5
 WHERE patch_id = $1`, patchID, status, patchRows, patchFileID, errArg)
	return err
}

// UscInfo 读取证券主数据；不存在时返回 nil。
func (s *Store) UscInfo(ctx context.Context, usc string) (*UscInfo, error) {
	var info UscInfo
	err := s.pool.QueryRow(ctx, `
SELECT usc, category, day_expect_rows, date_list, date_delist, flag_auto_discover,
       region, market, provider, name, timezone, currency
  FROM `+tableUsc+` WHERE usc = $1`, usc).
		Scan(&info.Usc, &info.Category, &info.DayExpectRows, &info.DateList, &info.DateDelist,
			&info.FlagAutoDiscover, &info.Region, &info.Market, &info.Provider,
			&info.Name, &info.Timezone, &info.Currency)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// ---------------------------------------------------------------------
// 健康指标
// ---------------------------------------------------------------------

// StatusCounts 按状态聚合任务数。
func (s *Store) JobStatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.countBy(ctx, `SELECT status, count(*) FROM `+tableJob+` GROUP BY status`)
}

// JobStatusCountsByMonth 按业务月份聚合任务状态（用于导出任务批次的完成进度）。
func (s *Store) JobStatusCountsByMonth(ctx context.Context, bizMonth int) (map[string]int64, error) {
	return s.countBy(ctx, `SELECT status, count(*) FROM `+tableJob+` WHERE biz_month = $1 GROUP BY status`, bizMonth)
}

// PatchStatusCountsByMonth 按业务月份聚合补丁任务状态（补丁属于本月导出批次的一部分）。
func (s *Store) PatchStatusCountsByMonth(ctx context.Context, bizMonth int) (map[string]int64, error) {
	return s.countBy(ctx, `SELECT status, count(*) FROM `+tablePatch+` WHERE biz_month = $1 GROUP BY status`, bizMonth)
}

// UploadStatusCounts 按上传状态聚合文件数。
func (s *Store) UploadStatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.countBy(ctx, `SELECT upload_status, count(*) FROM `+tableFile+` GROUP BY upload_status`)
}

func (s *Store) countBy(ctx context.Context, sql string, args ...any) (map[string]int64, error) {
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int64{}
	for rows.Next() {
		var key string
		var count int64
		if err := rows.Scan(&key, &count); err != nil {
			return nil, err
		}
		counts[key] = count
	}
	return counts, rows.Err()
}

// HeartbeatTimeoutCount 统计心跳超时的 RUNNING 任务数。
func (s *Store) HeartbeatTimeoutCount(ctx context.Context) (int64, error) {
	var count int64
	err := s.pool.QueryRow(ctx, `
SELECT count(*) FROM `+tableJob+`
 WHERE status IN ('LOADING', 'LOADED', 'EXPORTING')
   AND (locked_until IS NULL OR locked_until < now())`).Scan(&count)
	return count, err
}

// UploadRetryExhaustedCount 统计重试耗尽的文件数（G-12 告警）。
func (s *Store) UploadRetryExhaustedCount(ctx context.Context, maxAttempts int) (int64, error) {
	var count int64
	err := s.pool.QueryRow(ctx, `
SELECT count(*) FROM `+tableFile+`
 WHERE upload_status = 'UPLOAD_FAILED' AND upload_attempt >= $1`, maxAttempts).Scan(&count)
	return count, err
}

// PatchStats 返回近 7 天补丁总数与「同证券月补丁 > 3 次」的分组数（M-07）。
func (s *Store) PatchStats(ctx context.Context) (int64, int64, error) {
	var total, groups int64
	if err := s.pool.QueryRow(ctx, `
SELECT count(*) FROM `+tablePatch+`
 WHERE gmt_create >= now() - make_interval(days => 7)`).Scan(&total); err != nil {
		return 0, 0, err
	}
	if err := s.pool.QueryRow(ctx, `
SELECT count(*) FROM (
  SELECT job_id FROM `+tablePatch+`
   WHERE gmt_create >= now() - make_interval(days => 7)
   GROUP BY job_id
  HAVING count(*) > 3) t`).Scan(&groups); err != nil {
		return total, 0, err
	}
	return total, groups, nil
}

// LocalFilesPendingCleanup 统计满足清理条件但尚未清理的文件数。
func (s *Store) LocalFilesPendingCleanup(ctx context.Context) (int64, error) {
	var count int64
	err := s.pool.QueryRow(ctx, `
SELECT count(*) FROM `+tableFile+`
 WHERE cleanup_status = 'NONE'
   AND (
        (upload_status = 'UPLOADED' AND remote_checksum_verified)
        OR (file_type = 'MANIFEST' AND upload_status IN ('UPLOADED', 'SKIPPED'))
       )
   AND network_status = 'ARCHIVED'`).Scan(&count)
	return count, err
}

// FrozenMissing 统计 FROZEN 证券月的缺失行数与预期行数。
func (s *Store) FrozenMissing(ctx context.Context, minBizMonth int) (int64, int64, error) {
	var missing, expected int64
	err := s.pool.QueryRow(ctx, `
SELECT COALESCE(sum(GREATEST(expected_rows - actual_rows, 0)), 0)::bigint,
       COALESCE(sum(expected_rows), 0)::bigint
  FROM `+tableMaturity+`
 WHERE maturity_status = 'FROZEN' AND biz_month >= $1`, minBizMonth).Scan(&missing, &expected)
	return missing, expected, err
}

// RecentFilesForVerify 返回最近登记的数据文件（BASE / PATCH），用于 MVsv 结构抽检。
func (s *Store) RecentFilesForVerify(ctx context.Context, limit int) ([]File, error) {
	rows, err := s.pool.Query(ctx, `
SELECT `+fileColumns+`
  FROM `+tableFile+`
 WHERE file_type IN ('BASE', 'PATCH') AND export_status = 'EXPORTED'
 ORDER BY file_id DESC
 LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[File])
}
