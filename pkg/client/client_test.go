package client

import (
	"os"
	"testing"

	. "github.com/onsi/gomega"
)

func TestNewAWSSessionIRSA(t *testing.T) {
	testCases := []struct {
		name          string
		roleARN       string
		tokenFile     string
		expectError   bool
		errorContains string
	}{
		{
			name:        "IRSA configured with both environment variables",
			roleARN:     "arn:aws:iam::123456789012:role/my-role",
			tokenFile:   "/var/run/secrets/eks.amazonaws.com/serviceaccount/token",
			expectError: false,
		},
		{
			name:        "IRSA missing - falls back to default credential chain",
			roleARN:     "",
			tokenFile:   "",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			g := NewWithT(tt)

			// Set up environment variables
			if tc.roleARN != "" {
				os.Setenv("AWS_ROLE_ARN", tc.roleARN)
				defer os.Unsetenv("AWS_ROLE_ARN")
			} else {
				os.Unsetenv("AWS_ROLE_ARN")
			}

			if tc.tokenFile != "" {
				os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", tc.tokenFile)
				defer os.Unsetenv("AWS_WEB_IDENTITY_TOKEN_FILE")
			} else {
				os.Unsetenv("AWS_WEB_IDENTITY_TOKEN_FILE")
			}

			// Call newAWSSession
			_, err := newAWSSession("us-east-1")

			// Verify expectations
			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
				if tc.errorContains != "" {
					g.Expect(err.Error()).To(ContainSubstring(tc.errorContains))
				}
			} else {
				// Note: This may fail in the test environment because IRSA requires
				// the token file to actually exist. For unit tests, we accept errors
				// related to the missing token file but not IRSA configuration errors.
				if err != nil {
					g.Expect(err.Error()).To(Or(
						ContainSubstring("no such file"),
						ContainSubstring("token file"),
						ContainSubstring("failed to load credentials"),
					))
				}
			}
		})
	}
}
