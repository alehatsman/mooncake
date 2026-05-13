package executor

// Test-only re-exports of unexported functions.
// This file is compiled only during `go test`.

var (
	MarkStepFailed           = markStepFailed
	HandleVars               = handleVars
	HandleWhenExpression     = handleWhenExpression
	ShouldSkipByTags         = shouldSkipByTags
	CheckIdempotencyConditions = checkIdempotencyConditions
	CheckSkipConditions      = checkSkipConditions
	GetStepDisplayName       = getStepDisplayName
	ParseFileMode            = parseFileMode
)
