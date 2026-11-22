-- user_role_change_logs：用户角色变更审计表
-- 说明：
-- 1. 每次管理员为用户重设角色时写入一条审计记录。
-- 2. before_roles / after_roles 使用 jsonb，便于保留一次变更前后的完整角色集合。
-- 3. operator_user_id 和 target_user_id 都加外键约束，用户删除后同步删除相关审计记录。
create table if not exists user_role_change_logs (
    id bigserial primary key,
    operator_user_id bigint not null,
    target_user_id bigint not null,
    before_roles jsonb not null default '[]'::jsonb,
    after_roles jsonb not null default '[]'::jsonb,
    created_at timestamptz not null default now(),
    constraint fk_user_role_change_logs_operator
        foreign key (operator_user_id) references users(id) on delete cascade,
    constraint fk_user_role_change_logs_target
        foreign key (target_user_id) references users(id) on delete cascade
);

comment on table user_role_change_logs is '用户角色变更审计表';
comment on column user_role_change_logs.operator_user_id is '执行角色变更的管理员用户 ID';
comment on column user_role_change_logs.target_user_id is '被修改角色的目标用户 ID';
comment on column user_role_change_logs.before_roles is '变更前角色集合';
comment on column user_role_change_logs.after_roles is '变更后角色集合';
comment on column user_role_change_logs.created_at is '角色变更时间';

create index if not exists idx_user_role_change_logs_operator_user_id
    on user_role_change_logs (operator_user_id);

create index if not exists idx_user_role_change_logs_target_user_id
    on user_role_change_logs (target_user_id);

create index if not exists idx_user_role_change_logs_created_at
    on user_role_change_logs (created_at);
