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
	ID          uuid.UUID
	DisplayName string
	Email       string
	Role        UserRole
	IsActive    bool
	LastLoginAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (u User) IsAdmin() bool {
	return u.Role == UserRoleAdmin
}

func (u User) CanUseSystem() bool {
	return u.IsActive
}
