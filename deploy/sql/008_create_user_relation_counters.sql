-- user_relation_counters: cached counters for relation-service
create table if not exists user_relation_counters (
    user_id bigint primary key,
    following_count bigint not null default 0,
    follower_count bigint not null default 0,
    friend_count bigint not null default 0,
    blocked_count bigint not null default 0,
    updated_at timestamptz not null default now(),
    constraint fk_user_relation_counters_user
        foreign key (user_id) references users(id) on delete cascade,
    constraint chk_user_relation_counters_non_negative
        check (
            following_count >= 0 and
            follower_count >= 0 and
            friend_count >= 0 and
            blocked_count >= 0
        )
);

comment on table user_relation_counters is 'cached relation counters for each user';
