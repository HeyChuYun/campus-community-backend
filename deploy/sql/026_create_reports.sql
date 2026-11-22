create table if not exists report_settings (
    setting_key varchar(100) primary key,
    setting_value varchar(255) not null default '',
    updated_at timestamptz not null default now()
);

create table if not exists reports (
    id bigserial primary key,
    report_no varchar(40) not null,
    reporter_user_id bigint not null,
    target_type varchar(20) not null,
    target_user_id bigint null,
    target_post_id bigint null,
    category_code varchar(50) not null,
    reason_text text not null,
    status varchar(40) not null default 'pending',
    target_snapshot_json jsonb not null default '{}'::jsonb,
    latest_decision_code varchar(50) not null default '',
    latest_decision_note text not null default '',
    assigned_to_user_id bigint null,
    closed_at timestamptz null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint fk_reports_reporter_user
        foreign key (reporter_user_id) references users(id) on delete cascade,
    constraint fk_reports_target_user
        foreign key (target_user_id) references users(id) on delete set null,
    constraint chk_reports_target_type
        check (target_type in ('user', 'post'))
);

create unique index if not exists uk_reports_report_no on reports (report_no);
create index if not exists idx_reports_reporter_created_at on reports (reporter_user_id, created_at desc);
create index if not exists idx_reports_target_user_status on reports (target_user_id, status, created_at desc);

create table if not exists report_evidences (
    id bigserial primary key,
    report_id bigint not null,
    media_type varchar(20) not null default 'image',
    file_url text not null,
    file_size bigint not null default 0,
    width integer not null default 0,
    height integer not null default 0,
    sort_order integer not null default 0,
    created_at timestamptz not null default now(),
    constraint fk_report_evidences_report
        foreign key (report_id) references reports(id) on delete cascade
);

create index if not exists idx_report_evidences_report_id on report_evidences (report_id, sort_order asc);

create table if not exists report_actions (
    id bigserial primary key,
    report_id bigint not null,
    operator_user_id bigint not null,
    action_type varchar(40) not null,
    from_status varchar(40) not null default '',
    to_status varchar(40) not null default '',
    note text not null default '',
    extra_json jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now(),
    constraint fk_report_actions_report
        foreign key (report_id) references reports(id) on delete cascade,
    constraint fk_report_actions_operator_user
        foreign key (operator_user_id) references users(id) on delete cascade
);

create index if not exists idx_report_actions_report_id on report_actions (report_id, created_at asc);

insert into report_settings (setting_key, setting_value, updated_at)
values
    ('daily_report_target_limit', '3', now()),
    ('same_target_cooldown_days', '7', now()),
    ('min_reason_length', '15', now()),
    ('max_reason_length', '500', now()),
    ('max_evidence_count', '6', now()),
    ('max_evidence_size_bytes', '8388608', now())
on conflict (setting_key) do nothing;
