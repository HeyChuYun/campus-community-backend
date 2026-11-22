create table if not exists im_direct_read_cursors (
    id bigserial primary key,
    user_id bigint not null,
    peer_user_id bigint not null,
    last_read_seq bigint not null default 0,
    last_read_at timestamptz not null default to_timestamp(0),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint fk_im_direct_read_cursors_user
        foreign key (user_id) references users(id) on delete cascade,
    constraint fk_im_direct_read_cursors_peer
        foreign key (peer_user_id) references users(id) on delete cascade
);

comment on table im_direct_read_cursors is '私聊会话读指针表';

create unique index if not exists uk_im_direct_read_cursors_pair
    on im_direct_read_cursors (user_id, peer_user_id);

create index if not exists idx_im_direct_read_cursors_user_updated_at
    on im_direct_read_cursors (user_id, updated_at desc);
