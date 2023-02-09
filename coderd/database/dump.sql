-- Code generated by 'make coderd/database/generate'. DO NOT EDIT.

CREATE TYPE api_key_scope AS ENUM (
    'all',
    'application_connect'
);

CREATE TYPE app_sharing_level AS ENUM (
    'owner',
    'authenticated',
    'public'
);

CREATE TYPE audit_action AS ENUM (
    'create',
    'write',
    'delete',
    'start',
    'stop',
    'login',
    'logout'
);

CREATE TYPE build_reason AS ENUM (
    'initiator',
    'autostart',
    'autostop'
);

CREATE TYPE log_level AS ENUM (
    'trace',
    'debug',
    'info',
    'warn',
    'error'
);

CREATE TYPE log_source AS ENUM (
    'provisioner_daemon',
    'provisioner'
);

CREATE TYPE login_type AS ENUM (
    'password',
    'github',
    'oidc',
    'token'
);

CREATE TYPE parameter_destination_scheme AS ENUM (
    'none',
    'environment_variable',
    'provisioner_variable'
);

CREATE TYPE parameter_scope AS ENUM (
    'template',
    'import_job',
    'workspace'
);

CREATE TYPE parameter_source_scheme AS ENUM (
    'none',
    'data'
);

CREATE TYPE parameter_type_system AS ENUM (
    'none',
    'hcl'
);

CREATE TYPE provisioner_job_type AS ENUM (
    'template_version_import',
    'workspace_build',
    'template_version_dry_run'
);

CREATE TYPE provisioner_storage_method AS ENUM (
    'file'
);

CREATE TYPE provisioner_type AS ENUM (
    'echo',
    'terraform'
);

CREATE TYPE resource_type AS ENUM (
    'organization',
    'template',
    'template_version',
    'user',
    'workspace',
    'git_ssh_key',
    'api_key',
    'group',
    'workspace_build',
    'license'
);

CREATE TYPE user_status AS ENUM (
    'active',
    'suspended'
);

CREATE TYPE workspace_agent_lifecycle_state AS ENUM (
    'created',
    'starting',
    'start_timeout',
    'start_error',
    'ready'
);

CREATE TYPE workspace_app_health AS ENUM (
    'disabled',
    'initializing',
    'healthy',
    'unhealthy'
);

CREATE TYPE workspace_transition AS ENUM (
    'start',
    'stop',
    'delete'
);

CREATE TABLE agent_stats (
    id uuid NOT NULL,
    created_at timestamp with time zone NOT NULL,
    user_id uuid NOT NULL,
    agent_id uuid NOT NULL,
    workspace_id uuid NOT NULL,
    template_id uuid NOT NULL,
    payload jsonb NOT NULL
);

CREATE TABLE api_keys (
    id text NOT NULL,
    hashed_secret bytea NOT NULL,
    user_id uuid NOT NULL,
    last_used timestamp with time zone NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    login_type login_type NOT NULL,
    lifetime_seconds bigint DEFAULT 86400 NOT NULL,
    ip_address inet DEFAULT '0.0.0.0'::inet NOT NULL,
    scope api_key_scope DEFAULT 'all'::api_key_scope NOT NULL
);

COMMENT ON COLUMN api_keys.hashed_secret IS 'hashed_secret contains a SHA256 hash of the key secret. This is considered a secret and MUST NOT be returned from the API as it is used for API key encryption in app proxying code.';

CREATE TABLE audit_logs (
    id uuid NOT NULL,
    "time" timestamp with time zone NOT NULL,
    user_id uuid NOT NULL,
    organization_id uuid NOT NULL,
    ip inet,
    user_agent character varying(256),
    resource_type resource_type NOT NULL,
    resource_id uuid NOT NULL,
    resource_target text NOT NULL,
    action audit_action NOT NULL,
    diff jsonb NOT NULL,
    status_code integer NOT NULL,
    additional_fields jsonb NOT NULL,
    request_id uuid NOT NULL,
    resource_icon text NOT NULL
);

CREATE TABLE files (
    hash character varying(64) NOT NULL,
    created_at timestamp with time zone NOT NULL,
    created_by uuid NOT NULL,
    mimetype character varying(64) NOT NULL,
    data bytea NOT NULL,
    id uuid DEFAULT gen_random_uuid() NOT NULL
);

