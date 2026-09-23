package mysql

import (
	"context"
	"testing"

	"cloud.google.com/go/cloudsqlconn"
)

func TestCloudsqlAuth(t *testing.T) {
	tests := []struct {
		name          string
		iamAuth       bool
		impersonateSA string
		password      string
		expected      cloudsqlAuthMode
	}{
		{
			name:          "iam auth with impersonation",
			iamAuth:       true,
			impersonateSA: "sa@project.iam.gserviceaccount.com",
			expected:      cloudsqlImpersonatedIAM,
		},
		{
			name:          "impersonation wins over a token in the password",
			iamAuth:       true,
			impersonateSA: "sa@project.iam.gserviceaccount.com",
			password:      "ya29.some-access-token",
			expected:      cloudsqlImpersonatedIAM,
		},
		{
			name:     "iam auth with a token in the password",
			iamAuth:  true,
			password: "ya29.some-access-token",
			expected: cloudsqlTokenIAM,
		},
		{
			name:     "iam auth without token falls back to default credentials",
			iamAuth:  true,
			expected: cloudsqlADCIAM,
		},
		{
			name:          "mysql user with impersonation",
			impersonateSA: "sa@project.iam.gserviceaccount.com",
			password:      "hunter2",
			expected:      cloudsqlImpersonatedPassword,
		},
		{
			name:     "mysql user with default credentials",
			password: "hunter2",
			expected: cloudsqlADCPassword,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cloudsqlAuth(tt.iamAuth, tt.impersonateSA, tt.password)
			if got != tt.expected {
				t.Errorf("expected auth mode %d, got %d", tt.expected, got)
			}
		})
	}
}

func TestCloudsqlDsnPassword(t *testing.T) {
	tests := []struct {
		name     string
		mode     cloudsqlAuthMode
		password string
		expected string
	}{
		{
			name:     "dropped when impersonating with iam auth",
			mode:     cloudsqlImpersonatedIAM,
			password: "leftover-from-the-environment",
			expected: "",
		},
		{
			name:     "kept for a mysql user reached through impersonation",
			mode:     cloudsqlImpersonatedPassword,
			password: "hunter2",
			expected: "hunter2",
		},
		{
			name:     "kept for the legacy access token",
			mode:     cloudsqlTokenIAM,
			password: "ya29.some-access-token",
			expected: "ya29.some-access-token",
		},
		{
			name:     "kept for a plain mysql user",
			mode:     cloudsqlADCPassword,
			password: "hunter2",
			expected: "hunter2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cloudsqlDsnPassword(tt.mode, tt.password); got != tt.expected {
				t.Errorf("expected password %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestGcpServiceAccountEmailRegexp(t *testing.T) {
	valid := []string{
		"",
		"sa-name@project-id.iam.gserviceaccount.com",
		"123456789-compute@developer.gserviceaccount.com",
		"project-id@appspot.gserviceaccount.com",
	}
	for _, email := range valid {
		if !gcpServiceAccountEmailRegexp.MatchString(email) {
			t.Errorf("expected %q to be a valid service account email", email)
		}
	}

	invalid := []string{
		// A shortened form that is not a service account email.
		"sa-name@project-id.iam",
		"sa-name",
		"sa-name@gserviceaccount.com.evil.com",
		"sa name@project-id.iam.gserviceaccount.com",
	}
	for _, email := range invalid {
		if gcpServiceAccountEmailRegexp.MatchString(email) {
			t.Errorf("expected %q to be rejected", email)
		}
	}
}

// The connector rejects IAM AuthN combined with a plain token source, so make
// sure the options built for the static token mode are actually accepted. The
// other modes cannot be checked this way, they need application default
// credentials.
func TestCloudsqlOptionsAcceptedByConnector(t *testing.T) {
	opts, err := cloudsqlOptions(cloudsqlTokenIAM, "", "ya29.some-access-token", true)
	if err != nil {
		t.Fatalf("unexpected error building options: %v", err)
	}

	dialer, err := cloudsqlconn.NewDialer(context.Background(), opts...)
	if err != nil {
		t.Fatalf("connector rejected the options: %v", err)
	}
	defer dialer.Close()
}
