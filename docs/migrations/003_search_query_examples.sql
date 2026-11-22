-- 优化后的搜索查询示例

-- 帖子全文搜索（替换原来的ILIKE查询）
-- 原查询：WHERE content_text ILIKE '%' || $1 || '%'
-- 新查询：
SELECT id, author_user_id, content_text, content_type, visibility, status,
       like_count, comment_count, favorite_count, created_at, updated_at, deleted_at,
       ts_rank(content_tsv, query) AS rank
FROM posts, to_tsquery('simple', $1) query
WHERE status = 1
  AND deleted_at IS NULL
  AND content_tsv @@ query
ORDER BY rank DESC, like_count DESC, comment_count DESC, created_at DESC
LIMIT 20;

-- 话题搜索（替换原来的ILIKE查询）
-- 原查询：WHERE display_name ILIKE '%' || $1 || '%' OR topic_name ILIKE '%' || $1 || '%'
-- 新查询：
SELECT id, topic_name, display_name, post_count, hot_score,
       COALESCE(last_refreshed_at, '0001-01-01T00:00:00Z'::timestamptz), created_at, updated_at,
       ts_rank(name_tsv, query) AS rank
FROM topics, to_tsquery('simple', $1) query
WHERE deleted_at IS NULL
  AND name_tsv @@ query
ORDER BY rank DESC, hot_score DESC, post_count DESC, id DESC
LIMIT 20;

-- 如果需要支持中文分词，可以使用zhparser扩展
-- CREATE EXTENSION IF NOT EXISTS zhparser;
-- CREATE TEXT SEARCH CONFIGURATION chinese_zh (PARSER = zhparser);
-- ALTER TEXT SEARCH CONFIGURATION chinese_zh ADD MAPPING FOR n,v,a,i,e,l WITH simple;
