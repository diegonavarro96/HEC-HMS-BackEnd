package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/twilio/twilio-go"
	openapi "github.com/twilio/twilio-go/rest/api/v2010"
)

// SendSMS sends `body` to the phone number `to` using Twilio
// and returns an error if the API call fails.
func SendSMS(ctx context.Context, to, body string) error {
	accountSid := os.Getenv("TWILIO_ACCOUNT_SID")
	authToken := os.Getenv("TWILIO_AUTH_TOKEN")
	fromNumber := os.Getenv("TWILIO_FROM_NUMBER")

	if accountSid == "" || authToken == "" || fromNumber == "" {
		return fmt.Errorf("missing required Twilio environment variables")
	}

	client := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: accountSid,
		Password: authToken,
	})

	params := &openapi.CreateMessageParams{}
	params.SetTo(to)
	params.SetFrom(fromNumber)
	params.SetBody(body)

	resp, err := client.Api.CreateMessage(params)
	if err != nil {
		return fmt.Errorf("failed to send SMS to %s: %w", to, err)
	}

	if resp.Status != nil && *resp.Status == "failed" {
		return fmt.Errorf("SMS to %s failed with status: %s", to, *resp.Status)
	}

	return nil
}

// SendSMSRequest represents the request body for sending an SMS
type SendSMSRequest struct {
	To      string `json:"to" validate:"required"`      // Phone number to send SMS to (with country code)
	Message string `json:"message" validate:"required"` // Message body
}

// SendSMSResponse represents the response after sending an SMS
type SendSMSResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// handleSendSMS handles the API endpoint for sending SMS messages
func handleSendSMS(c echo.Context) error {
	var req SendSMSRequest
	
	// Parse and validate the request body
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, SendSMSResponse{
			Success: false,
			Error:   "Invalid request body",
		})
	}
	
	// Validate required fields
	if req.To == "" || req.Message == "" {
		return c.JSON(http.StatusBadRequest, SendSMSResponse{
			Success: false,
			Error:   "Both 'to' and 'message' fields are required",
		})
	}
	
	// Send the SMS
	ctx := c.Request().Context()
	if err := SendSMS(ctx, req.To, req.Message); err != nil {
		// Log the error but don't expose internal details to the client
		c.Logger().Errorf("Failed to send SMS: %v", err)
		
		return c.JSON(http.StatusInternalServerError, SendSMSResponse{
			Success: false,
			Error:   "Failed to send SMS",
		})
	}
	
	return c.JSON(http.StatusOK, SendSMSResponse{
		Success: true,
		Message: fmt.Sprintf("SMS sent successfully to %s", req.To),
	})
}
