-- school_accounts：学校侧账号绑定表
-- 说明：
-- 1. 一条记录表示一个平台用户绑定的一个学校账号。
-- 2. password 字段当前仅保留兼容结构，后端不再保存校园密码，默认写空串。
-- 3. admission_at / graduation_at 用于后续定时任务按时间纠偏 student_status。
create table if not exists school_accounts (
    id bigserial primary key,
    user_id bigint not null,
    student_no varchar(64) not null,
    student_name varchar(64) not null default '',
    password text not null default '',
    student_status smallint not null default 0,
    bind_status smallint not null default 1,
    admission_at timestamptz null,
    graduation_at timestamptz null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint fk_school_accounts_user_id
        foreign key (user_id) references users(id) on delete cascade,
    constraint chk_school_accounts_student_status
        check (student_status in (0, 1, 2, 3)),
    constraint chk_school_accounts_bind_status
        check (bind_status in (1, 2, 3))
);

comment on table school_accounts is '学校账号绑定表';
comment on column school_accounts.id is '主键';
comment on column school_accounts.user_id is '平台用户 ID，外键关联 users.id';
comment on column school_accounts.student_no is '学号';
comment on column school_accounts.student_name is '学校侧学生姓名';
comment on column school_accounts.password is '兼容保留字段，当前后端不存校园密码，默认空串';
comment on column school_accounts.student_status is '学生状态：0未知 1在校 2毕业 3休学/停学';
comment on column school_accounts.bind_status is '绑定状态：1正常绑定 2已解绑 3已失效';
comment on column school_accounts.admission_at is '入学时间';
comment on column school_accounts.graduation_at is '毕业时间或预计毕业时间';
comment on column school_accounts.created_at is '创建时间';
comment on column school_accounts.updated_at is '更新时间';

create unique index if not exists uk_school_accounts_user_id
    on school_accounts (user_id);

create unique index if not exists uk_school_accounts_student_no
    on school_accounts (student_no);

create index if not exists idx_school_accounts_bind_status
    on school_accounts (bind_status);

create index if not exists idx_school_accounts_student_status
    on school_accounts (student_status);

create index if not exists idx_school_accounts_admission_at
    on school_accounts (admission_at);

create index if not exists idx_school_accounts_graduation_at
    on school_accounts (graduation_at);
