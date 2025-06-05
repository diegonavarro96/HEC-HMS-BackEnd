#!/bin/bash

# HEC-HMS Backend Database Seeder Script
# This script seeds the database with sample users and organizations

set -e  # Exit on error

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Database configuration (defaults)
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-hms_user}"
DB_NAME="${DB_NAME:-hms_backend}"
DB_PASSWORD=""

# Function to log messages
log() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

# Function to prompt for database password
prompt_password() {
    local password
    read -s -p "Enter PostgreSQL password for $DB_USER: " password
    echo
    echo "$password"
}

# Function to execute SQL
execute_sql() {
    local sql="$1"
    PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "$sql"
}

# Function to check if connection works
check_connection() {
    PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1;" >/dev/null 2>&1
}

# Main script
echo "================================================================="
echo "   HEC-HMS Backend Database Seeder"
echo "================================================================="
echo ""

# Check if .env file exists and source it
if [ -f "$SCRIPT_DIR/Go/.env" ]; then
    info "Loading configuration from .env file..."
    # Parse .env file
    while IFS='=' read -r key value; do
        # Remove quotes and export variables
        value="${value%\"}"
        value="${value#\"}"
        case "$key" in
            DB_HOST) DB_HOST="$value" ;;
            DB_PORT) DB_PORT="$value" ;;
            DB_USER) DB_USER="$value" ;;
            DB_PASSWORD) DB_PASSWORD="$value" ;;
            DB_NAME) DB_NAME="$value" ;;
        esac
    done < "$SCRIPT_DIR/Go/.env"
fi

info "Database Configuration:"
echo "  Host: $DB_HOST"
echo "  Port: $DB_PORT"
echo "  User: $DB_USER"
echo "  Database: $DB_NAME"
echo ""

# Get password if not set
if [ -z "$DB_PASSWORD" ]; then
    DB_PASSWORD=$(prompt_password)
fi

# Test connection
info "Testing database connection..."
if ! check_connection; then
    error "Failed to connect to database. Please check your credentials."
fi
log "Database connection successful!"

# Check if tables exist
info "Checking if tables exist..."
if ! execute_sql "SELECT 1 FROM public.organizations LIMIT 1;" >/dev/null 2>&1; then
    error "Organizations table does not exist. Please run the schema.sql first."
fi

if ! execute_sql "SELECT 1 FROM public.\"Users\" LIMIT 1;" >/dev/null 2>&1; then
    error "Users table does not exist. Please run the schema.sql first."
fi

# Ask user what they want to do
echo ""
echo "What would you like to do?"
echo "1. Add sample organizations and users (development data)"
echo "2. Add a custom organization and users"
echo "3. View existing organizations and users"
echo "4. Clear all data (WARNING: This will delete all users and organizations)"
echo ""
read -p "Enter your choice (1-4): " choice