CREATE TABLE git_auth_links (
    provider_id text NOT NULL,
    user_id uuid NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    oauth_access_token text NOT NULL,
    oauth_refresh_token text NOT NULL,
    oauth_expiry timestamp with time zone NOT NULL
);

CREATE TABLE gitsshkeys (
    user_id uuid NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    private_key text NOT NULL,
    public_key text NOT NULL
);

CREATE TABLE group_members (
    user_id uuid NOT NULL,
    group_id uuid NOT NULL
);

CREATE TABLE groups (
    id uuid NOT NULL,
    name text NOT NULL,
    organization_id uuid NOT NULL,
    avatar_url text DEFAULT ''::text NOT NULL,
    quota_allowance integer DEFAULT 0 NOT NULL
);

CREATE TABLE licenses (
    id integer NOT NULL,
    uploaded_at timestamp with time zone NOT NULL,
    jwt text NOT NULL,
    exp timestamp with time zone NOT NULL,
    uuid uuid NOT NULL
);

COMMENT ON COLUMN licenses.exp IS 'exp tracks the claim of the same name in the JWT, and we include it here so that we can easily query for licenses that have not yet expired.';

CREATE SEQUENCE licenses_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE licenses_id_seq OWNED BY licenses.id;

CREATE TABLE organization_members (
    user_id uuid NOT NULL,
    organization_id uuid NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    roles text[] DEFAULT '{organization-member}'::text[] NOT NULL
);

CREATE TABLE organizations (
    id uuid NOT NULL,
    name text NOT NULL,
    description text NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL
);

CREATE TABLE parameter_schemas (
    id uuid NOT NULL,
    created_at timestamp with time zone NOT NULL,
    job_id uuid NOT NULL,
    name character varying(64) NOT NULL,
    description character varying(8192) DEFAULT ''::character varying NOT NULL,
    default_source_scheme parameter_source_scheme NOT NULL,
    default_source_value text NOT NULL,
    allow_override_source boolean NOT NULL,
    default_destination_scheme parameter_destination_scheme NOT NULL,
    allow_override_destination boolean NOT NULL,
    default_refresh text NOT NULL,
    redisplay_value boolean NOT NULL,
    validation_error character varying(256) NOT NULL,
    validation_condition character varying(512) NOT NULL,
    validation_type_system parameter_type_system NOT NULL,
    validation_value_type character varying(64) NOT NULL,
    index integer NOT NULL
);

CREATE TABLE parameter_values (
    id uuid NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    scope parameter_scope NOT NULL,
    scope_id uuid NOT NULL,
    name character varying(64) NOT NULL,
    source_scheme parameter_source_scheme NOT NULL,
    source_value text NOT NULL,
    destination_scheme parameter_destination_scheme NOT NULL
);

CREATE TABLE provisioner_daemons (
    id uuid NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone,
    name character varying(64) NOT NULL,
    provisioners provisioner_type[] NOT NULL,
    replica_id uuid,
    tags jsonb DEFAULT '{}'::jsonb NOT NULL
);

CREATE TABLE provisioner_job_logs (
    job_id uuid NOT NULL,
    created_at timestamp with time zone NOT NULL,
    source log_source NOT NULL,
    level log_level NOT NULL,
    stage character varying(128) NOT NULL,
    output character varying(1024) NOT NULL,
    id bigint NOT NULL
);

CREATE SEQUENCE provisioner_job_logs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE provisioner_job_logs_id_seq OWNED BY provisioner_job_logs.id;

CREATE TABLE provisioner_jobs (
    id uuid NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    started_at timestamp with time zone,
    canceled_at timestamp with time zone,
    completed_at timestamp with time zone,
    error text,
    organization_id uuid NOT NULL,
    initiator_id uuid NOT NULL,
    provisioner provisioner_type NOT NULL,
    storage_method provisioner_storage_method NOT NULL,
    type provisioner_job_type NOT NULL,
    input jsonb NOT NULL,
    worker_id uuid,
    file_id uuid NOT NULL,
    tags jsonb DEFAULT '{"scope": "organization"}'::jsonb NOT NULL
);

