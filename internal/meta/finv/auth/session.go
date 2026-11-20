package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// ContextSessionKey 登录会话在 HTTP 上下文中的键。
//
// 放在 auth 包是为了让 api（中间件）与 handler（处理器）共用同一字符串，
// 同时避免 handler 反向依赖 api 包造成循环引用。
const ContextSessionKey = "finv.auth.session"

// sessionKeyPrefix Redis 会话键前缀。
const sessionKeyPrefix = "finv:auth:session:"

// userSessionsKeyPrefix 用户会话索引键前缀（Set，保存该用户的全部令牌）。
//
// 有了这个索引才能实现「改密后踢掉该用户在别处的登录」：会话键是
// token → 会话，无法按 userID 反查，必须额外维护反向索引。
const userSessionsKeyPrefix = "finv:auth:user:"

// tokenBytes 会话令牌随机字节数（hex 后 64 字符）。
const tokenBytes = 32

// ErrSessionNotFound 表示会话不存在、已过期或已被吊销。
var ErrSessionNotFound = errors.New("会话不存在或已过期")

// Session 登录会话。令牌本身不承载业务信息，仅作 Redis 键；
// 用户身份以 Redis 中的记录为准，便于服务端即时吊销。
type Session struct {
	Token     string    `json:"token"`
	UserID    int64     `json:"user_id"`
	UID       string    `json:"uid"`
	NickName  string    `json:"nick_name"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// DefaultTTL 会话有效期兜底值（20012 缺失或非法时使用）。
const DefaultTTL = 8 * time.Hour

// SessionStore 基于 Redis 的会话存储。
//
// 有效期由调用方按次传入（取自 sys.sys_config 20012），因此在线修改配置
// 无需重启或重建存储即可生效。
type SessionStore struct {
	rdb *redis.Client
}

// NewSessionStore 创建会话存储。
func NewSessionStore(rdb *redis.Client) *SessionStore {
	return &SessionStore{rdb: rdb}
}

// Create 生成新会话令牌并写入 Redis；ttl <= 0 时回落到 DefaultTTL。
func (s *SessionStore) Create(ctx context.Context, userID int64, uid, nickName string, ttl time.Duration) (*Session, error) {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("生成会话令牌失败: %w", err)
	}
	now := time.Now()
	session := &Session{
		Token:     hex.EncodeToString(buf),
		UserID:    userID,
		UID:       uid,
		NickName:  nickName,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}
	payload, err := json.Marshal(session)
	if err != nil {
		return nil, fmt.Errorf("序列化会话失败: %w", err)
	}
	if err := s.rdb.Set(ctx, sessionKeyPrefix+session.Token, payload, ttl).Err(); err != nil {
		return nil, fmt.Errorf("写入会话失败: %w", err)
	}
	// 维护用户会话索引，供改密时批量吊销；索引不设 TTL，
	// 过期残留由 Revoke 与 RevokeOthers 顺带清理，不会无限增长。
	if err := s.rdb.SAdd(ctx, userSessionsKey(userID), session.Token).Err(); err != nil {
		return nil, fmt.Errorf("写入会话索引失败: %w", err)
	}
	return session, nil
}

// Resolve 校验令牌并返回会话；不存在或过期返回 ErrSessionNotFound。
func (s *SessionStore) Resolve(ctx context.Context, token string) (*Session, error) {
	if token == "" {
		return nil, ErrSessionNotFound
	}
	raw, err := s.rdb.Get(ctx, sessionKeyPrefix+token).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("读取会话失败: %w", err)
	}
	var session Session
	if err := json.Unmarshal(raw, &session); err != nil {
		// 存储内容损坏视为无效会话，按未登录处理
		return nil, ErrSessionNotFound
	}
	if !time.Now().Before(session.ExpiresAt) {
		return nil, ErrSessionNotFound
	}
	return &session, nil
}

// Revoke 吊销单个会话并清理其索引项。
func (s *SessionStore) Revoke(ctx context.Context, session *Session) error {
	if session == nil || session.Token == "" {
		return nil
	}
	if err := s.rdb.Del(ctx, sessionKeyPrefix+session.Token).Err(); err != nil {
		return fmt.Errorf("吊销会话失败: %w", err)
	}
	_ = s.rdb.SRem(ctx, userSessionsKey(session.UserID), session.Token).Err()
	return nil
}

// RevokeOthers 吊销同一用户除 keepToken 外的全部会话，返回实际吊销数量。
//
// 用于修改口令后让其它设备/标签页的登录立即失效：口令已变而旧会话仍有效
// 是典型的安全缺口。当前会话保留，避免用户自己刚改完密码就被踢出。
func (s *SessionStore) RevokeOthers(ctx context.Context, userID int64, keepToken string) (int, error) {
	if userID == 0 {
		return 0, nil
	}
	key := userSessionsKey(userID)
	tokens, err := s.rdb.SMembers(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("读取用户会话索引失败: %w", err)
	}

	removed := 0
	for _, token := range tokens {
		if token == keepToken {
			continue
		}
		deleted, err := s.rdb.Del(ctx, sessionKeyPrefix+token).Result()
		if err != nil {
			// 单条失败不阻断其余会话的吊销
			continue
		}
		if deleted > 0 {
			removed++
		}
		// 无论会话键是否存在都从索引移除：过期的残留也一并清掉
		_ = s.rdb.SRem(ctx, key, token).Err()
	}
	return removed, nil
}

// userSessionsKey 用户会话索引键。
func userSessionsKey(userID int64) string {
	return userSessionsKeyPrefix + strconv.FormatInt(userID, 10)
}

// Touch 滑动续期：仅在剩余时间不足一半时延长，避免每次请求都写 Redis。
//
// 续期长度取该会话创建时的有效期（ExpiresAt - CreatedAt），
// 因此运行期修改 20012 不会改变已签发会话的续期时长。
func (s *SessionStore) Touch(ctx context.Context, session *Session) {
	if session == nil || session.Token == "" {
		return
	}
	ttl := session.ExpiresAt.Sub(session.CreatedAt)
	if ttl <= 0 {
		return
	}
	if time.Until(session.ExpiresAt) > ttl/2 {
		return
	}
	now := time.Now()
	session.CreatedAt = now
	session.ExpiresAt = now.Add(ttl)
	payload, err := json.Marshal(session)
	if err != nil {
		return
	}
	_ = s.rdb.Set(ctx, sessionKeyPrefix+session.Token, payload, ttl).Err()
}
