package executor

// Test-only re-exports of unexported functions.
// This file is compiled only during `go test`.

var (
	HandleWhenExpression       = handleWhenExpression
	CheckIdempotencyConditions = checkIdempotencyConditions
	CheckSkipConditions        = checkSkipConditions
	GetStepDisplayName         = getStepDisplayName
)
