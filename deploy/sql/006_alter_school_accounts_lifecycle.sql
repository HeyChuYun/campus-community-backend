-- 迁移 school_accounts：
-- 1. 删除 last_authorized_at / last_login_at
-- 2. 新增 admission_at / graduation_at
-- 3. 将 password 置为默认空串，明确后端不再保存校园密码

alter table if exists school_accounts
    add column if not exists admission_at timestamptz null;

alter table if exists school_accounts
    add column if not exists graduation_at timestamptz null;

alter table if exists school_accounts
    alter column password set default '';

alter table if exists school_accounts
    drop column if exists last_authorized_at;

alter table if exists school_accounts
    drop column if exists last_login_at;

create index if not exists idx_school_accounts_admission_at
    on school_accounts (admission_at);

create index if not exists idx_school_accounts_graduation_at
    on school_accounts (graduation_at);
