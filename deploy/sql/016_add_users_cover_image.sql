alter table if exists users
    add column if not exists cover_image_url text not null default '';

comment on column users.cover_image_url is '个人主页展示图地址';
