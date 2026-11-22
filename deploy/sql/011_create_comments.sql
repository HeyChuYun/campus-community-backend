-- comments: post comments
-- status:
-- 1 = normal
-- 2 = blocked
-- 3 = deleted
create table if not exists comments (
    id bigserial primary key,
    post_id bigint not null,
    user_id bigint not null,
    reply_to_comment_id bigint null,
    reply_to_user_id bigint null,
    content text not null,
    status smallint not null default 1,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    deleted_at timestamptz null,
    constraint fk_comments_post_id
        foreign key (post_id) references posts(id) on delete cascade,
    constraint fk_comments_user_id
        foreign key (user_id) references users(id) on delete cascade,
    constraint fk_comments_reply_to_comment_id
        foreign key (reply_to_comment_id) references comments(id) on delete set null,
    constraint fk_comments_reply_to_user_id
        foreign key (reply_to_user_id) references users(id) on delete set null,
    constraint chk_comments_status
        check (status in (1, 2, 3))
);

comment on table comments is 'post comments';

create index if not exists idx_comments_post_created_at
    on comments (post_id, created_at asc)
    where deleted_at is null;

create index if not exists idx_comments_user_id
    on comments (user_id);

create index if not exists idx_comments_reply_to_comment_id
    on comments (reply_to_comment_id);
