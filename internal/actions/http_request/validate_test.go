package http_request

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

func TestValidate_TableDriven(t *testing.T) {
	cases := []struct {
		name      string
		step      *config.Step
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "nil step",
			step:    &config.Step{},
			wantErr: true,
		},
		{
			name:      "missing url",
			step:      &config.Step{HTTPRequest: &config.HTTPRequest{}},
			wantErr:   true,
			errSubstr: "url is required",
		},
		{
			name: "default GET is fine without idempotency",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{URL: "http://x"}},
		},
		{
			name: "explicit PUT is fine without idempotency (idempotent by spec)",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{URL: "http://x", Method: "PUT"}},
		},
		{
			name: "DELETE is fine without idempotency (idempotent by spec)",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{URL: "http://x", Method: "DELETE"}},
		},
		{
			name: "POST without idempotency is REJECTED",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL:    "http://x",
				Method: "POST",
				Body:   "{}",
			}},
			wantErr:   true,
			errSubstr: "non-idempotent",
		},
		{
			name: "PATCH without idempotency is REJECTED",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL:    "http://x",
				Method: "PATCH",
				Body:   "{}",
			}},
			wantErr:   true,
			errSubstr: "non-idempotent",
		},
		{
			name: "POST + idempotency_key OK",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL:            "http://x",
				Method:         "POST",
				IdempotencyKey: "deploy-1",
			}},
		},
		{
			name: "POST + creates_when OK",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL:         "http://x",
				Method:      "POST",
				CreatesWhen: "facts.hook == null",
			}},
		},
		{
			name: "POST + risk:high OK",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL:    "http://x",
				Method: "POST",
				Risk:   "high",
			}},
		},
		{
			name: "POST + risk:low is NOT a valid ack (only risk:high is)",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL:    "http://x",
				Method: "POST",
				Risk:   "low",
			}},
			wantErr:   true,
			errSubstr: "non-idempotent",
		},
		{
			name: "POST with TWO idempotency signals is REJECTED",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL:            "http://x",
				Method:         "POST",
				IdempotencyKey: "k",
				CreatesWhen:    "x",
			}},
			wantErr:   true,
			errSubstr: "exactly one",
		},
		{
			name: "body + json one-of",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL:  "http://x",
				Body: "abc",
				JSON: map[string]interface{}{"x": 1},
			}},
			wantErr:   true,
			errSubstr: "at most one of body/json/form/file",
		},
		{
			name: "auth bearer + basic one-of",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL: "http://x",
				Auth: &config.HTTPAuth{
					Bearer: "t",
					Basic:  &config.HTTPBasicAuth{User: "u", Pass: "p"},
				},
			}},
			wantErr:   true,
			errSubstr: "at most one of auth",
		},
		{
			name: "auth.header.name required",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL: "http://x",
				Auth: &config.HTTPAuth{
					Header: &config.HTTPAuthHeader{Value: "v"},
				},
			}},
			wantErr:   true,
			errSubstr: "auth.header.name",
		},
		{
			name: "auth.basic.user required",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL: "http://x",
				Auth: &config.HTTPAuth{
					Basic: &config.HTTPBasicAuth{Pass: "p"},
				},
			}},
			wantErr:   true,
			errSubstr: "auth.basic.user",
		},
		{
			name: "unknown retry_on token",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL:     "http://x",
				RetryOn: []string{"5xx", "frobnicate"},
			}},
			wantErr:   true,
			errSubstr: "unknown retry_on",
		},
		{
			name: "known retry_on tokens",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL:     "http://x",
				RetryOn: []string{"5xx", "4xx", "429", "timeout", "connection_error"},
			}},
		},
		{
			name: "bad timeout duration",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL:     "http://x",
				Timeout: "lol",
			}},
			wantErr:   true,
			errSubstr: "invalid timeout",
		},
		{
			name: "negative retries",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL:     "http://x",
				Retries: -1,
			}},
			wantErr:   true,
			errSubstr: "retries must be >= 0",
		},
		{
			name: "bad expect_status",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL:          "http://x",
				ExpectStatus: []int{99},
			}},
			wantErr:   true,
			errSubstr: "invalid expect_status",
		},
		{
			name: "bad risk value",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL:  "http://x",
				Risk: "yolo",
			}},
			wantErr:   true,
			errSubstr: "risk must be one of",
		},
		{
			name: "as_user rejected",
			step: &config.Step{
				AsUser:      "root",
				HTTPRequest: &config.HTTPRequest{URL: "http://x"},
			},
			wantErr:   true,
			errSubstr: "as_user is not supported",
		},
		{
			name: "unsupported method",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL:    "http://x",
				Method: "TRACE",
			}},
			wantErr:   true,
			errSubstr: "unsupported method",
		},

		// Wave 2 — probe / reverse.
		{
			name: "probe GET is fine",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL:         "http://x",
				CreatesWhen: "probe.status_code == 404",
				Probe:       &config.HTTPRequest{URL: "http://x", Method: "GET"},
			}},
		},
		{
			name: "probe POST is REJECTED",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL:   "http://x",
				Probe: &config.HTTPRequest{URL: "http://x", Method: "POST"},
			}},
			wantErr:   true,
			errSubstr: "probe method must be GET/HEAD/OPTIONS",
		},
		{
			name: "probe.url required",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL:   "http://x",
				Probe: &config.HTTPRequest{Method: "GET"},
			}},
			wantErr:   true,
			errSubstr: "probe.url",
		},
		{
			name: "probe must NOT nest probe",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL: "http://x",
				Probe: &config.HTTPRequest{
					URL:   "http://x",
					Probe: &config.HTTPRequest{URL: "http://x"},
				},
			}},
			wantErr:   true,
			errSubstr: "probe must not nest",
		},
		{
			name: "probe must NOT declare reverse",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL: "http://x",
				Probe: &config.HTTPRequest{
					URL:     "http://x",
					Reverse: &config.HTTPRequest{URL: "http://x", Method: "DELETE"},
				},
			}},
			wantErr:   true,
			errSubstr: "probe must not declare reverse",
		},
		{
			name: "reverse with DELETE is fine",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL:            "http://x",
				Method:         "POST",
				IdempotencyKey: "k",
				Reverse:        &config.HTTPRequest{URL: "http://x/1", Method: "DELETE"},
			}},
		},
		{
			name: "reverse must NOT nest reverse",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL:            "http://x",
				Method:         "POST",
				IdempotencyKey: "k",
				Reverse: &config.HTTPRequest{
					URL:     "http://x",
					Method:  "DELETE",
					Reverse: &config.HTTPRequest{URL: "http://x", Method: "DELETE"},
				},
			}},
			wantErr:   true,
			errSubstr: "reverse must not nest",
		},
		{
			name: "reverse must NOT declare probe",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL:            "http://x",
				Method:         "POST",
				IdempotencyKey: "k",
				Reverse: &config.HTTPRequest{
					URL:    "http://x",
					Method: "DELETE",
					Probe:  &config.HTTPRequest{URL: "http://x", Method: "GET"},
				},
			}},
			wantErr:   true,
			errSubstr: "reverse must not declare probe",
		},
		{
			name: "reverse: bad method",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL:            "http://x",
				Method:         "POST",
				IdempotencyKey: "k",
				Reverse:        &config.HTTPRequest{URL: "http://x", Method: "FROBNICATE"},
			}},
			wantErr:   true,
			errSubstr: "unsupported method",
		},
		{
			name: "reverse: missing url",
			step: &config.Step{HTTPRequest: &config.HTTPRequest{
				URL:            "http://x",
				Method:         "POST",
				IdempotencyKey: "k",
				Reverse:        &config.HTTPRequest{Method: "DELETE"},
			}},
			wantErr:   true,
			errSubstr: "reverse.url",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := (&Handler{}).Validate(c.step)
			if (err != nil) != c.wantErr {
				t.Fatalf("Validate err = %v, wantErr = %v", err, c.wantErr)
			}
			if err != nil && c.errSubstr != "" && !strings.Contains(err.Error(), c.errSubstr) {
				t.Errorf("err %q does not contain %q", err.Error(), c.errSubstr)
			}
		})
	}
}
