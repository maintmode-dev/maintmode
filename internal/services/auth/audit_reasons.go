package auth

// Whitelist-safe причины отказа логина для аудит-метаданных. Сырой текст
// ошибки в аудит не пишется — он может содержать внутренние детали (RUK-81).
const (
	auditFailureUserProvisioning = "user provisioning failed"
	//nolint:gosec // G101 false positive: человекочитаемая причина отказа, не credential
	auditFailureTokenIssuance = "token issuance failed"
)
