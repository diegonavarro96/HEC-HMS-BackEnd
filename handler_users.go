package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"HMSBackend/sqlcdb"
)
func validateUser(queries *sqlcdb.Queries, validateUsername string) bool {

	users, err := queries.GetUsers(context.Background())

	if err != nil {
		return false
	}

	for _, user := range users {
		if user.Email == validateUsername {
			return true
		}
	}
	return false

}


func handleValidateUser(queries *sqlcdb.Queries) echo.HandlerFunc {
	return func(c echo.Context) error {
		var user User
		if err := c.Bind(&user); err != nil {
			return respondWithError(c, http.StatusBadRequest, "Could not read request json from client")
		}
		userAllowed := Response{
			Allowed: "true",
		}
		if validateUser(queries, user.Email) {
			return respondWithJSON(c, http.StatusOK, userAllowed)
		}
		log.Printf("Could not find %s in database\n", user.Email)
		return respondWithError(c, 500, "user is not allowed")
	}
}

func handleCallback(c echo.Context) error {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	redirect := os.Getenv("REDIRECT_URL")
	code := c.QueryParam("code")

	if code == "" {
		return respondWithError(c, http.StatusBadRequest, "Missing authorization code")
	}

	tokenResponse, err := exchangeCodeForToken(code)
	if err != nil {
		return respondWithError(c, http.StatusUnauthorized, "authentication_failed")
	}

	// Set cookie
	cookie := new(http.Cookie)
	cookie.Name = "arcgis_token"
	cookie.Value = tokenResponse.AccessToken
	cookie.HttpOnly = true
	cookie.Secure = os.Getenv("NODE_ENV") != "development"
	cookie.Expires = time.Now().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second)
	cookie.SameSite = http.SameSiteLaxMode
	cookie.Path = "/"
	c.SetCookie(cookie)

	return c.Redirect(http.StatusFound, redirect)
}

func exchangeCodeForToken(code string) (*TokenResponse, error) {
	form := url.Values{}
	form.Add("grant_type", "authorization_code")
	form.Add("client_id", os.Getenv("ARCGIS_CLIENT_ID"))
	form.Add("client_secret", os.Getenv("ARCGIS_CLIENT_SECRET"))
	form.Add("code", code)
	form.Add("redirect_uri", os.Getenv("REDIRECT_URI"))

	req, err := http.NewRequest("POST", "https://www.arcgis.com/sharing/rest/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tokenResponse TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed: %s", tokenResponse.ErrorDesc)
	}

	return &tokenResponse, nil
}

func handleUserSession(c echo.Context) error {
	// Get the token from the cookies
	cookie, err := c.Cookie("arcgis_token")
	if err != nil {
		log.Println("No token found in cookies")
		return c.JSON(http.StatusUnauthorized, UserResponse{IsAuthenticated: false})
	}

	token := cookie.Value
	log.Println("Token found, verifying with ArcGIS")

	req, err := http.NewRequest("GET", "https://www.arcgis.com/sharing/rest/community/self?f=json", nil)
	if err != nil {
		log.Println("Error creating request:", err)
		return c.JSON(http.StatusInternalServerError, UserResponse{Error: "Internal server error"})
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error sending request to ArcGIS:", err)
		return c.JSON(http.StatusInternalServerError, UserResponse{Error: "Internal server error"})
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error reading response body:", err)
		return c.JSON(http.StatusInternalServerError, UserResponse{Error: "Internal server error"})
	}

	if resp.StatusCode == http.StatusOK {
		var userData interface{}
		if err := json.Unmarshal(body, &userData); err != nil {
			log.Println("Error parsing user data:", err)
			return c.JSON(http.StatusInternalServerError, UserResponse{
				Error:   "Error parsing user data",
				Details: string(body),
			})
		}

		log.Println("User data retrieved successfully")
		return c.JSON(http.StatusOK, UserResponse{IsAuthenticated: true, User: userData})
	} else {
		log.Println("Failed to retrieve user data")
		return c.JSON(http.StatusUnauthorized, UserResponse{
			IsAuthenticated: false,
			Details:         string(body),
		})
	}
}

