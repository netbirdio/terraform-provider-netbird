package provider

import (
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestIntSetting(t *testing.T) {
	const env = "NB_TEST_INT_SETTING"
	const def = 30

	cases := []struct {
		name   string
		val    types.Int64
		setEnv bool
		env    string
		min    int
		want   int
	}{
		// min=0 (max_retries semantics): 0 is a valid value (disable retries).
		{"config value wins", types.Int64Value(5), false, "", 0, 5},
		{"config zero valid when min 0", types.Int64Value(0), true, "99", 0, 0},
		{"negative config ignored, env used", types.Int64Value(-3), true, "9", 0, 9},
		{"null config uses env", types.Int64Null(), true, "7", 0, 7},
		{"unknown config uses env", types.Int64Unknown(), true, "8", 0, 8},
		{"negative env falls back to default", types.Int64Null(), true, "-1", 0, def},
		{"unparseable env falls back to default", types.Int64Null(), true, "abc", 0, def},
		{"empty env falls back to default", types.Int64Null(), true, "", 0, def},
		{"nothing set uses default", types.Int64Null(), false, "", 0, def},
		// min=1 (request_timeout semantics): 0 is invalid and must not pass
		// through as a 0s per-attempt timeout (would cause a retry storm).
		{"config zero rejected when min 1", types.Int64Value(0), false, "", 1, def},
		{"env zero rejected when min 1", types.Int64Null(), true, "0", 1, def},
		{"config one allowed when min 1", types.Int64Value(1), false, "", 1, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setEnv {
				t.Setenv(env, tc.env)
			} else {
				_ = os.Unsetenv(env)
			}
			if got := intSetting(tc.val, env, def, tc.min); got != tc.want {
				t.Fatalf("intSetting(%v, env=%q, min=%d) = %d, want %d", tc.val, tc.env, tc.min, got, tc.want)
			}
		})
	}
}

// TestNewRetryingHTTPClientGuardsZeroTimeout verifies the constructor never
// builds a transport with a non-positive perTryTimeout, which would expire
// every attempt immediately and cause a retry storm.
func TestNewRetryingHTTPClientGuardsZeroTimeout(t *testing.T) {
	for _, d := range []time.Duration{0, -5 * time.Second} {
		c := newRetryingHTTPClient(d, 4)
		rt, ok := c.Transport.(*retryTransport)
		if !ok {
			t.Fatalf("unexpected transport type %T", c.Transport)
		}
		if rt.perTryTimeout <= 0 {
			t.Fatalf("perTryTimeout = %v, want > 0", rt.perTryTimeout)
		}
	}
}
