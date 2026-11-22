-- casbin_rule：Casbin 策略表
-- 说明：
-- 1. p 规则保存角色 -> 权限策略。
-- 2. g 规则保存用户 -> 角色关系。
-- 3. passport-service 启动时会自动补默认策略。

create table if not exists casbin_rule (
    id bigserial primary key,
    ptype varchar(100) not null default '',
    v0 varchar(100) not null default '',
    v1 varchar(100) not null default '',
    v2 varchar(100) not null default '',
    v3 varchar(100) not null default '',
    v4 varchar(100) not null default '',
    v5 varchar(100) not null default ''
);

comment on table casbin_rule is 'Casbin 策略规则表';
comment on column casbin_rule.ptype is '策略类型，例如 p 或 g';
comment on column casbin_rule.v0 is 'Casbin 规则字段 v0';
comment on column casbin_rule.v1 is 'Casbin 规则字段 v1';
comment on column casbin_rule.v2 is 'Casbin 规则字段 v2';
comment on column casbin_rule.v3 is 'Casbin 规则字段 v3';
comment on column casbin_rule.v4 is 'Casbin 规则字段 v4';
comment on column casbin_rule.v5 is 'Casbin 规则字段 v5';

create index if not exists idx_casbin_rule_ptype on casbin_rule (ptype);
create index if not exists idx_casbin_rule_v0 on casbin_rule (v0);
create index if not exists idx_casbin_rule_v1 on casbin_rule (v1);
create index if not exists idx_casbin_rule_v2 on casbin_rule (v2);