CREATE TABLE replicas (
    id uuid NOT NULL,
    created_at timestamp with time zone NOT NULL,
    started_at timestamp with time zone NOT NULL,
    stopped_at timestamp with time zone,
    updated_at timestamp with time zone NOT NULL,
    hostname text NOT NULL,
    region_id integer NOT NULL,
    relay_address text NOT NULL,
    database_latency integer NOT NULL,
    version text NOT NULL,
    error text DEFAULT ''::text NOT NULL
);

CREATE TABLE site_configs (
    key character varying(256) NOT NULL,
    value character varying(8192) NOT NULL
);

CREATE TABLE template_version_parameters (
    template_version_id uuid NOT NULL,
    name text NOT NULL,
    description text NOT NULL,
    type text NOT NULL,
    mutable boolean NOT NULL,
    default_value text NOT NULL,
    icon text NOT NULL,
    options jsonb DEFAULT '[]'::jsonb NOT NULL,
    validation_regex text NOT NULL,
    validation_min integer NOT NULL,
    validation_max integer NOT NULL,
    validation_error text DEFAULT ''::text NOT NULL,
    validation_monotonic text DEFAULT ''::text NOT NULL,
    CONSTRAINT validation_monotonic_order CHECK ((validation_monotonic = ANY (ARRAY['increasing'::text, 'decreasing'::text, ''::text])))
);

COMMENT ON COLUMN template_version_parameters.name IS 'Parameter name';

COMMENT ON COLUMN template_version_parameters.description IS 'Parameter description';

COMMENT ON COLUMN template_version_parameters.type IS 'Parameter type';

COMMENT ON COLUMN template_version_parameters.mutable IS 'Is parameter mutable?';

COMMENT ON COLUMN template_version_parameters.default_value IS 'Default value';

COMMENT ON COLUMN template_version_parameters.icon IS 'Icon';

COMMENT ON COLUMN template_version_parameters.options IS 'Additional options';

COMMENT ON COLUMN template_version_parameters.validation_regex IS 'Validation: regex pattern';

COMMENT ON COLUMN template_version_parameters.validation_min IS 'Validation: minimum length of value';

COMMENT ON COLUMN template_version_parameters.validation_max IS 'Validation: maximum length of value';

COMMENT ON COLUMN template_version_parameters.validation_error IS 'Validation: error displayed when the regex does not match.';

COMMENT ON COLUMN template_version_parameters.validation_monotonic IS 'Validation: consecutive values preserve the monotonic order';

CREATE TABLE template_version_variables (
    template_version_id uuid NOT NULL,
    name text NOT NULL,
    description text NOT NULL,
    type text NOT NULL,
    value text NOT NULL,
    default_value text NOT NULL,
    required boolean NOT NULL,
    sensitive boolean NOT NULL
);

COMMENT ON COLUMN template_version_variables.name IS 'Variable name';

COMMENT ON COLUMN template_version_variables.description IS 'Variable description';

COMMENT ON COLUMN template_version_variables.type IS 'Variable type';

COMMENT ON COLUMN template_version_variables.value IS 'Variable value';

COMMENT ON COLUMN template_version_variables.default_value IS 'Variable default value';

COMMENT ON COLUMN template_version_variables.required IS 'Required variables needs a default value or a value provided by template admin';

COMMENT ON COLUMN template_version_variables.sensitive IS 'Sensitive variables have their values redacted in logs or site UI';

CREATE TABLE template_versions (
    id uuid NOT NULL,
    template_id uuid,
    organization_id uuid NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    name character varying(64) NOT NULL,
    readme character varying(1048576) NOT NULL,
    job_id uuid NOT NULL,
    created_by uuid NOT NULL
);

CREATE TABLE templates (
    id uuid NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    organization_id uuid NOT NULL,
    deleted boolean DEFAULT false NOT NULL,
    name character varying(64) NOT NULL,
    provisioner provisioner_type NOT NULL,
    active_version_id uuid NOT NULL,
    description character varying(128) DEFAULT ''::character varying NOT NULL,
    default_ttl bigint DEFAULT '604800000000000'::bigint NOT NULL,
    created_by uuid NOT NULL,
    icon character varying(256) DEFAULT ''::character varying NOT NULL,
    user_acl jsonb DEFAULT '{}'::jsonb NOT NULL,
    group_acl jsonb DEFAULT '{}'::jsonb NOT NULL,
    display_name character varying(64) DEFAULT ''::character varying NOT NULL,
    allow_user_cancel_workspace_jobs boolean DEFAULT true NOT NULL
);

