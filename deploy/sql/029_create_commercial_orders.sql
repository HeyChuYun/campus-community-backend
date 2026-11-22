-- Commercial monetization: ad campaign orders, membership orders and payment records.
-- Amounts are stored in cents: 30000 = CNY 300.00, 990 = CNY 9.90.

create table if not exists membership_plans (
    plan_code varchar(32) primary key,
    plan_name varchar(64) not null,
    monthly_amount_cents integer not null,
    yearly_amount_cents integer not null,
    features jsonb not null default '{}'::jsonb,
    is_active boolean not null default true,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint chk_membership_plans_amount
        check (monthly_amount_cents >= 0 and yearly_amount_cents >= 0)
);

create table if not exists membership_orders (
    id bigserial primary key,
    order_no varchar(64) not null unique,
    user_id bigint not null references users(id) on delete cascade,
    plan_code varchar(32) not null references membership_plans(plan_code),
    billing_cycle varchar(16) not null,
    amount_cents integer not null,
    pay_channel varchar(16) not null,
    pay_status smallint not null default 0,
    provider_trade_no varchar(128) not null default '',
    paid_at timestamptz null,
    closed_at timestamptz null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint chk_membership_orders_cycle check (billing_cycle in ('month', 'year')),
    constraint chk_membership_orders_channel check (pay_channel in ('alipay', 'wechat')),
    constraint chk_membership_orders_pay_status check (pay_status in (0, 1, 2, 3)),
    constraint chk_membership_orders_amount check (amount_cents >= 0)
);

comment on column membership_orders.pay_status is '0 pending, 1 paid, 2 closed, 3 refunded';

create table if not exists user_memberships (
    user_id bigint primary key references users(id) on delete cascade,
    plan_code varchar(32) not null references membership_plans(plan_code),
    starts_at timestamptz not null,
    ends_at timestamptz not null,
    source_order_no varchar(64) not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint chk_user_memberships_range check (ends_at > starts_at)
);

create table if not exists ad_campaign_orders (
    id bigserial primary key,
    order_no varchar(64) not null unique,
    user_id bigint not null references users(id) on delete cascade,
    post_id bigint not null references posts(id) on delete cascade,
    amount_cents integer not null default 30000,
    duration_days integer not null default 30,
    pay_channel varchar(16) not null,
    pay_status smallint not null default 0,
    review_status smallint not null default 0,
    provider_trade_no varchar(128) not null default '',
    paid_at timestamptz null,
    starts_at timestamptz null,
    ends_at timestamptz null,
    reviewer_user_id bigint null references users(id) on delete set null,
    review_note text not null default '',
    rejected_at timestamptz null,
    closed_at timestamptz null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint chk_ad_campaign_orders_channel check (pay_channel in ('alipay', 'wechat')),
    constraint chk_ad_campaign_orders_pay_status check (pay_status in (0, 1, 2, 3)),
    constraint chk_ad_campaign_orders_review_status check (review_status in (0, 1, 2, 3)),
    constraint chk_ad_campaign_orders_amount check (amount_cents >= 0),
    constraint chk_ad_campaign_orders_duration check (duration_days > 0)
);

comment on column ad_campaign_orders.pay_status is '0 pending, 1 paid, 2 closed, 3 refunded';
comment on column ad_campaign_orders.review_status is '0 pending_payment, 1 pending_review, 2 approved, 3 rejected';

create table if not exists payment_transactions (
    id bigserial primary key,
    order_no varchar(64) not null,
    biz_type varchar(32) not null,
    channel varchar(16) not null,
    provider_trade_no varchar(128) not null default '',
    amount_cents integer not null default 0,
    status varchar(32) not null default 'unknown',
    notify_payload jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now(),
    constraint chk_payment_transactions_biz_type check (biz_type in ('membership', 'ad')),
    constraint chk_payment_transactions_channel check (channel in ('alipay', 'wechat'))
);

create table if not exists ad_impressions (
    id bigserial primary key,
    campaign_order_no varchar(64) not null references ad_campaign_orders(order_no) on delete cascade,
    viewer_user_id bigint not null references users(id) on delete cascade,
    feed_type varchar(32) not null default 'discover',
    impression_at timestamptz not null default now()
);

create table if not exists ad_clicks (
    id bigserial primary key,
    campaign_order_no varchar(64) not null references ad_campaign_orders(order_no) on delete cascade,
    viewer_user_id bigint not null references users(id) on delete cascade,
    feed_type varchar(32) not null default 'discover',
    clicked_at timestamptz not null default now()
);

alter table ad_clicks
    add column if not exists feed_type varchar(32) not null default 'discover';

create table if not exists media_processing_jobs (
    id bigserial primary key,
    user_id bigint not null references users(id) on delete cascade,
    storage_key text not null,
    media_type varchar(16) not null,
    content_type text not null default '',
    size_bytes bigint not null default 0,
    priority smallint not null default 0,
    status smallint not null default 0,
    error_message text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    processed_at timestamptz null,
    constraint chk_media_processing_jobs_priority check (priority in (0, 10)),
    constraint chk_media_processing_jobs_status check (status in (0, 1, 2, 3))
);

