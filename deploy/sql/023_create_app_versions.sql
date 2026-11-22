create table if not exists app_versions (
    id bigserial primary key,
    version_name varchar(32) not null,
    version_code integer not null default 0,
    platform varchar(16) not null default 'android',
    download_url text not null default '',
    app_store_url text not null default '',
    force_update boolean not null default false,
    min_supported_version varchar(32) not null default '',
    update_log text not null default '',
    is_active boolean not null default true,
    created_by varchar(64) not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create unique index if not exists uk_app_versions_platform_version
    on app_versions (platform, version_name);

create index if not exists idx_app_versions_platform_active_created
    on app_versions (platform, is_active, created_at desc);
