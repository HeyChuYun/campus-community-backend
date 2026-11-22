create table if not exists membership_admin_operations (
    id bigserial primary key,
    operation_no varchar(64) not null unique,
    user_id bigint not null references users(id) on delete cascade,
    operator_user_id bigint not null references users(id) on delete restrict,
    action varchar(16) not null,
    plan_code varchar(32) not null default '',
    duration_value integer not null default 0,
    duration_unit varchar(16) not null default '',
    previous_plan_code varchar(32) not null default '',
    previous_starts_at timestamptz null,
    previous_ends_at timestamptz null,
    new_starts_at timestamptz null,
    new_ends_at timestamptz null,
    reason text not null default '',
    created_at timestamptz not null default now(),
    constraint chk_membership_admin_operations_action
        check (action in ('grant', 'extend', 'revoke')),
    constraint chk_membership_admin_operations_duration_value
        check (duration_value >= 0),
    constraint chk_membership_admin_operations_duration_unit
        check (duration_unit in ('', 'day', 'month', 'year'))
);

create index if not exists idx_membership_admin_operations_user_created
    on membership_admin_operations (user_id, created_at desc);

create index if not exists idx_membership_admin_operations_operator_created
    on membership_admin_operations (operator_user_id, created_at desc);

comment on table membership_admin_operations is '后台会员发放、续期、撤销操作审计表';
comment on column membership_admin_operations.operation_no is '后台会员操作单号';
comment on column membership_admin_operations.action is '操作类型：grant/extend/revoke';
comment on column membership_admin_operations.plan_code is '操作后的目标会员方案';
comment on column membership_admin_operations.reason is '后台操作原因';
