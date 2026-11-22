create table if not exists post_topics (
    post_id bigint not null,
    topic_id bigint not null,
    created_at timestamptz not null default now(),
    primary key (post_id, topic_id),
    constraint fk_post_topics_post_id
        foreign key (post_id) references posts(id) on delete cascade,
    constraint fk_post_topics_topic_id
        foreign key (topic_id) references topics(id) on delete cascade
);

comment on table post_topics is 'many-to-many mapping between posts and topics';

create index if not exists idx_post_topics_topic_id
    on post_topics (topic_id, created_at desc);
