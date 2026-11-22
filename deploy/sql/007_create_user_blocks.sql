-- user_blocks: user block relations
create table if not exists user_blocks (
    id bigserial primary key,
    user_id bigint not null,
    blocked_user_id bigint not null,
    created_at timestamptz not null default now(),
    constraint fk_user_blocks_user
        foreign key (user_id) references users(id) on delete cascade,
    constraint fk_user_blocks_blocked_user
        foreign key (blocked_user_id) references users(id) on delete cascade,
    constraint chk_user_blocks_not_self
        check (user_id <> blocked_user_id)
);

comment on table user_blocks is 'block relations between users';
comment on column user_blocks.user_id is 'who blocks';
comment on column user_blocks.blocked_user_id is 'who is blocked';

create unique index if not exists uk_user_blocks_pair
    on user_blocks (user_id, blocked_user_id);

create index if not exists idx_user_blocks_user_id
    on user_blocks (user_id);

create index if not exists idx_user_blocks_blocked_user_id
    on user_blocks (blocked_user_id);
