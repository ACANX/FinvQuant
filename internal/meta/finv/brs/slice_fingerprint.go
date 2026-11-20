// 分片内容指纹：用于判断「已备份的分片是否需要重做」。
//
// 为什么需要：原先对末尾 N 个分片无条件重做（redo_slices），简单但粗暴——
// 数据没变也要重新导出、压缩、上传一遍；反过来，窗口外的分片即使数据变了
// 也永远跳过。正确做法是**比较内容差异**：
//
//	源端指纹 = (行数, 全行文本哈希和)           ← 每轮现算，一次范围扫描
//	已备份指纹 = 上次备份成功时写入的 sidecar    ← 本地 <分片文件>.fp / 远端 <key>.fp
//
// 两者相同 → 跳过（不下发导出、不上传）；不同 → 重做并覆盖指纹。
//
// 指纹取 "行数 + sum(hashtextextended(行文本, 0))"：
//   - 与行的物理顺序无关（sum 可交换），不受 VACUUM/重排影响；
//   - hashtextextended 是 64 位哈希，碰撞概率可忽略；
//   - 只读不写，成本约为一次分片范围扫描，远低于「导出 + gzip + 上传」。
package brs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acanx/finvquant/internal/meta/base/oss"
)

// sliceFingerprint 分片内容指纹。
type sliceFingerprint struct {
	Rows int64  `json:"rows"` // 行数
	Hash string `json:"hash"` // 全行哈希和（十进制字符串，避免 JS 精度问题）
	Gmt  string `json:"gmt"`  // 计算时间（UTC RFC3339，仅用于排查）
}

// Equal 判断两个指纹是否表示同一份内容。
func (f sliceFingerprint) Equal(other sliceFingerprint) bool {
	return f.Rows == other.Rows && f.Hash == other.Hash
}

// String 便于日志输出。
func (f sliceFingerprint) String() string {
	return fmt.Sprintf("%d 行 / %s", f.Rows, shortHash(f.Hash))
}

// shortHash 截断哈希便于日志阅读。
func shortHash(hash string) string {
	if len(hash) <= 12 {
		return hash
	}
	return hash[:12] + "…"
}

// computeSliceFingerprint 计算源端分片指纹（一次范围扫描）。
//
// SQL 里用 hashtextextended(t::text, 0) 对整行文本取 64 位哈希后求和；
// 注意 sum(bigint) 在 PG 里返回 numeric，这里显式转 text 再解析，
// 避免 int8 溢出后被 pgx 扫描成浮点。
func computeSliceFingerprint(ctx context.Context, pool *pgxpool.Pool, rule SliceRule, slice Slice) (sliceFingerprint, error) {
	where, err := rule.whereClause(slice)
	if err != nil {
		return sliceFingerprint{}, err
	}
	query := fmt.Sprintf(`SELECT count(*)::text,
       COALESCE(sum(hashtextextended(t::text, 0)), 0)::text
  FROM %s.%s t
 WHERE %s`, rule.SchemaName, rule.TableName, where)

	var rowsText, hashText string
	if err := pool.QueryRow(ctx, query).Scan(&rowsText, &hashText); err != nil {
		return sliceFingerprint{}, fmt.Errorf("计算分片指纹失败: %w", err)
	}
	var rows int64
	if _, err := fmt.Sscan(rowsText, &rows); err != nil {
		return sliceFingerprint{}, fmt.Errorf("解析分片行数失败: %w", err)
	}
	// numeric 可能带 ".000" 之类的小数尾巴，去掉小数点后部分。
	hash := strings.TrimSpace(hashText)
	if idx := strings.IndexByte(hash, '.'); idx >= 0 {
		hash = hash[:idx]
	}
	if hash == "" {
		hash = "0"
	}
	return sliceFingerprint{
		Rows: rows,
		Hash: hash,
		Gmt:  time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// fingerprintPath 本地产物的指纹 sidecar 路径：<分片文件>.fp。
func (r SliceRule) fingerprintPath(backupDir string, s Slice) string {
	return r.localPath(backupDir, s) + ".fp"
}

// remoteFingerprintKey 远端指纹 sidecar 的 key：<分片 key>.fp。
func (r SliceRule) remoteFingerprintKey(env string, s Slice) string {
	return r.remoteKey(env, s) + ".fp"
}

// loadLocalFingerprint 读取本地指纹；不存在或损坏返回 ok=false。
func loadLocalFingerprint(path string) (sliceFingerprint, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return sliceFingerprint{}, false
	}
	var fp sliceFingerprint
	if err := json.Unmarshal(data, &fp); err != nil || fp.Hash == "" {
		return sliceFingerprint{}, false
	}
	return fp, true
}

// writeLocalFingerprint 写入本地指纹（原子改名）。
func writeLocalFingerprint(path string, fp sliceFingerprint) error {
	data, err := json.Marshal(fp)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// loadRemoteFingerprint 读取远端指纹 sidecar；不存在或损坏返回 ok=false。
func loadRemoteFingerprint(ctx context.Context, store oss.Store, key string) (sliceFingerprint, bool) {
	if store == nil || key == "" {
		return sliceFingerprint{}, false
	}
	body, err := store.Get(ctx, key)
	if err != nil {
		return sliceFingerprint{}, false
	}
	defer body.Close()
	data, err := io.ReadAll(io.LimitReader(body, 4096))
	if err != nil {
		return sliceFingerprint{}, false
	}
	var fp sliceFingerprint
	if err := json.Unmarshal(data, &fp); err != nil || fp.Hash == "" {
		return sliceFingerprint{}, false
	}
	return fp, true
}

// saveFingerprints 备份成功后写入本地与远端指纹（远端失败只告警，不影响分片结果）。
func saveFingerprints(ctx context.Context, store oss.Store, rule SliceRule, opts Options, env string, s Slice, fp sliceFingerprint, hooks Hooks) {
	if err := writeLocalFingerprint(rule.fingerprintPath(opts.BackupDir, s), fp); err != nil {
		hooks.emit("warn", "写入本地指纹失败 %s 分片 %s: %v", rule.Table(), s.Key, err)
	}
	if store == nil || rule.FlagUpload != "1" || env == "" {
		return
	}
	data, err := json.Marshal(fp)
	if err != nil {
		hooks.emit("warn", "序列化指纹失败 %s 分片 %s: %v", rule.Table(), s.Key, err)
		return
	}
	key := rule.remoteFingerprintKey(env, s)
	if err := store.Put(ctx, key, strings.NewReader(string(data)), int64(len(data)), "application/json"); err != nil {
		hooks.emit("warn", "上传远端指纹失败 %s 分片 %s: %v（只影响下一轮的差异判断）", rule.Table(), s.Key, err)
	}
}
