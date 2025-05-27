-- name: GetUsers :many
SELECT 
    id, 
    username, 
    email 
FROM 
    public."Users";

-- name: GetUsersWithRole :many
SELECT 
    u.id,
    u.username,
    u.email,
    u."firstName",
    u."lastName",
    u.role,
    u.organization_id,
    o.name AS organization_name,
    u.created_at,
    u.updated_at
FROM public."Users" u
JOIN public.organizations o ON u.organization_id = o.id;

-- name: AddUser :exec
INSERT INTO public."Users" (
    "firstName",
    "lastName",
    username,
    email,
    role,
    organization_id
) VALUES (
    $1, $2, $3, $4, $5, $6
);

-- name: UpdateUser :exec
UPDATE public."Users"
SET
    "firstName" = $1,
    "lastName" = $2,
    username = $3,
    email = $4,
    role = $5,
    organization_id = $6
WHERE
    email = $7;

-- name: DeleteUser :exec
DELETE FROM public."Users"
WHERE email = $1;

-- name: GetUsersByOrganizationAndRole :many
SELECT
    u.id,
    u.username,
    u.email,
    u."firstName",
    u."lastName",
    u.role,
    u.organization_id,
    o.name AS organization_name,
    u.created_at,
    u.updated_at
FROM public."Users" u
JOIN public.organizations o ON u.organization_id = o.id
WHERE 
    u.organization_id = $1
    AND (
        CASE u.role
            WHEN 'superUser' THEN 3
            WHEN 'admin' THEN 2
            WHEN 'editor' THEN 1
            ELSE 0
        END
    ) <= (
        CASE $2
            WHEN 'superUser' THEN 3
            WHEN 'admin' THEN 2
            WHEN 'editor' THEN 1
            ELSE 0
        END
    )
ORDER BY u.email;

-- name: GetUserByEmail :one
SELECT
    u.id,
    u.username,
    u.email,
    u."firstName",
    u."lastName",
    u.role,
    u.organization_id,
    o.name AS organization_name,
    u.created_at,
    u.updated_at
FROM public."Users" u
JOIN public.organizations o ON u.organization_id = o.id
WHERE u.email = $1
LIMIT 1;
