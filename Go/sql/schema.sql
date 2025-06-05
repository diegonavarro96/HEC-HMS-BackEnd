-- Create sequence for organizations table
CREATE SEQUENCE IF NOT EXISTS public.organizations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

-- Tabla Organizations
CREATE TABLE IF NOT EXISTS public.organizations
(
    id integer NOT NULL DEFAULT nextval('organizations_id_seq'::regclass),
    name text COLLATE pg_catalog."default" NOT NULL,
    created_at timestamp without time zone NOT NULL DEFAULT now(),
    updated_at timestamp without time zone NOT NULL DEFAULT now(),
    CONSTRAINT organizations_pkey PRIMARY KEY (id)
);

-- Tabla Usuarios con organización y rol
CREATE TABLE IF NOT EXISTS public."Users"
(
    id SERIAL PRIMARY KEY,
    username TEXT NOT NULL,
    email TEXT NOT NULL,
    "firstName" TEXT NOT NULL,
    "lastName" TEXT NOT NULL,
    role TEXT NOT NULL,
    organization_id INT NOT NULL REFERENCES public.organizations(id),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);