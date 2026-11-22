-- 为posts表添加全文搜索索引
-- 使用PostgreSQL的全文搜索功能替代ILIKE

-- 添加tsvector列用于全文搜索
ALTER TABLE posts ADD COLUMN IF NOT EXISTS content_tsv tsvector;

-- 创建触发器函数，自动更新tsvector
CREATE OR REPLACE FUNCTION posts_content_tsv_trigger() RETURNS trigger AS $$
BEGIN
    NEW.content_tsv := to_tsvector('simple', COALESCE(NEW.content_text, ''));
    RETURN NEW;
END
$$ LANGUAGE plpgsql;

-- 创建触发器
DROP TRIGGER IF EXISTS posts_content_tsv_update ON posts;
CREATE TRIGGER posts_content_tsv_update
    BEFORE INSERT OR UPDATE ON posts
    FOR EACH ROW
    EXECUTE FUNCTION posts_content_tsv_trigger();

-- 更新现有数据
UPDATE posts SET content_tsv = to_tsvector('simple', COALESCE(content_text, ''));

-- 创建GIN索引以加速全文搜索
CREATE INDEX IF NOT EXISTS idx_posts_content_tsv ON posts USING GIN(content_tsv);

-- 为topics表添加全文搜索索引
ALTER TABLE topics ADD COLUMN IF NOT EXISTS name_tsv tsvector;

CREATE OR REPLACE FUNCTION topics_name_tsv_trigger() RETURNS trigger AS $$
BEGIN
    NEW.name_tsv := to_tsvector('simple', COALESCE(NEW.display_name, '') || ' ' || COALESCE(NEW.topic_name, ''));
    RETURN NEW;
END
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS topics_name_tsv_update ON topics;
CREATE TRIGGER topics_name_tsv_update
    BEFORE INSERT OR UPDATE ON topics
    FOR EACH ROW
    EXECUTE FUNCTION topics_name_tsv_trigger();

UPDATE topics SET name_tsv = to_tsvector('simple', COALESCE(display_name, '') || ' ' || COALESCE(topic_name, ''));

CREATE INDEX IF NOT EXISTS idx_topics_name_tsv ON topics USING GIN(name_tsv);

-- 添加其他性能优化索引
CREATE INDEX IF NOT EXISTS idx_posts_author_created ON posts(author_user_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_posts_status_visibility ON posts(status, visibility, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_comments_post_created ON comments(post_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_post_topics_topic_id ON post_topics(topic_id, post_id);
