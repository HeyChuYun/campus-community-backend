create table if not exists topics (
    id bigserial primary key,
    topic_name varchar(64) not null,
    display_name varchar(64) not null,
    post_count bigint not null default 0,
    hot_score double precision not null default 0,
    last_refreshed_at timestamptz null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    deleted_at timestamptz null,
    constraint chk_topics_post_count_non_negative check (post_count >= 0)
);

comment on table topics is 'normalized topic dictionary';
comment on column topics.topic_name is 'normalized topic name used for deduplication';
comment on column topics.display_name is 'display topic name shown to client';
comment on column topics.post_count is 'number of active posts bound to this topic';
comment on column topics.hot_score is 'precomputed hot topic score';
comment on column topics.last_refreshed_at is 'last time hot score was refreshed';

create unique index if not exists uk_topics_topic_name_active
    on topics (topic_name)
    where deleted_at is null;

create index if not exists idx_topics_display_name_trgm
    on topics using gin (display_name gin_trgm_ops)
    where deleted_at is null;

create index if not exists idx_topics_topic_name_trgm
    on topics using gin (topic_name gin_trgm_ops)
    where deleted_at is null;
