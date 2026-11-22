create table if not exists hot_topics (
    topic_id bigint primary key,
    rank_no integer not null,
    hot_score double precision not null default 0,
    post_count bigint not null default 0,
    calculated_at timestamptz not null default now(),
    constraint fk_hot_topics_topic_id
        foreign key (topic_id) references topics(id) on delete cascade,
    constraint chk_hot_topics_rank_positive check (rank_no > 0),
    constraint chk_hot_topics_post_count_non_negative check (post_count >= 0)
);

comment on table hot_topics is 'materialized hot topic list for app search home';

create unique index if not exists uk_hot_topics_rank_no
    on hot_topics (rank_no);

create index if not exists idx_hot_topics_calculated_at
    on hot_topics (calculated_at desc);

create index if not exists idx_posts_content_text_trgm
    on posts using gin (content_text gin_trgm_ops)
    where deleted_at is null;

create index if not exists idx_users_account_trgm
    on users using gin (account gin_trgm_ops)
    where deleted_at is null;

create index if not exists idx_users_nickname_trgm
    on users using gin (nickname gin_trgm_ops)
    where deleted_at is null;
