package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// tableUser 系统用户表（全限定，符合 DbSchemaSpec 的 schema 限定要求）。
const tableUser = "sys.sys_user"

// ErrUserNotFound 表示按 uid 未查到用户。
var ErrUserNotFound = errors.New("用户不存在")

// ErrUserDisabled 表示账号已被逻辑删除或停用。
var ErrUserDisabled = errors.New("账号已停用")

// User 认证所需的用户字段（不含任何明文凭据以外的敏感信息）。
type User struct {
	IDn          int64
	UIN          *int64
	UID          string
	NickName     string
	Email        string
	Password     string
	PasswordSalt string
	AuthorityStr string
	FlagDelete   string
	FlagEnable   bool
}

// findUserByUID 按 uid 查询未删除用户。
func findUserByUID(ctx context.Context, pool *pgxpool.Pool, uid string) (*User, error) {
	const query = `
		SELECT idn, uin, uid, nick_name, email,
		       password, password_salt, authority_str, flag_delete
		  FROM ` + tableUser + `
		 WHERE uid = $1
		 LIMIT 1`

	var user User
	err := pool.QueryRow(ctx, query, uid).Scan(
		&user.IDn, &user.UIN, &user.UID, &user.NickName, &user.Email,
		&user.Password, &user.PasswordSalt, &user.AuthorityStr, &user.FlagDelete,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("查询系统用户失败: %w", err)
	}
	user.FlagEnable = user.FlagDelete != "1"
	return &user, nil
}
