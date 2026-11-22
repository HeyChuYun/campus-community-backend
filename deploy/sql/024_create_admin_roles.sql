-- admin_roles：管理后台角色元数据表
-- 说明：
-- 1. 角色权限仍写入 casbin_rule：p 规则保存角色权限，g 规则保存用户角色。
-- 2. 本表只保存角色编码、展示名称和系统角色标记，便于管理后台创建空角色和展示角色名。

create table if not exists admin_roles (
    role_code varchar(100) primary key,
    role_name varchar(100) not null default '',
    is_system boolean not null default false,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

comment on table admin_roles is '管理后台角色元数据表';
comment on column admin_roles.role_code is '角色编码，对应 casbin_rule 中的角色 subject';
comment on column admin_roles.role_name is '角色展示名称';
comment on column admin_roles.is_system is '是否系统内置角色，系统角色不允许删除';
comment on column admin_roles.created_at is '创建时间';
comment on column admin_roles.updated_at is '更新时间';

create index if not exists idx_admin_roles_is_system on admin_roles (is_system);

insert into admin_roles (role_code, role_name, is_system, created_at, updated_at)
values ('user', '普通用户', true, now(), now()),
       ('admin', '管理员', true, now(), now())
on conflict (role_code) do update set
    role_name = excluded.role_name,
    is_system = true,
    updated_at = now();