COMMENT ON COLUMN templates.default_ttl IS 'The default duration for auto-stop for workspaces created from this template.';

COMMENT ON COLUMN templates.display_name IS 'Display name is a custom, human-friendly template name that user can set.';

COMMENT ON COLUMN templates.allow_user_cancel_workspace_jobs IS 'Allow users to cancel in-progress workspace jobs.';

CREATE TABLE user_links (
    user_id uuid NOT NULL,
    login_type login_type NOT NULL,
    linked_id text DEFAULT ''::text NOT NULL,
    oauth_access_token text DEFAULT ''::text NOT NULL,
    oauth_refresh_token text DEFAULT ''::text NOT NULL,
    oauth_expiry timestamp with time zone DEFAULT '0001-01-01 00:00:00+00'::timestamp with time zone NOT NULL
);

CREATE TABLE users (
    id uuid NOT NULL,
    email text NOT NULL,
    username text DEFAULT ''::text NOT NULL,
    hashed_password bytea NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    status user_status DEFAULT 'active'::user_status NOT NULL,
    rbac_roles text[] DEFAULT '{}'::text[] NOT NULL,
    login_type login_type DEFAULT 'password'::login_type NOT NULL,
    avatar_url text,
    deleted boolean DEFAULT false NOT NULL,
    last_seen_at timestamp without time zone DEFAULT '0001-01-01 00:00:00'::timestamp without time zone NOT NULL
);

CREATE TABLE workspace_agents (
    id uuid NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    name character varying(64) NOT NULL,
    first_connected_at timestamp with time zone,
    last_connected_at timestamp with time zone,
    disconnected_at timestamp with time zone,
    resource_id uuid NOT NULL,
    auth_token uuid NOT NULL,
    auth_instance_id character varying,
    architecture character varying(64) NOT NULL,
    environment_variables jsonb,
    operating_system character varying(64) NOT NULL,
    startup_script character varying(65534),
    instance_metadata jsonb,
    resource_metadata jsonb,
    directory character varying(4096) DEFAULT ''::character varying NOT NULL,
    version text DEFAULT ''::text NOT NULL,
    last_connected_replica_id uuid,
    connection_timeout_seconds integer DEFAULT 0 NOT NULL,
    troubleshooting_url text DEFAULT ''::text NOT NULL,
    motd_file text DEFAULT ''::text NOT NULL,
    lifecycle_state workspace_agent_lifecycle_state DEFAULT 'created'::workspace_agent_lifecycle_state NOT NULL,
    login_before_ready boolean DEFAULT true NOT NULL,
    startup_script_timeout_seconds integer DEFAULT 0 NOT NULL,
    expanded_directory character varying(4096) DEFAULT ''::character varying NOT NULL,
    shutdown_script character varying(65534),
    shutdown_script_timeout_seconds integer DEFAULT 0 NOT NULL
);

COMMENT ON COLUMN workspace_agents.version IS 'Version tracks the version of the currently running workspace agent. Workspace agents register their version upon start.';

COMMENT ON COLUMN workspace_agents.connection_timeout_seconds IS 'Connection timeout in seconds, 0 means disabled.';

COMMENT ON COLUMN workspace_agents.troubleshooting_url IS 'URL for troubleshooting the agent.';

COMMENT ON COLUMN workspace_agents.motd_file IS 'Path to file inside workspace containing the message of the day (MOTD) to show to the user when logging in via SSH.';

COMMENT ON COLUMN workspace_agents.lifecycle_state IS 'The current lifecycle state reported by the workspace agent.';

COMMENT ON COLUMN workspace_agents.login_before_ready IS 'If true, the agent will not prevent login before it is ready (e.g. startup script is still executing).';

