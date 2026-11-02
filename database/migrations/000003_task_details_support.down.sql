DROP TABLE task_submission_attachments;
DROP TABLE task_attachments;
DROP TABLE task_submissions;
DROP INDEX idx_task_comments_parent;
ALTER TABLE task_comments DROP CONSTRAINT chk_comment_not_own_parent;
ALTER TABLE task_comments DROP COLUMN parent_comment_id;
