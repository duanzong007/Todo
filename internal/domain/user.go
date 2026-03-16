package domain

import (
	"time"

	"github.com/google/uuid"
)

type UserRole string

const (
	UserRoleMember UserRole = "member"
	UserRoleAdmin  UserRole = "admin"
)

type User struct {
	ID           uuid.UUID
	Username     string
	DisplayName  string
	PasswordHash string
	Role         UserRole
	IsActive     bool
	LastLoginAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
