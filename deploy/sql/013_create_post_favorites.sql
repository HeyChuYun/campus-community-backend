-- post_favorites: post favorite relation
create table if not exists post_favorites (
    id bigserial primary key,
    post_id bigint not null,
    user_id bigint not null,
    created_at timestamptz not null default now(),
    constraint fk_post_favorites_post_id
        foreign key (post_id) references posts(id) on delete cascade,
    constraint fk_post_favorites_user_id
        foreign key (user_id) references users(id) on delete cascade
);

comment on table post_favorites is 'post favorite relation';

create unique index if not exists uk_post_favorites_post_user
    on post_favorites (post_id, user_id);

create index if not exists idx_post_favorites_user_id
    on post_favorites (user_id);
