package windows_firewall_rule

import "encoding/base64"

// base64Enc is the package-local handle for std base64 — exposed so
// handler.go's runPS can call EncodeToString without a top-of-file
// import that breaks the cmd-build during the spec-56 transitional
// period.
var base64Enc = base64.StdEncoding