func handleGetAllUsers(queries *sqlcdb.Queries) echo.HandlerFunc {
	return func(c echo.Context) error {
		cookie, err := c.Cookie("arcgis_token")
		if err != nil {
			return respondWithError(c, http.StatusUnauthorized, "Authentication required, missing token")
		}

		// Get user info from ArcGIS
		req, err := http.NewRequest("GET", "https://www.arcgis.com/sharing/rest/community/self?f=json", nil)
		if err != nil {
			return respondWithError(c, http.StatusInternalServerError, "Error creating request")
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cookie.Value))

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return respondWithError(c, http.StatusInternalServerError, "Error fetching user info")
		}
		defer resp.Body.Close()

		var arcgisUser struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&arcgisUser); err != nil {
			return respondWithError(c, http.StatusInternalServerError, "Error parsing user info")
		}

		log.Printf("User info: %+v", arcgisUser)
		// Get the requesting user's details from our database
		requestingUser, err := queries.GetUserByEmail(c.Request().Context(), arcgisUser.Email)
		if err != nil {
			return respondWithError(c, http.StatusInternalServerError, "Error fetching user details")
		}

		switch requestingUser.Role {
		case "superUser":
			// Fetch all users
			users, err := queries.GetUsersWithRole(c.Request().Context())
			if err != nil {
				return respondWithError(c, http.StatusInternalServerError, "Could not fetch users from the database")
			}
			return respondWithJSON(c, http.StatusOK, users)

		case "admin":
			// Fetch users from the same organization with lower or equal role level
			users, err := queries.GetUsersByOrganizationAndRole(c.Request().Context(), sqlcdb.GetUsersByOrganizationAndRoleParams{
				OrganizationID: requestingUser.OrganizationID,
				Role:           "admin",
			})
			if err != nil {
				return respondWithError(c, http.StatusInternalServerError, "Could not fetch users from the database")
			}
			return respondWithJSON(c, http.StatusOK, users)

		case "editor":
			return respondWithError(c, http.StatusForbidden, "Editors do not have access to user management")

		default:
			return respondWithError(c, http.StatusForbidden, "Invalid role for this operation")
		}
	}
}

func handleModifyUser(queries *sqlcdb.Queries) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req UserActionRequest

		// Bind request payload
		if err := c.Bind(&req); err != nil {
			log.Printf("Error binding request: %v", err)
			return respondWithError(c, http.StatusBadRequest, "Invalid request payload")
		}

		switch req.Action {
		case "delete":
			// Handle delete
			err := queries.DeleteUser(c.Request().Context(), req.User.Email)
			if err != nil {
				log.Printf("Error deleting user with Email %s: %v", req.User.Email, err)
				return respondWithError(c, http.StatusInternalServerError, "Failed to delete user")
			}
			log.Printf("User with Email %s deleted successfully", req.User.Email)
			return respondWithJSON(c, http.StatusOK, map[string]string{"message": "User deleted successfully"})

		case "update":
			auxUser := sqlcdb.UpdateUserParams{
				FirstName:      req.User.FirstName,
				LastName:       req.User.LastName,
				Username:       req.User.Username,
				Email:          req.User.Email,
				Role:           req.User.Role,
				OrganizationID: req.User.OrganizationID,
				Email_2:        req.User.Email, // The condition to find the user
			}
			// Handle modify
			err := queries.UpdateUser(c.Request().Context(), auxUser)
			if err != nil {
				log.Printf("Error modifying user with Email %s: %v", req.User.Email, err)
				return respondWithError(c, http.StatusInternalServerError, "Failed to modify user")
			}
			log.Printf("User with Email %s modified successfully", req.User.Email)
			return respondWithJSON(c, http.StatusOK, map[string]string{"message": "User modified successfully"})

		case "add":
			newUser := sqlcdb.AddUserParams{
				FirstName:      req.User.FirstName,
				LastName:       req.User.LastName,
				Username:       req.User.Username,
				Email:          req.User.Email,
				Role:           req.User.Role,
				OrganizationID: req.User.OrganizationID,
			}
			// Handle add
			err := queries.AddUser(c.Request().Context(), newUser)
			if err != nil {
				log.Printf("Error adding user: %v", err)
				return respondWithError(c, http.StatusInternalServerError, "Failed to add user")
			}
			log.Printf("User with Email %s added successfully", req.User.Email)
			return respondWithJSON(c, http.StatusOK, map[string]string{"message": "User added successfully"})

		default:
			return respondWithError(c, http.StatusBadRequest, "Invalid action. Supported actions are 'delete', 'update', and 'add'")
		}
	}
}