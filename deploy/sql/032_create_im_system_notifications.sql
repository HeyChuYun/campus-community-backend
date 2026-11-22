create table if not exists im_system_notifications (
    id bigserial primary key,
    user_id bigint not null,
    event_id varchar(128) not null,
    notice_type varchar(64) not null default '',
    title varchar(128) not null default '',
    content text not null default '',
    resource_type varchar(32) not null default '',
    resource_id bigint not null default 0,
    media_payload text not null default '',
    is_read boolean not null default false,
    created_at timestamptz not null default now(),
    constraint fk_im_system_notifications_user_id
        foreign key (user_id) references users(id) on delete cascade
);

comment on table im_system_notifications is '系统通知落库表';

create unique index if not exists uk_im_system_notifications_user_event
    on im_system_notifications (user_id, event_id);

create index if not exists idx_im_system_notifications_user_read_created_at
    on im_system_notifications (user_id, is_read, created_at desc);