COMMENT ON COLUMN workspace_agents.startup_script_timeout_seconds IS 'The number of seconds to wait for the startup script to complete. If the script does not complete within this time, the agent lifecycle will be marked as start_timeout.';

COMMENT ON COLUMN workspace_agents.expanded_directory IS 'The resolved path of a user-specified directory. e.g. ~/coder -> /home/coder/coder';

COMMENT ON COLUMN workspace_agents.shutdown_script IS 'Script that is executed before the agent is stopped.';

COMMENT ON COLUMN workspace_agents.shutdown_script_timeout_seconds IS 'The number of seconds to wait for the shutdown script to complete. If the script does not complete within this time, the agent lifecycle will be marked as shutdown_timeout.';

CREATE TABLE workspace_apps (
    id uuid NOT NULL,
    created_at timestamp with time zone NOT NULL,
    agent_id uuid NOT NULL,
    display_name character varying(64) NOT NULL,
    icon character varying(256) NOT NULL,
    command character varying(65534),
    url character varying(65534),
    healthcheck_url text DEFAULT ''::text NOT NULL,
    healthcheck_interval integer DEFAULT 0 NOT NULL,
    healthcheck_threshold integer DEFAULT 0 NOT NULL,
    health workspace_app_health DEFAULT 'disabled'::workspace_app_health NOT NULL,
    subdomain boolean DEFAULT false NOT NULL,
    sharing_level app_sharing_level DEFAULT 'owner'::app_sharing_level NOT NULL,
    slug text NOT NULL,
    external boolean DEFAULT false NOT NULL
);

CREATE TABLE workspace_build_parameters (
    workspace_build_id uuid NOT NULL,
    name text NOT NULL,
    value text NOT NULL
);

COMMENT ON COLUMN workspace_build_parameters.name IS 'Parameter name';

COMMENT ON COLUMN workspace_build_parameters.value IS 'Parameter value';

CREATE TABLE workspace_builds (
    id uuid NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    workspace_id uuid NOT NULL,
    template_version_id uuid NOT NULL,
    build_number integer NOT NULL,
    transition workspace_transition NOT NULL,
    initiator_id uuid NOT NULL,
    provisioner_state bytea,
    job_id uuid NOT NULL,
    deadline timestamp with time zone DEFAULT '0001-01-01 00:00:00+00'::timestamp with time zone NOT NULL,
    reason build_reason DEFAULT 'initiator'::build_reason NOT NULL,
    daily_cost integer DEFAULT 0 NOT NULL
);

CREATE TABLE workspace_resource_metadata (
    workspace_resource_id uuid NOT NULL,
    key character varying(1024) NOT NULL,
    value character varying(65536),
    sensitive boolean NOT NULL,
    id bigint NOT NULL
);

CREATE SEQUENCE workspace_resource_metadata_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE workspace_resource_metadata_id_seq OWNED BY workspace_resource_metadata.id;

CREATE TABLE workspace_resources (
    id uuid NOT NULL,
    created_at timestamp with time zone NOT NULL,
    job_id uuid NOT NULL,
    transition workspace_transition NOT NULL,
    type character varying(192) NOT NULL,
    name character varying(64) NOT NULL,
    hide boolean DEFAULT false NOT NULL,
    icon character varying(256) DEFAULT ''::character varying NOT NULL,
    instance_type character varying(256),
    daily_cost integer DEFAULT 0 NOT NULL
);

CREATE TABLE workspaces (
    id uuid NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    owner_id uuid NOT NULL,
    organization_id uuid NOT NULL,
    template_id uuid NOT NULL,
    deleted boolean DEFAULT false NOT NULL,
    name character varying(64) NOT NULL,
    autostart_schedule text,
    ttl bigint,
    last_used_at timestamp without time zone DEFAULT '0001-01-01 00:00:00'::timestamp without time zone NOT NULL
);

ALTER TABLE ONLY licenses ALTER COLUMN id SET DEFAULT nextval('licenses_id_seq'::regclass);

ALTER TABLE ONLY provisioner_job_logs ALTER COLUMN id SET DEFAULT nextval('provisioner_job_logs_id_seq'::regclass);

ALTER TABLE ONLY workspace_resource_metadata ALTER COLUMN id SET DEFAULT nextval('workspace_resource_metadata_id_seq'::regclass);

