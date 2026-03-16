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

type UserApprovalStatus string

const (
	UserApprovalPending  UserApprovalStatus = "pending"
	UserApprovalApproved UserApprovalStatus = "approved"
)

type User struct {
	ID             uuid.UUID
	Username       string
	DisplayName    string
	PasswordHash   string
	Role           UserRole
	ApprovalStatus UserApprovalStatus
	IsActive       bool
	LastLoginAt    *time.Time
	ApprovedAt     *time.Time
	ApprovedBy     *uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (u User) IsAdmin() bool {
	return u.Role == UserRoleAdmin
}

func (u User) IsApproved() bool {
	return u.ApprovalStatus == UserApprovalApproved
}

func (u User) CanUseSystem() bool {
	return u.IsApproved() && u.IsActive
}