comment on column media_processing_jobs.priority is '0 normal, 10 high';
comment on column media_processing_jobs.status is '0 pending, 1 processing, 2 done, 3 failed';

create index if not exists idx_membership_orders_user_created
    on membership_orders (user_id, created_at desc);
create index if not exists idx_membership_orders_status_created
    on membership_orders (pay_status, created_at desc);
create index if not exists idx_user_memberships_plan_ends
    on user_memberships (plan_code, ends_at desc);
create index if not exists idx_ad_campaign_orders_user_created
    on ad_campaign_orders (user_id, created_at desc);
create index if not exists idx_ad_campaign_orders_review_created
    on ad_campaign_orders (review_status, created_at desc);
create index if not exists idx_ad_campaign_orders_active
    on ad_campaign_orders (starts_at, ends_at)
    where pay_status = 1 and review_status = 2;
create index if not exists idx_ad_impressions_viewer_time
    on ad_impressions (viewer_user_id, impression_at desc);
create index if not exists idx_ad_clicks_viewer_time
    on ad_clicks (viewer_user_id, clicked_at desc);
create index if not exists idx_media_processing_jobs_pending
    on media_processing_jobs (priority desc, created_at asc)
    where status = 0;

insert into membership_plans (plan_code, plan_name, monthly_amount_cents, yearly_amount_cents, features, is_active, created_at, updated_at)
values
    ('Premium', '青衿', 990, 9900, '{
        "ad_policy": "light_reduction",
        "daily_post_bonus": 3,
        "image_count_bonus": 3,
        "single_image_bytes_bonus": 20971520,
        "single_video_bytes_bonus": 41943040,
        "dynamic_badge": true,
        "scheduled_posts": true
    }'::jsonb, true, now(), now()),
    ('PremiumPro', '鸿儒', 1990, 19900, '{
        "ad_policy": "no_feed_ads",
        "daily_post_bonus": 12,
        "image_count_bonus": 12,
        "single_image_bytes_bonus": 62914560,
        "single_video_bytes_bonus": 209715200,
        "dynamic_badge": true,
        "scheduled_posts": true,
        "media_queue_priority": "high"
    }'::jsonb, true, now(), now())
on conflict (plan_code) do update set
    plan_name = excluded.plan_name,
    monthly_amount_cents = excluded.monthly_amount_cents,
    yearly_amount_cents = excluded.yearly_amount_cents,
    features = excluded.features,
    is_active = excluded.is_active,
    updated_at = now();

insert into admin_roles (role_code, role_name, is_system, created_at, updated_at)
values
    ('Premium', '青衿', true, now(), now()),
    ('PremiumPro', '鸿儒', true, now(), now())
on conflict (role_code) do update set
    role_name = excluded.role_name,
    is_system = true,
    updated_at = now();

insert into casbin_rule (ptype, v0, v1, v2)
select 'p', 'Premium', 'membership.premium', 'active'
where not exists (
    select 1 from casbin_rule where ptype = 'p' and v0 = 'Premium' and v1 = 'membership.premium' and v2 = 'active'
);
insert into casbin_rule (ptype, v0, v1, v2)
select 'p', 'Premium', 'monetization.ads', 'reduced'
where not exists (
    select 1 from casbin_rule where ptype = 'p' and v0 = 'Premium' and v1 = 'monetization.ads' and v2 = 'reduced'
);
insert into casbin_rule (ptype, v0, v1, v2)
select 'p', 'PremiumPro', 'membership.premium_pro', 'active'
where not exists (
    select 1 from casbin_rule where ptype = 'p' and v0 = 'PremiumPro' and v1 = 'membership.premium_pro' and v2 = 'active'
);
insert into casbin_rule (ptype, v0, v1, v2)
select 'p', 'PremiumPro', 'monetization.ads', 'free'
where not exists (
    select 1 from casbin_rule where ptype = 'p' and v0 = 'PremiumPro' and v1 = 'monetization.ads' and v2 = 'free'
);
insert into casbin_rule (ptype, v0, v1, v2)
select 'p', 'admin', 'admin.ad', 'review'
where not exists (
    select 1 from casbin_rule where ptype = 'p' and v0 = 'admin' and v1 = 'admin.ad' and v2 = 'review'
);
insert into casbin_rule (ptype, v0, v1, v2)
select 'p', 'admin', 'admin.membership', 'read'
where not exists (
    select 1 from casbin_rule where ptype = 'p' and v0 = 'admin' and v1 = 'admin.membership' and v2 = 'read'
);
insert into casbin_rule (ptype, v0, v1, v2)
select 'p', 'admin', 'admin.membership', 'manage'
where not exists (
    select 1 from casbin_rule where ptype = 'p' and v0 = 'admin' and v1 = 'admin.membership' and v2 = 'manage'
);
