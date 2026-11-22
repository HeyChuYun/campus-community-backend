alter table posts
    add column if not exists campus_code varchar(32) not null default 'all';

comment on column posts.campus_code is 'post campus scope: all, xinqiao, gaoji';

create index if not exists idx_posts_campus_status_created_at
    on posts (campus_code, status, created_at desc)
    where deleted_at is null;

create table if not exists post_moderation_tasks (
    id bigserial primary key,
    post_id bigint not null unique references posts(id) on delete cascade,
    provider varchar(32) not null default 'baidu',
    status varchar(24) not null default 'pending',
    provider_task_id varchar(128) not null default '',
    sync_result_payload jsonb not null default '{}'::jsonb,
    async_result_payload jsonb not null default '{}'::jsonb,
    final_result_payload jsonb not null default '{}'::jsonb,
    conclusion varchar(32) not null default '',
    categories jsonb not null default '[]'::jsonb,
    reason text not null default '',
    attempts integer not null default 0,
    next_retry_at timestamptz null,
    last_error text not null default '',
    last_checked_at timestamptz null,
    reviewed_at timestamptz null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint chk_post_moderation_tasks_status
        check (status in ('pending', 'running', 'waiting_async', 'passed', 'rejected', 'failed'))
);

create index if not exists idx_post_moderation_tasks_status_retry
    on post_moderation_tasks (status, next_retry_at asc, created_at asc);

create table if not exists content_violation_records (
    id bigserial primary key,
    violation_no varchar(64) not null unique,
    user_id bigint not null references users(id) on delete cascade,
    post_id bigint not null references posts(id) on delete cascade,
    source varchar(32) not null,
    action_type varchar(24) not null default 'block_post',
    categories jsonb not null default '[]'::jsonb,
    reason text not null default '',
    evidence_payload jsonb not null default '{}'::jsonb,
    mute_scope varchar(32) not null default 'post_publish',
    mute_duration_seconds integer not null default 1800,
    operator_user_id bigint null references users(id) on delete set null,
    created_at timestamptz not null default now(),
    constraint chk_content_violation_records_source
        check (source in ('baidu_auto', 'admin_manual')),
    constraint chk_content_violation_records_action
        check (action_type in ('block_post')),
    constraint chk_content_violation_records_mute_scope
        check (mute_scope in ('post_publish', 'direct_message')),
    constraint chk_content_violation_records_mute_duration
        check (mute_duration_seconds >= 0)
);

create index if not exists idx_content_violation_records_user_created
    on content_violation_records (user_id, created_at desc);

create index if not exists idx_content_violation_records_post_created
    on content_violation_records (post_id, created_at desc);

create table if not exists user_restrictions (
    id bigserial primary key,
    restriction_no varchar(64) not null unique,
    user_id bigint not null references users(id) on delete cascade,
    scope varchar(32) not null,
    status varchar(16) not null default 'active',
    source varchar(32) not null,
    reason text not null default '',
    related_violation_id bigint null references content_violation_records(id) on delete set null,
    operator_user_id bigint null references users(id) on delete set null,
    starts_at timestamptz not null default now(),
    ends_at timestamptz null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint chk_user_restrictions_scope
        check (scope in ('post_publish', 'direct_message')),
    constraint chk_user_restrictions_status
        check (status in ('active', 'revoked', 'expired'))
);

create index if not exists idx_user_restrictions_user_scope_status
    on user_restrictions (user_id, scope, status, ends_at desc);

create index if not exists idx_user_restrictions_active_scope
    on user_restrictions (scope, status, ends_at asc);