case $choice in
    1)
        log "Adding sample organizations and users..."
        
        # Create sample organizations
        execute_sql "
        INSERT INTO public.organizations (name) 
        VALUES 
            ('Floodace Corporation'),
            ('City of San Antonio'),
            ('Texas Water Development Board')
        ON CONFLICT DO NOTHING;"
        
        # Get organization IDs
        FLOODACE_ID=$(execute_sql "SELECT id FROM public.organizations WHERE name = 'Floodace Corporation' LIMIT 1;" -t | tr -d ' ')
        CITY_ID=$(execute_sql "SELECT id FROM public.organizations WHERE name = 'City of San Antonio' LIMIT 1;" -t | tr -d ' ')
        TWDB_ID=$(execute_sql "SELECT id FROM public.organizations WHERE name = 'Texas Water Development Board' LIMIT 1;" -t | tr -d ' ')
        
        # Create sample users
        execute_sql "
        INSERT INTO public.\"Users\" (\"firstName\", \"lastName\", username, email, role, organization_id)
        VALUES 
            ('Admin', 'User', 'admin', 'admin@floodace.com', 'superUser', $FLOODACE_ID),
            ('John', 'Doe', 'johndoe', 'john.doe@floodace.com', 'admin', $FLOODACE_ID),
            ('Jane', 'Smith', 'janesmith', 'jane.smith@floodace.com', 'editor', $FLOODACE_ID),
            ('City', 'Manager', 'citymanager', 'manager@sanantonio.gov', 'admin', $CITY_ID),
            ('Water', 'Engineer', 'watereng', 'engineer@sanantonio.gov', 'editor', $CITY_ID),
            ('Texas', 'Admin', 'twdbadmin', 'admin@twdb.texas.gov', 'admin', $TWDB_ID)
        ON CONFLICT DO NOTHING;"
        
        log "Sample data added successfully!"
        ;;
        
    2)
        log "Adding custom organization and users..."
        
        # First, show existing organizations
        echo ""
        info "Existing organizations:"
        execute_sql "SELECT id, name FROM public.organizations ORDER BY name;"
        
        echo ""
        echo "Options:"
        echo "1. Use an existing organization"
        echo "2. Create a new organization"
        read -p "Enter choice (1-2): " org_choice
        
        if [ "$org_choice" = "1" ]; then
            # Use existing organization
            read -p "Enter organization ID from the list above: " ORG_ID
            
            # Verify the organization exists
            org_name=$(execute_sql "SELECT name FROM public.organizations WHERE id = $ORG_ID;" -t | tr -d ' ')
            if [ -z "$org_name" ]; then
                error "Organization with ID $ORG_ID not found!"
            fi
            log "Using organization: $org_name (ID: $ORG_ID)"
        else
            # Create new organization
            read -p "Enter new organization name: " org_name
            
            # Check if organization already exists
            existing_id=$(execute_sql "SELECT id FROM public.organizations WHERE name = '$org_name' LIMIT 1;" -t | tr -d ' ')
            
            if [ -n "$existing_id" ] && [ "$existing_id" != "" ]; then
                warning "Organization '$org_name' already exists with ID: $existing_id"
                read -p "Use existing organization? (y/n): " use_existing
                if [[ $use_existing =~ ^[Yy]$ ]]; then
                    ORG_ID=$existing_id
                    log "Using existing organization: $org_name (ID: $ORG_ID)"
                else
                    error "Please choose a different organization name."
                fi
            else
                # Create new organization
                execute_sql "INSERT INTO public.organizations (name) VALUES ('$org_name');"
                ORG_ID=$(execute_sql "SELECT id FROM public.organizations WHERE name = '$org_name' ORDER BY id DESC LIMIT 1;" -t | tr -d ' ')
                log "Organization '$org_name' created with ID: $ORG_ID"
            fi
        fi
        
        # Add users
        while true; do
            echo ""
            read -p "Add a user? (y/n): " add_user
            if [[ ! $add_user =~ ^[Yy]$ ]]; then
                break
            fi
            
            read -p "First name: " first_name
            read -p "Last name: " last_name
            read -p "Username: " username
            read -p "Email: " email
            
            echo "Select role:"
            echo "1. superUser"
            echo "2. admin"
            echo "3. editor"
            read -p "Enter choice (1-3): " role_choice
            
            case $role_choice in
                1) role="superUser" ;;
                2) role="admin" ;;
                3) role="editor" ;;
                *) role="editor" ;;
            esac
            
            execute_sql "
            INSERT INTO public.\"Users\" (\"firstName\", \"lastName\", username, email, role, organization_id)
            VALUES ('$first_name', '$last_name', '$username', '$email', '$role', $ORG_ID);"
            
            log "User $email added successfully!"
        done
        ;;
        
    3)
        log "Viewing existing data..."
        
        echo ""
        info "Organizations:"
        execute_sql "SELECT id, name, created_at FROM public.organizations ORDER BY id;"
        
        echo ""
        info "Users:"
        execute_sql "
        SELECT 
            u.id,
            u.username,
            u.email,
            u.\"firstName\" || ' ' || u.\"lastName\" as full_name,
            u.role,
            o.name as organization
        FROM public.\"Users\" u
        JOIN public.organizations o ON u.organization_id = o.id
        ORDER BY o.name, u.role, u.email;"
        ;;
        
    4)
        warning "This will delete ALL users and organizations!"
        read -p "Are you sure? Type 'DELETE ALL' to confirm: " confirm
        
        if [ "$confirm" = "DELETE ALL" ]; then
            log "Clearing all data..."
            
            # Delete in correct order due to foreign key constraints
            execute_sql "DELETE FROM public.\"Users\";"
            execute_sql "DELETE FROM public.organizations;"
            execute_sql "ALTER SEQUENCE organizations_id_seq RESTART WITH 1;"
            
            log "All data cleared!"
        else
            log "Operation cancelled."
        fi
        ;;
        
    *)
        error "Invalid choice"
        ;;
esac

echo ""
log "Database seeding complete!"

# Show summary
echo ""
info "Current database summary:"
ORG_COUNT=$(execute_sql "SELECT COUNT(*) FROM public.organizations;" -t | tr -d ' ')
USER_COUNT=$(execute_sql "SELECT COUNT(*) FROM public.\"Users\";" -t | tr -d ' ')
echo "  Total organizations: $ORG_COUNT"
echo "  Total users: $USER_COUNT"

echo ""
log "Done!"