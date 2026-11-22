create table if not exists user_privacy_blocks (
    id bigserial primary key,
    user_id bigint not null,
    target_user_id bigint not null,
    scope varchar(20) not null,
    created_at timestamptz not null default now(),
    constraint fk_user_privacy_blocks_user
        foreign key (user_id) references users(id) on delete cascade,
    constraint fk_user_privacy_blocks_target_user
        foreign key (target_user_id) references users(id) on delete cascade,
    constraint chk_user_privacy_blocks_scope
        check (scope in ('dm', 'post')),
    constraint chk_user_privacy_blocks_not_self
        check (user_id <> target_user_id)
);

create unique index if not exists uk_user_privacy_blocks_pair_scope
    on user_privacy_blocks (user_id, target_user_id, scope);

create index if not exists idx_user_privacy_blocks_user_scope
    on user_privacy_blocks (user_id, scope, created_at desc);

create index if not exists idx_user_privacy_blocks_target_scope
    on user_privacy_blocks (target_user_id, scope, created_at desc);

create table if not exists post_privacy_settings (
    post_id bigint primary key,
    allow_comment boolean not null default true,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint fk_post_privacy_settings_post
        foreign key (post_id) references posts(id) on delete cascade
);

create table if not exists post_hidden_users (
    id bigserial primary key,
    post_id bigint not null,
    hidden_user_id bigint not null,
    created_at timestamptz not null default now(),
    constraint fk_post_hidden_users_post
        foreign key (post_id) references posts(id) on delete cascade,
    constraint fk_post_hidden_users_user
        foreign key (hidden_user_id) references users(id) on delete cascade
);

create unique index if not exists uk_post_hidden_users_pair
    on post_hidden_users (post_id, hidden_user_id);

create index if not exists idx_post_hidden_users_hidden_user
    on post_hidden_users (hidden_user_id, created_at desc);
