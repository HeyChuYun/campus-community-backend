-- relation_outbox_events: outbox table for relation-service
-- status:
-- 1 = pending
-- 2 = sent
-- 3 = failed
create table if not exists relation_outbox_events (
    id bigserial primary key,
    event_id varchar(64) not null,
    event_type varchar(128) not null,
    routing_key varchar(128) not null,
    payload_json jsonb not null,
    status smallint not null default 1,
    retry_count integer not null default 0,
    next_retry_at timestamptz null,
    created_at timestamptz not null default now(),
    sent_at timestamptz null,
    constraint chk_relation_outbox_events_status
        check (status in (1, 2, 3)),
    constraint chk_relation_outbox_events_retry_count
        check (retry_count >= 0)
);

comment on table relation_outbox_events is 'relation-service outbox events';

create unique index if not exists uk_relation_outbox_events_event_id
    on relation_outbox_events (event_id);

create index if not exists idx_relation_outbox_events_status_next_retry_at
    on relation_outbox_events (status, next_retry_at nulls first, created_at asc);
