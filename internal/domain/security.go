package domain

// OTPPurpose identifies what an OTP code was issued for.
// This prevents an OTP from one flow being replayed in another.
type OTPPurpose string

const (
	OTPPurposePhoneVerify   OTPPurpose = "phone_verify"
	OTPPurposeLogin         OTPPurpose = "login"
	OTPPurposeAction        OTPPurpose = "action"
	OTPPurposeResetPassword OTPPurpose = "reset_password"
)

// ActionType identifies which sensitive action an action-token authorises.
type ActionType string

const (
	ActionWithdraw       ActionType = "withdraw"
	ActionChangePassword ActionType = "change_password"
	ActionResetPassword  ActionType = "reset_password"
)
