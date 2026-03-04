package domain

import (
	"time"
)

type UserRole string

const (
	UserRoleClient        UserRole = "client"
	UserRoleAdmin         UserRole = "admin"
	UserRoleSuperAdmin    UserRole = "super_admin"
	UserRoleOperator      UserRole = "operator"
	UserRoleSupport       UserRole = "support"
	UserRoleAMLSpecialist UserRole = "aml_specialist"
	UserRoleCompliance    UserRole = "compliance"
)

func (r UserRole) IsStaffRole() bool {
	return r != UserRoleClient
}

func (r UserRole) IsValidRole() bool {
	switch r {
	case UserRoleClient, UserRoleAdmin, UserRoleSuperAdmin,
		UserRoleOperator, UserRoleSupport, UserRoleAMLSpecialist, UserRoleCompliance:
		return true
	}
	return false
}

type AuthMethod string

const (
	AuthMethodRegular AuthMethod = "regular"
	AuthMethodGoogle  AuthMethod = "google"
)

type User struct {
	ID           int64      `db:"id" json:"id"`
	Email        string     `db:"email" json:"email"`
	PasswordHash string     `db:"password_hash" json:"-"`
	FirstName    string     `db:"first_name" json:"first_name"`
	LastName     string     `db:"last_name" json:"last_name"`
	MiddleName   *string    `db:"middle_name" json:"middle_name"`
	Phone        *string    `db:"phone" json:"phone"`
	KycLevel     int        `db:"kyc_level" json:"kyc_level"`
	Role         UserRole   `db:"role" json:"role"`
	IsActive     bool       `db:"is_active" json:"is_active"`
	IsVerified   bool       `db:"is_verified" json:"is_verified"`
	AuthMethod   AuthMethod `db:"auth_method" json:"auth_method"`
	GoogleID     *string    `db:"google_id" json:"-"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at" json:"updated_at"`
}

type UserSession struct {
	ID           int64     `db:"id" json:"id"`
	UserID       int64     `db:"user_id" json:"user_id"`
	RefreshToken string    `db:"refresh_token" json:"-"`
	ExpiresAt    time.Time `db:"expires_at" json:"expires_at"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
}
