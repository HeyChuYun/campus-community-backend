-- im-service 基础表

create table if not exists im_user_bindings (
    id bigserial primary key,
    user_id bigint not null,
    im_uid varchar(64) not null,
    status varchar(16) not null default 'active',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint fk_im_user_bindings_user_id
        foreign key (user_id) references users(id) on delete cascade
);

comment on table im_user_bindings is '平台用户和 WuKongIM 用户映射表';

create unique index if not exists uk_im_user_bindings_user_id
    on im_user_bindings (user_id);

create unique index if not exists uk_im_user_bindings_im_uid
    on im_user_bindings (im_uid);

create table if not exists im_dm_quotas (
    id bigserial primary key,
    sender_user_id bigint not null,
    receiver_user_id bigint not null,
    used_count integer not null default 0,
    limit_count integer not null default 3,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint fk_im_dm_quotas_sender
        foreign key (sender_user_id) references users(id) on delete cascade,
    constraint fk_im_dm_quotas_receiver
        foreign key (receiver_user_id) references users(id) on delete cascade,
    constraint chk_im_dm_quotas_used_count
        check (used_count >= 0),
    constraint chk_im_dm_quotas_limit_count
        check (limit_count >= 0)
);

comment on table im_dm_quotas is '未互关用户之间的定向私信额度表';

create unique index if not exists uk_im_dm_quotas_pair
    on im_dm_quotas (sender_user_id, receiver_user_id);

create table if not exists im_message_audits (
    id bigserial primary key,
    message_id_in_wk bigint not null default 0,
    message_seq bigint not null default 0,
    client_msg_no varchar(128) not null default '',
    conversation_type smallint not null,
    sender_user_id bigint not null,
    receiver_user_id bigint not null,
    biz_type smallint not null,
    content_preview varchar(255) not null default '',
    status varchar(16) not null default 'sent',
    created_at timestamptz not null default now(),
    constraint fk_im_message_audits_sender
        foreign key (sender_user_id) references users(id) on delete cascade,
    constraint fk_im_message_audits_receiver
        foreign key (receiver_user_id) references users(id) on delete cascade
);

comment on table im_message_audits is '私信/互动消息审计表';

create index if not exists idx_im_message_audits_sender_created_at
    on im_message_audits (sender_user_id, created_at desc);

create index if not exists idx_im_message_audits_receiver_created_at
    on im_message_audits (receiver_user_id, created_at desc);

create table if not exists im_interaction_messages (
    id bigserial primary key,
    user_id bigint not null,
    event_id varchar(64) not null,
    event_type smallint not null,
    operator_user_id bigint not null default 0,
    title varchar(128) not null default '',
    content varchar(255) not null default '',
    resource_type varchar(32) not null default '',
    resource_id bigint not null default 0,
    is_read boolean not null default false,
    created_at timestamptz not null default now(),
    constraint fk_im_interaction_messages_user_id
        foreign key (user_id) references users(id) on delete cascade
);

comment on table im_interaction_messages is '互动消息落库表';

create unique index if not exists uk_im_interaction_messages_user_event
    on im_interaction_messages (user_id, event_id);

create index if not exists idx_im_interaction_messages_user_read_created_at
    on im_interaction_messages (user_id, is_read, created_at desc);

create table if not exists im_event_consume_logs (
    id bigserial primary key,
    event_id varchar(64) not null,
    consumer varchar(64) not null,
    status varchar(16) not null default 'done',
    consumed_at timestamptz not null default now()
);

comment on table im_event_consume_logs is 'im-service MQ 事件幂等消费日志';

create unique index if not exists uk_im_event_consume_logs_event_consumer
    on im_event_consume_logs (event_id, consumer);
