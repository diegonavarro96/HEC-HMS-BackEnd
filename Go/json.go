package main

import (
	"log"

	"github.com/labstack/echo/v4"
)

type Response struct {
	Allowed string `json:"Allowed"`
}

func respondWithError(c echo.Context, code int, msg string) error {
	if code > 499 {
		log.Printf("Responding with 5XX error: %s", msg)
	}

	// Define the error response structure
	type errorResponse struct {
		Error string `json:"error"`
	}

	// Use Echo's c.JSON method to send a JSON response
	return c.JSON(code, errorResponse{
		Error: msg,
	})
}
func respondWithJSON(c echo.Context, code int, payload interface{}) error {
	// Set the Content-Type header to "application/json" and send the response
	return c.JSON(code, payload)
}
