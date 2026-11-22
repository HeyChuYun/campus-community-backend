-- posts: content-service main post table
-- content_type:
-- 1 = text
-- 2 = image
-- 3 = video
-- 4 = gif
-- visibility:
-- 1 = public
-- 2 = followers_only
-- status:
-- 1 = normal
-- 2 = reviewing
-- 3 = blocked
-- 4 = deleted
create table if not exists posts (
    id bigserial primary key,
    author_user_id bigint not null,
    content_text text not null default '',
    content_type smallint not null default 1,
    visibility smallint not null default 1,
    status smallint not null default 1,
    like_count bigint not null default 0,
    comment_count bigint not null default 0,
    favorite_count bigint not null default 0,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    deleted_at timestamptz null,
    constraint fk_posts_author_user_id
        foreign key (author_user_id) references users(id) on delete cascade,
    constraint chk_posts_content_type
        check (content_type in (1, 2, 3, 4)),
    constraint chk_posts_visibility
        check (visibility in (1, 2)),
    constraint chk_posts_status
        check (status in (1, 2, 3, 4)),
    constraint chk_posts_counts_non_negative
        check (
            like_count >= 0 and
            comment_count >= 0 and
            favorite_count >= 0
        )
);

comment on table posts is 'community posts';
comment on column posts.visibility is '1 public, 2 followers_only';
comment on column posts.status is '1 normal, 2 reviewing, 3 blocked, 4 deleted';

create index if not exists idx_posts_author_created_at
    on posts (author_user_id, created_at desc);

create index if not exists idx_posts_visibility_status_created_at
    on posts (visibility, status, created_at desc)
    where deleted_at is null;

create index if not exists idx_posts_status_created_at
    on posts (status, created_at desc)
    where deleted_at is null;
