package main

import (
	"context"
	"os"
	"testing"
	
	"github.com/joho/godotenv"
)

func TestSendSMS(t *testing.T) {
	tests := []struct {
		name      string
		setupEnv  func()
		to        string
		body      string
		wantError bool
		errorMsg  string
	}{
		{
			name: "missing account SID",
			setupEnv: func() {
				os.Unsetenv("TWILIO_ACCOUNT_SID")
				os.Setenv("TWILIO_AUTH_TOKEN", "test-token")
				os.Setenv("TWILIO_FROM_NUMBER", "+1234567890")
			},
			to:        "+2103883174",
			body:      "Test message",
			wantError: true,
			errorMsg:  "missing required Twilio environment variables",
		},
		{
			name: "missing auth token",
			setupEnv: func() {
				os.Setenv("TWILIO_ACCOUNT_SID", "test-sid")
				os.Unsetenv("TWILIO_AUTH_TOKEN")
				os.Setenv("TWILIO_FROM_NUMBER", "+1234567890")
			},
			to:        "+1987654321",
			body:      "Test message",
			wantError: true,
			errorMsg:  "missing required Twilio environment variables",
		},
		{
			name: "missing from number",
			setupEnv: func() {
				os.Setenv("TWILIO_ACCOUNT_SID", "test-sid")
				os.Setenv("TWILIO_AUTH_TOKEN", "test-token")
				os.Unsetenv("TWILIO_FROM_NUMBER")
			},
			to:        "+1987654321",
			body:      "Test message",
			wantError: true,
			errorMsg:  "missing required Twilio environment variables",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalSid := os.Getenv("TWILIO_ACCOUNT_SID")
			originalToken := os.Getenv("TWILIO_AUTH_TOKEN")
			originalFrom := os.Getenv("TWILIO_FROM_NUMBER")

			defer func() {
				os.Setenv("TWILIO_ACCOUNT_SID", originalSid)
				os.Setenv("TWILIO_AUTH_TOKEN", originalToken)
				os.Setenv("TWILIO_FROM_NUMBER", originalFrom)
			}()

			tt.setupEnv()

			err := SendSMS(context.Background(), tt.to, tt.body)

			if tt.wantError {
				if err == nil {
					t.Errorf("expected error containing '%s', got nil", tt.errorMsg)
				} else if err.Error() != tt.errorMsg {
					t.Errorf("expected error '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// Example of how to use a mock Twilio client in integration tests
// This demonstrates the pattern but requires additional setup with gomock
/*
type MockTwilioClient interface {
	CreateMessage(params *openapi.CreateMessageParams) (*openapi.ApiV2010Message, error)
}

func TestSendSMSWithMock(t *testing.T) {
	// This example shows how you would structure tests with a mock client
	// In production code, you'd need to refactor SendSMS to accept a client interface
	// rather than creating the client internally, to enable dependency injection

	// Example usage:
	// mockClient := NewMockTwilioClient(ctrl)
	// mockClient.EXPECT().CreateMessage(gomock.Any()).Return(&openapi.ApiV2010Message{
	//     Status: &successStatus,
	// }, nil)
}
*/

// TestSendSMSIntegration is a real integration test that sends an actual SMS
// Run with: go test -v -run TestSendSMSIntegration
// Skip with -short flag: go test -short
func TestSendSMSIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Load .env file
	if err := godotenv.Load(); err != nil {
		t.Logf("Warning: Could not load .env file: %v", err)
	}

	// Check if all required environment variables are set
	if os.Getenv("TWILIO_ACCOUNT_SID") == "" ||
		os.Getenv("TWILIO_AUTH_TOKEN") == "" ||
		os.Getenv("TWILIO_FROM_NUMBER") == "" {
		t.Skip("Skipping integration test: Twilio environment variables not set")
	}

	// IMPORTANT: Change this to your actual phone number!
	testPhoneNumber := "+1234567890" // CHANGE THIS TO YOUR NUMBER
	
	// You can also use an environment variable for the test number
	if envPhone := os.Getenv("TEST_PHONE_NUMBER"); envPhone != "" {
		testPhoneNumber = envPhone
	}

	testMessage := "HMS Backend SMS Test - This is a test message!"

	t.Logf("Attempting to send SMS to %s", testPhoneNumber)
	
	err := SendSMS(context.Background(), testPhoneNumber, testMessage)
	if err != nil {
		t.Errorf("Failed to send SMS: %v", err)
	} else {
		t.Logf("SMS sent successfully to %s", testPhoneNumber)
	}
}