ALTER TABLE ONLY agent_stats
    ADD CONSTRAINT agent_stats_pkey PRIMARY KEY (id);

ALTER TABLE ONLY api_keys
    ADD CONSTRAINT api_keys_pkey PRIMARY KEY (id);

ALTER TABLE ONLY audit_logs
    ADD CONSTRAINT audit_logs_pkey PRIMARY KEY (id);

ALTER TABLE ONLY files
    ADD CONSTRAINT files_hash_created_by_key UNIQUE (hash, created_by);

ALTER TABLE ONLY files
    ADD CONSTRAINT files_pkey PRIMARY KEY (id);

ALTER TABLE ONLY git_auth_links
    ADD CONSTRAINT git_auth_links_provider_id_user_id_key UNIQUE (provider_id, user_id);

ALTER TABLE ONLY gitsshkeys
    ADD CONSTRAINT gitsshkeys_pkey PRIMARY KEY (user_id);

ALTER TABLE ONLY group_members
    ADD CONSTRAINT group_members_user_id_group_id_key UNIQUE (user_id, group_id);

ALTER TABLE ONLY groups
    ADD CONSTRAINT groups_name_organization_id_key UNIQUE (name, organization_id);

ALTER TABLE ONLY groups
    ADD CONSTRAINT groups_pkey PRIMARY KEY (id);

ALTER TABLE ONLY licenses
    ADD CONSTRAINT licenses_jwt_key UNIQUE (jwt);

ALTER TABLE ONLY licenses
    ADD CONSTRAINT licenses_pkey PRIMARY KEY (id);

ALTER TABLE ONLY organization_members
    ADD CONSTRAINT organization_members_pkey PRIMARY KEY (organization_id, user_id);

ALTER TABLE ONLY organizations
    ADD CONSTRAINT organizations_pkey PRIMARY KEY (id);

ALTER TABLE ONLY parameter_schemas
    ADD CONSTRAINT parameter_schemas_job_id_name_key UNIQUE (job_id, name);

ALTER TABLE ONLY parameter_schemas
    ADD CONSTRAINT parameter_schemas_pkey PRIMARY KEY (id);

ALTER TABLE ONLY parameter_values
    ADD CONSTRAINT parameter_values_pkey PRIMARY KEY (id);

ALTER TABLE ONLY parameter_values
    ADD CONSTRAINT parameter_values_scope_id_name_key UNIQUE (scope_id, name);

ALTER TABLE ONLY provisioner_daemons
    ADD CONSTRAINT provisioner_daemons_name_key UNIQUE (name);

ALTER TABLE ONLY provisioner_daemons
    ADD CONSTRAINT provisioner_daemons_pkey PRIMARY KEY (id);

ALTER TABLE ONLY provisioner_job_logs
    ADD CONSTRAINT provisioner_job_logs_pkey PRIMARY KEY (id);

ALTER TABLE ONLY provisioner_jobs
    ADD CONSTRAINT provisioner_jobs_pkey PRIMARY KEY (id);

ALTER TABLE ONLY site_configs
    ADD CONSTRAINT site_configs_key_key UNIQUE (key);

ALTER TABLE ONLY template_version_parameters
    ADD CONSTRAINT template_version_parameters_template_version_id_name_key UNIQUE (template_version_id, name);

ALTER TABLE ONLY template_version_variables
    ADD CONSTRAINT template_version_variables_template_version_id_name_key UNIQUE (template_version_id, name);

ALTER TABLE ONLY template_versions
    ADD CONSTRAINT template_versions_pkey PRIMARY KEY (id);

ALTER TABLE ONLY template_versions
    ADD CONSTRAINT template_versions_template_id_name_key UNIQUE (template_id, name);

ALTER TABLE ONLY templates
    ADD CONSTRAINT templates_pkey PRIMARY KEY (id);

ALTER TABLE ONLY user_links
    ADD CONSTRAINT user_links_pkey PRIMARY KEY (user_id, login_type);

ALTER TABLE ONLY users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);

ALTER TABLE ONLY workspace_agents
    ADD CONSTRAINT workspace_agents_pkey PRIMARY KEY (id);

