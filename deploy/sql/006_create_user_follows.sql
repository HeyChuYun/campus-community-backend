-- user_follows: user follow relations
-- status:
-- 1 = active
-- 2 = cancelled
create table if not exists user_follows (
    id bigserial primary key,
    follower_user_id bigint not null,
    followee_user_id bigint not null,
    status smallint not null default 1,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint fk_user_follows_follower
        foreign key (follower_user_id) references users(id) on delete cascade,
    constraint fk_user_follows_followee
        foreign key (followee_user_id) references users(id) on delete cascade,
    constraint chk_user_follows_status
        check (status in (1, 2)),
    constraint chk_user_follows_not_self
        check (follower_user_id <> followee_user_id)
);

comment on table user_follows is 'follow relations between users';
comment on column user_follows.follower_user_id is 'who follows';
comment on column user_follows.followee_user_id is 'who is followed';
comment on column user_follows.status is '1 active, 2 cancelled';

create unique index if not exists uk_user_follows_pair
    on user_follows (follower_user_id, followee_user_id);

create index if not exists idx_user_follows_followee_status
    on user_follows (followee_user_id, status);

create index if not exists idx_user_follows_follower_status
    on user_follows (follower_user_id, status);
