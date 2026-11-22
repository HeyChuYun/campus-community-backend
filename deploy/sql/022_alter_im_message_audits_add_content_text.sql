alter table if exists im_message_audits
    add column if not exists content_text text not null default '';

comment on column im_message_audits.content_text is '私信原始文本内容，供会话列表和历史消息读取';