ALTER TABLE ONLY workspace_apps
    ADD CONSTRAINT workspace_apps_agent_id_slug_idx UNIQUE (agent_id, slug);

ALTER TABLE ONLY workspace_apps
    ADD CONSTRAINT workspace_apps_pkey PRIMARY KEY (id);

ALTER TABLE ONLY workspace_build_parameters
    ADD CONSTRAINT workspace_build_parameters_workspace_build_id_name_key UNIQUE (workspace_build_id, name);

ALTER TABLE ONLY workspace_builds
    ADD CONSTRAINT workspace_builds_job_id_key UNIQUE (job_id);

ALTER TABLE ONLY workspace_builds
    ADD CONSTRAINT workspace_builds_pkey PRIMARY KEY (id);

ALTER TABLE ONLY workspace_builds
    ADD CONSTRAINT workspace_builds_workspace_id_build_number_key UNIQUE (workspace_id, build_number);

ALTER TABLE ONLY workspace_resource_metadata
    ADD CONSTRAINT workspace_resource_metadata_name UNIQUE (workspace_resource_id, key);

ALTER TABLE ONLY workspace_resource_metadata
    ADD CONSTRAINT workspace_resource_metadata_pkey PRIMARY KEY (id);

ALTER TABLE ONLY workspace_resources
    ADD CONSTRAINT workspace_resources_pkey PRIMARY KEY (id);

ALTER TABLE ONLY workspaces
    ADD CONSTRAINT workspaces_pkey PRIMARY KEY (id);

CREATE INDEX idx_agent_stats_created_at ON agent_stats USING btree (created_at);

CREATE INDEX idx_agent_stats_user_id ON agent_stats USING btree (user_id);

CREATE INDEX idx_api_keys_user ON api_keys USING btree (user_id);

CREATE INDEX idx_audit_log_organization_id ON audit_logs USING btree (organization_id);

CREATE INDEX idx_audit_log_resource_id ON audit_logs USING btree (resource_id);

CREATE INDEX idx_audit_log_user_id ON audit_logs USING btree (user_id);

CREATE INDEX idx_audit_logs_time_desc ON audit_logs USING btree ("time" DESC);

CREATE INDEX idx_organization_member_organization_id_uuid ON organization_members USING btree (organization_id);

CREATE INDEX idx_organization_member_user_id_uuid ON organization_members USING btree (user_id);

CREATE UNIQUE INDEX idx_organization_name ON organizations USING btree (name);

CREATE UNIQUE INDEX idx_organization_name_lower ON organizations USING btree (lower(name));

CREATE UNIQUE INDEX idx_users_email ON users USING btree (email) WHERE (deleted = false);

CREATE UNIQUE INDEX idx_users_username ON users USING btree (username) WHERE (deleted = false);

CREATE INDEX provisioner_job_logs_id_job_id_idx ON provisioner_job_logs USING btree (job_id, id);

CREATE INDEX provisioner_jobs_started_at_idx ON provisioner_jobs USING btree (started_at) WHERE (started_at IS NULL);

CREATE UNIQUE INDEX templates_organization_id_name_idx ON templates USING btree (organization_id, lower((name)::text)) WHERE (deleted = false);

CREATE UNIQUE INDEX users_email_lower_idx ON users USING btree (lower(email)) WHERE (deleted = false);

CREATE UNIQUE INDEX users_username_lower_idx ON users USING btree (lower(username)) WHERE (deleted = false);

CREATE INDEX workspace_agents_auth_token_idx ON workspace_agents USING btree (auth_token);

CREATE INDEX workspace_agents_resource_id_idx ON workspace_agents USING btree (resource_id);

CREATE INDEX workspace_resources_job_id_idx ON workspace_resources USING btree (job_id);

CREATE UNIQUE INDEX workspaces_owner_id_lower_idx ON workspaces USING btree (owner_id, lower((name)::text)) WHERE (deleted = false);

ALTER TABLE ONLY api_keys
    ADD CONSTRAINT api_keys_user_id_uuid_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE ONLY gitsshkeys
    ADD CONSTRAINT gitsshkeys_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id);

ALTER TABLE ONLY group_members
    ADD CONSTRAINT group_members_group_id_fkey FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE;

