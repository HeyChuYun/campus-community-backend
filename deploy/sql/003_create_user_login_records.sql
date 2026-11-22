-- user_login_records：平台账号登录记录表
-- 说明：
-- 1. 每次平台登录成功后写入一条记录。
-- 2. user_id 使用外键约束，用户被物理删除时同步删除登录记录。
-- 3. login_ip 由 gateway 从连接/代理头提取，device 由 gateway 从 HTTP 请求头提取，再通过 gRPC metadata 传给 passport-service。

create table if not exists user_login_records (
    id bigserial primary key,
    user_id bigint not null,
    login_ip varchar(64) not null default '',
    device text not null default '',
    login_at timestamptz not null default now(),
    constraint fk_user_login_records_user_id
        foreign key (user_id) references users(id) on delete cascade
);

comment on table user_login_records is '平台账号登录记录表';
comment on column user_login_records.id is '登录记录 ID，主键';
comment on column user_login_records.user_id is '平台用户 ID，外键关联 users.id';
comment on column user_login_records.login_ip is '登录 IP';
comment on column user_login_records.device is '登录设备信息，通常来自 User-Agent 或客户端设备标识';
comment on column user_login_records.login_at is '登录时间';

create index if not exists idx_user_login_records_user_id
    on user_login_records (user_id);

create index if not exists idx_user_login_records_login_at
    on user_login_records (login_at);
