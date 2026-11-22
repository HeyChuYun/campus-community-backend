-- post_media: media metadata for posts
-- media_type:
-- 1 = image
-- 2 = video
-- 3 = gif
create table if not exists post_media (
    id bigserial primary key,
    post_id bigint not null,
    media_type smallint not null,
    storage_key text not null,
    url text not null default '',
    width integer not null default 0,
    height integer not null default 0,
    duration_ms bigint not null default 0,
    size_bytes bigint not null default 0,
    sort_order integer not null default 0,
    created_at timestamptz not null default now(),
    constraint fk_post_media_post_id
        foreign key (post_id) references posts(id) on delete cascade,
    constraint chk_post_media_type
        check (media_type in (1, 2, 3)),
    constraint chk_post_media_size_non_negative
        check (
            width >= 0 and
            height >= 0 and
            duration_ms >= 0 and
            size_bytes >= 0 and
            sort_order >= 0
        )
);

comment on table post_media is 'post media metadata';

create index if not exists idx_post_media_post_id_sort_order
    on post_media (post_id, sort_order asc);