ALTER TABLE ONLY group_members
    ADD CONSTRAINT group_members_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE ONLY groups
    ADD CONSTRAINT groups_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE ONLY organization_members
    ADD CONSTRAINT organization_members_organization_id_uuid_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE ONLY organization_members
    ADD CONSTRAINT organization_members_user_id_uuid_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE ONLY parameter_schemas
    ADD CONSTRAINT parameter_schemas_job_id_fkey FOREIGN KEY (job_id) REFERENCES provisioner_jobs(id) ON DELETE CASCADE;

ALTER TABLE ONLY provisioner_job_logs
    ADD CONSTRAINT provisioner_job_logs_job_id_fkey FOREIGN KEY (job_id) REFERENCES provisioner_jobs(id) ON DELETE CASCADE;

ALTER TABLE ONLY provisioner_jobs
    ADD CONSTRAINT provisioner_jobs_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE ONLY template_version_parameters
    ADD CONSTRAINT template_version_parameters_template_version_id_fkey FOREIGN KEY (template_version_id) REFERENCES template_versions(id) ON DELETE CASCADE;

ALTER TABLE ONLY template_version_variables
    ADD CONSTRAINT template_version_variables_template_version_id_fkey FOREIGN KEY (template_version_id) REFERENCES template_versions(id) ON DELETE CASCADE;

ALTER TABLE ONLY template_versions
    ADD CONSTRAINT template_versions_created_by_fkey FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE RESTRICT;

ALTER TABLE ONLY template_versions
    ADD CONSTRAINT template_versions_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE ONLY template_versions
    ADD CONSTRAINT template_versions_template_id_fkey FOREIGN KEY (template_id) REFERENCES templates(id) ON DELETE CASCADE;

ALTER TABLE ONLY templates
    ADD CONSTRAINT templates_created_by_fkey FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE RESTRICT;

ALTER TABLE ONLY templates
    ADD CONSTRAINT templates_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE ONLY user_links
    ADD CONSTRAINT user_links_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE ONLY workspace_agents
    ADD CONSTRAINT workspace_agents_resource_id_fkey FOREIGN KEY (resource_id) REFERENCES workspace_resources(id) ON DELETE CASCADE;

ALTER TABLE ONLY workspace_apps
    ADD CONSTRAINT workspace_apps_agent_id_fkey FOREIGN KEY (agent_id) REFERENCES workspace_agents(id) ON DELETE CASCADE;

ALTER TABLE ONLY workspace_build_parameters
    ADD CONSTRAINT workspace_build_parameters_workspace_build_id_fkey FOREIGN KEY (workspace_build_id) REFERENCES workspace_builds(id) ON DELETE CASCADE;

ALTER TABLE ONLY workspace_builds
    ADD CONSTRAINT workspace_builds_job_id_fkey FOREIGN KEY (job_id) REFERENCES provisioner_jobs(id) ON DELETE CASCADE;

ALTER TABLE ONLY workspace_builds
    ADD CONSTRAINT workspace_builds_template_version_id_fkey FOREIGN KEY (template_version_id) REFERENCES template_versions(id) ON DELETE CASCADE;

ALTER TABLE ONLY workspace_builds
    ADD CONSTRAINT workspace_builds_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE;

ALTER TABLE ONLY workspace_resource_metadata
    ADD CONSTRAINT workspace_resource_metadata_workspace_resource_id_fkey FOREIGN KEY (workspace_resource_id) REFERENCES workspace_resources(id) ON DELETE CASCADE;

ALTER TABLE ONLY workspace_resources
    ADD CONSTRAINT workspace_resources_job_id_fkey FOREIGN KEY (job_id) REFERENCES provisioner_jobs(id) ON DELETE CASCADE;

ALTER TABLE ONLY workspaces
    ADD CONSTRAINT workspaces_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE RESTRICT;

ALTER TABLE ONLY workspaces
    ADD CONSTRAINT workspaces_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE RESTRICT;

ALTER TABLE ONLY workspaces
    ADD CONSTRAINT workspaces_template_id_fkey FOREIGN KEY (template_id) REFERENCES templates(id) ON DELETE RESTRICT;

