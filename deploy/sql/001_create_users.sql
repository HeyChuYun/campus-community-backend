-- users：平台用户主表
-- 说明：
-- 1. account 是平台登录账号，不再使用学号。
-- 2. nickname 是展示昵称，可以和 account 分开修改。
-- 3. password_hash 存平台密码哈希，禁止存明文密码。
-- 4. bio 是个人简介，默认空字符串。

create table if not exists users (
    id bigserial primary key,
    account varchar(10) not null,
    nickname varchar(64) not null,
    password_hash varchar(255) not null,
    avatar_url text not null default '',
    cover_image_url text not null default '',
    bio text not null default '',
    account_status smallint not null default 1,
    school_bound boolean not null default false,
    last_login_at timestamptz null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    deleted_at timestamptz null,
    constraint chk_users_account_status check (account_status in (1, 2, 3, 4)),
    constraint chk_users_account_length check (char_length(account) between 1 and 10),
    constraint chk_users_account_not_digits check (account !~ '^[0-9]+$'),
    constraint chk_users_account_charset check (account ~ '^[A-Za-z0-9_一-龥]+$')
);

comment on table users is '平台用户主表';
comment on column users.id is '平台用户 ID，主键';
comment on column users.account is '平台登录账号，只允许字母、数字、下划线、中文，不能全数字，最长 10 个字符';
comment on column users.nickname is '用户展示昵称';
comment on column users.password_hash is '平台密码哈希值，禁止存明文';
comment on column users.avatar_url is '头像地址';
comment on column users.cover_image_url is '个人主页展示图地址';
comment on column users.bio is '个人简介';
comment on column users.account_status is '账号状态：1正常 2只读 3封禁 4已注销';
comment on column users.school_bound is '是否已完成校园账号绑定';
comment on column users.last_login_at is '最近一次成功登录时间';
comment on column users.created_at is '创建时间';
comment on column users.updated_at is '更新时间';
comment on column users.deleted_at is '软删除时间，为空表示未删除';

create unique index if not exists uk_users_account_active
    on users (account)
    where deleted_at is null;

create unique index if not exists uk_users_nickname_active
    on users (nickname)
    where deleted_at is null;

create index if not exists idx_users_status
    on users (account_status);

create index if not exists idx_users_last_login_at
    on users (last_login_at);
