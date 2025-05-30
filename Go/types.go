package main

import (
	"database/sql"
	"encoding/json"

	"HMSBackend/sqlcdb"
)

// Feature represents a GeoJSON feature
type Feature struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Geometry   Geometry               `json:"geometry"`
}

// Geometry represents the geometry of a GeoJSON feature
type Geometry struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

// FeatureCollection represents a collection of GeoJSON features
type FeatureCollection struct {
	Type     string    `json:"type"`
	Features []Feature `json:"features"`
}

type BoundingBox struct {
	MinLat float64 `json:"minLat"`
	MinLon float64 `json:"minLon"`
	MaxLat float64 `json:"maxLat"`
	MaxLon float64 `json:"maxLon"`
}

type User struct {
	Email string `json:"email"`
}

type SingleFeatureMetaData struct {
	StreetName sql.NullString  `json:"streetname"`
	FromName   sql.NullString  `json:"fromname"`
	ToName     sql.NullString  `json:"toname"`
	Council    sql.NullFloat64 `json:"council"`
	ShapeLeng  sql.NullString  `json:"shape_leng"`
	Geometry   Geometry        `json:"geometry"`
}
type SingleFeature struct {
	GID      int                   `json:"gid"`
	Metadata SingleFeatureMetaData `json:"metadata"`
}

// Define a struct to maintain order
type FormResponse struct {
	StreetNames      []string `json:"street_names"`
	FromStreets      []string `json:"from_streets"`
	ToStreets        []string `json:"to_streets"`
	SideOfStreet     []string `json:"side_of_street"`
	Status           []string `json:"sidewalk_type"`
	ProjectScore     float64  `json:"project_score"`
	Priority         []string `json:"priority"`
	TotalLength      float64  `json:"total_length"`
	TotalLengthMiles float64  `json:"total_length_miles"`
	AverageWidth     float64  `json:"average_width"`
	CouncilDistrict  []string `json:"council_districts"`
	ProjectReasons   []string `json:"projectReasons"`
	ProjectStatus    []string `json:"project_status"`
	Application      []string `json:"application"`
	FundingSources   []string `json:"fundingSources"`
	IMPCost          float64  `json:"imp_cost_per_feet"`
	ProjectYears     []int32  `json:"fiscal_year"`
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

type UserResponse struct {
	IsAuthenticated bool        `json:"isAuthenticated"`
	User            interface{} `json:"user,omitempty"`
	Error           string      `json:"error,omitempty"`
	Details         string      `json:"details,omitempty"`
}

type AddProjectRequest struct {
	GID                       []int32 `json:"gid"`
	StreetName                string  `json:"street_name"`
	FromName                  string  `json:"from_name"`
	ToName                    string  `json:"to_name"`
	SideOfStreet              string  `json:"side_of_street"`
	SideWalkType              string  `json:"side_walk_type"`
	FiscalYear                int32   `json:"fiscal_year"`
	TotalLength               float64 `json:"total_length"`
	ProjectScore              float64 `json:"project_score"`
	CouncilDistrict           int32   `json:"council_district"`
	Cost                      float64 `json:"cost"`
	ProjectReason             string  `json:"project_reason"`
	FundingSource             string  `json:"funding_source"`
	Status                    string  `json:"status"`
	MaintenanceResponsibility string  `json:"maintenance_responsibility"`
	ProjectID                 int64   `json:"projectid"`
}
type AddProjectRequestSend struct {
	GID                       int32   `json:"gid"`
	StreetName                string  `json:"street_name"`
	FromName                  string  `json:"from_name"`
	ToName                    string  `json:"to_name"`
	SideOfStreet              string  `json:"side_of_street"`
	SideWalkType              string  `json:"side_walk_type"`
	FiscalYear                int32   `json:"fiscal_year"`
	TotalLength               float64 `json:"total_length"`
	ProjectScore              float64 `json:"project_score"`
	CouncilDistrict           int32   `json:"council_district"`
	FinalCost                 float64 `json:"final_cost"`
	ProjectReason             string  `json:"project_reason"`
	FundingSource             string  `json:"funding_source"`
	Status                    string  `json:"status"`
	MaintenanceResponsibility string  `json:"maintenance_responsibility"`
	ProjectID                 int64   `json:"projectid"`
	UsedDefaultInflation      bool    `json:"default_inflation"`
	InflationValue            float64 `json:"inflation_value"`
}

type ProjectsList struct {
	Gid                       int32   `json:"gid"`
	StreetName                string  `json:"street_name"`
	FromName                  string  `json:"from_name"`
	ToName                    string  `json:"to_name"`
	SideOfStreet              string  `json:"side_of_street"`
	SideWalkType              string  `json:"side_walk_type"`
	FiscalYear                int32   `json:"fiscal_year"`
	TotalLength               float64 `json:"total_length"`
	ProjectScore              float64 `json:"project_score"`
	CouncilDistrict           int32   `json:"council_district"`
	Cost                      float64 `json:"cost"`
	ProjectReason             string  `json:"project_reason"`
	FundingSource             string  `json:"funding_source"`
	Status                    string  `json:"status"`
	MaintenanceResponsibility string  `json:"maintenance_responsibility"`
	Projectid                 int64   `json:"projectid"`
}

type GetProjectsListInRangeParams struct {
	Limit  int32 `json:"limit"`
	Offset int32 `json:"offset"`
}

// Define a struct to represent the possible JSON payloads
type UpdateProjectRequest struct {
	Status     *string `json:"status,omitempty"`      // Optional field for status
	FiscalYear *int32  `json:"fiscal_year,omitempty"` // Optional field for fiscal year
	ProjectID  int64   `json:"projectId"`             // ProjectID is mandatory
}

// Define a struct to represent the possible JSON payloads
type DeleteProjectRequest struct {
	ProjectID int64 `json:"projectId"` // ProjectID is mandatory
}

type BudgetSingleResponse struct {
	CouncilDistrict int
	FiscalYear      int
	Budget          float32
	Spent           float32
	Remaining       float32
}
type BudgetByYearResponse []BudgetSingleResponse

type UserActionRequest struct {
	Action string               `json:"action"`
	User   sqlcdb.AddUserParams `json:"user"`
}

type GetUsersByClientAndRoleParams struct {
	Client string
	Role   string
}

type PrecipMeta struct {
	Timestamp string     `json:"timestamp"`
	COGPath   string     `json:"cog_path"`
	Bounds    [4]float64 `json:"bounds"`
	Width     int        `json:"width"`
	Height    int        `json:"height"`
}

type HistoricalDownloadRequest struct {
	StartDate string `json:"start_date"` // Format: YYYYMMDD
	EndDate   string `json:"end_date"`   // Format: YYYYMMDD
	StartTime string `json:"start_time"` // Format: HH:MM (ignored for now)
	EndTime   string `json:"end_time"`   // Format: HH:MM (ignored for now)
}
