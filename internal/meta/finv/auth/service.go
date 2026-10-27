package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/acanx/finvquant/internal/meta/finv/sys"
)

// ErrInvalidCredentials 表示用户名或密码错误。
//
// 对「用户不存在」与「密码不匹配」返回同一错误，避免向调用方泄露账号是否存在。
var ErrInvalidCredentials = errors.New("用户名或密码错误")

// dummyHash 在用户不存在时仍执行一次等价耗时的 argon2 计算，
// 使两条失败路径的响应时间接近，避免时序侧信道区分账号是否存在。
var dummyHash = func() string {
	const saltHex = "00000000000000000000000000000000"
	h, err := Hash("finvquant-timing-equalizer", saltHex)
	if err != nil {
		// 参数为常量，正常情况下不会失败；兜底为空串使 Verify 直接返回 false
		return ""
	}
	return h
}()

// Service 认证服务。
type Service struct {
	pool     *pgxpool.Pool
	sessions *SessionStore
	cfg      *syscfg.Store
}

// NewService 创建认证服务。
//
// cfg 为空时使用 DefaultTTL（8 小时）；非空时每次登录读取 20012，
// 使运维调整会话有效期无需重启进程。
func NewService(pool *pgxpool.Pool, rdb *redis.Client, cfg *syscfg.Store) *Service {
	return &Service{pool: pool, sessions: NewSessionStore(rdb), cfg: cfg}
}

// sessionTTL 读取 20012 得到登录会话有效期。
func (s *Service) sessionTTL(ctx context.Context) time.Duration {
	if s.cfg == nil {
		return DefaultTTL
	}
	values, err := s.cfg.Load(ctx)
	if err != nil || values.AuthSessionTTLMinutes <= 0 {
		return DefaultTTL
	}
	return time.Duration(values.AuthSessionTTLMinutes) * time.Minute
}

// Login 校验用户名密码并创建会话。
func (s *Service) Login(ctx context.Context, uid, password string) (*Session, error) {
	uid = strings.TrimSpace(uid)
	if uid == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	user, err := findUserByUID(ctx, s.pool, uid)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			// 与密码错误路径保持等价耗时
			Verify(password, dummyHash)
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if !user.FlagEnable {
		return nil, ErrUserDisabled
	}
	if !Verify(password, user.Password) {
		return nil, ErrInvalidCredentials
	}

	session, err := s.sessions.Create(ctx, user.IDn, user.UID, user.NickName, s.sessionTTL(ctx))
	if err != nil {
		return nil, fmt.Errorf("创建会话失败: %w", err)
	}
	return session, nil
}

// Logout 吊销会话。
func (s *Service) Logout(ctx context.Context, token string) error {
	return s.sessions.Revoke(ctx, token)
}

// Resolve 校验令牌并返回会话，必要时滑动续期。
func (s *Service) Resolve(ctx context.Context, token string) (*Session, error) {
	session, err := s.sessions.Resolve(ctx, token)
	if err != nil {
		return nil, err
	}
	s.sessions.Touch(ctx, session)
	return session, nil
}

// Profile 返回会话对应的用户资料（不含密码字段）。
func (s *Service) Profile(ctx context.Context, userID int64) (*User, error) {
	const query = `
		SELECT idn, uin, uid, nick_name, email, authority_str, flag_delete
		  FROM ` + tableUser + `
		 WHERE idn = $1
		 LIMIT 1`

	var user User
	err := s.pool.QueryRow(ctx, query, userID).Scan(
		&user.IDn, &user.UIN, &user.UID, &user.NickName, &user.Email,
		&user.AuthorityStr, &user.FlagDelete,
	)
	if err != nil {
		return nil, fmt.Errorf("查询用户资料失败: %w", err)
	}
	user.FlagEnable = user.FlagDelete != "1"
	return &user, nil
}
